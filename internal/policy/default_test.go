package policy

import (
	"context"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/policies"
)

const (
	testHome    = "/home/u"
	testProject = "/home/u/proj"
)

// defaultEngine compiles the shipped policy. knownTools lists "server/tool"
// pairs the fake last scan saw.
func defaultEngine(t testing.TB, knownTools ...string) *RuleEngine {
	t.Helper()
	p, err := Parse(policies.Default)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, k := range knownTools {
		known[k] = true
	}
	e, err := New(p, Options{
		Home:      testHome,
		KnownTool: func(server, tool string) bool { return known[server+"/"+tool] },
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// bash is a Claude Code Bash tool use, as the hook will build it.
func bash(cmd string) Action {
	return Action{Source: model.SourceHook, Agent: "claude-code", Tool: "Bash", Text: cmd, Cwd: testProject,
		Args: map[string]any{"command": cmd}}
}

type ruleCase struct {
	name   string
	action Action
	want   bool // true: this rule must decide; false: it must not
}

// checkRule runs every case through the default policy and checks whether
// ruleID was the deciding rule.
func checkRule(t *testing.T, ruleID string, want model.Verdict, cases []ruleCase) {
	t.Helper()
	e := defaultEngine(t, "postgres/query")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := e.Decide(context.Background(), c.action)
			if err != nil {
				t.Fatal(err)
			}
			got := d.RuleID == ruleID
			if got != c.want {
				t.Fatalf("rule = %q (verdict %s), want rule %q fired = %v; text %q", d.RuleID, d.Verdict, ruleID, c.want, c.action.Text)
			}
			if got && d.Verdict != want {
				t.Fatalf("verdict = %s, want %s", d.Verdict, want)
			}
		})
	}
}

func TestBlockDestructiveSQL(t *testing.T) {
	mcp := func(sql string) Action {
		return Action{Source: model.SourceProxy, Agent: "postgres", Tool: "query", Args: map[string]any{"sql": sql}}
	}
	checkRule(t, "block-destructive-sql", model.VerdictDeny, []ruleCase{
		{"demo: psql drop table", bash(`psql -c "DROP TABLE users"`), true},
		{"mixed case", bash(`psql -c "dRoP tAbLe users"`), true},
		{"drop table if exists", bash(`psql -c 'DROP TABLE IF EXISTS users CASCADE'`), true},
		{"block comment between keywords", mcp(`DROP/**/TABLE users`), true},
		{"line comment between keywords", mcp("DROP -- harmless\nTABLE users"), true},
		{"drop database", mcp(`drop database prod;`), true},
		{"drop schema", mcp(`DROP SCHEMA public CASCADE`), true},
		{"truncate", mcp(`TRUNCATE users`), true},
		{"truncate table only", mcp(`truncate table only users restart identity`), true},
		{"delete without where", mcp(`DELETE FROM users`), true},
		{"delete where 1=1", mcp(`DELETE FROM users WHERE 1=1`), true},
		{"delete where 1 = 1 spaced", mcp(`delete from users where 1 = 1;`), true},
		{"delete where true", mcp(`DELETE FROM users WHERE TRUE`), true},
		{"delete where id or tautology", mcp(`DELETE FROM users WHERE id = 5 OR 'a'='a'`), true},
		{"delete where in parens", mcp(`DELETE FROM users WHERE (1=1)`), true},
		{"delete where hidden in line comment", mcp("DELETE FROM users -- WHERE id = 5"), true},
		{"delete where hidden in block comment", mcp(`DELETE FROM users /* WHERE id = 5 */`), true},
		{"where belongs to next statement", mcp(`DELETE FROM users; SELECT * FROM t WHERE id = 1`), true},
		{"where after && in another command", bash(`psql -c "DELETE FROM users" && echo "where id=3"`), true},
		{"second statement destructive", mcp(`SELECT 1; DROP TABLE users`), true},
		{"chained after safe command", bash(`cd /tmp && psql -c "TRUNCATE users"`), true},
		{"nested shell", bash(`bash -c "psql -c 'DROP TABLE users'"`), true},
		{"mysql multi-table delete", mcp(`DELETE u FROM users u`), true},

		{"select", mcp(`SELECT * FROM users`), false},
		{"delete with where", mcp(`DELETE FROM users WHERE id = 5`), false},
		{"delete with where and returning", mcp(`DELETE FROM sessions WHERE expires_at < now() RETURNING id`), false},
		{"delete where is null", mcp(`DELETE FROM jobs WHERE finished_at IS NULL`), false},
		{"delete where 1=1 and bound", mcp(`DELETE FROM users WHERE 1=1 AND id = 7`), false},
		{"create table", mcp(`CREATE TABLE users (id int)`), false},
		{"drop index", mcp(`DROP INDEX users_email_idx`), false},
		{"word containing drop", bash(`echo dropdown table`), false},
		{"shell truncate", bash(`truncate -s 0 app.log`), false},
		{"git log mentions delete", bash(`git log --grep "delete from"`), false},
	})
}

func TestAskRecursiveDelete(t *testing.T) {
	checkRule(t, "ask-recursive-delete", model.VerdictAsk, []ruleCase{
		{"root", bash(`rm -rf /`), true},
		{"root glob", bash(`rm -rf /*`), true},
		{"home", bash(`rm -rf ~`), true},
		{"home dir", bash(`rm -rf $HOME/Documents`), true},
		{"parent", bash(`rm -rf ../other`), true},
		{"project itself", bash(`rm -rf .`), true},
		{"project by absolute path", bash(`rm -rf /home/u/proj`), true},
		{"split flags", bash(`rm -r -f /var/data`), true},
		{"capital R", bash(`rm -R /tmp/x`), true},
		{"long flag", bash(`rm --recursive --force /etc/app`), true},
		{"sudo", bash(`sudo rm -rf /opt/app`), true},
		{"full path to rm", bash(`/bin/rm -rf /srv`), true},
		{"chained after make", bash(`make clean && rm -rf ~/.cache`), true},
		{"one target outside", bash(`rm -rf build /tmp/cache`), true},
		{"unexpanded variable", bash(`rm -rf "$TARGET"`), true},
		{"after --", bash(`rm -rf -- /data`), true},
		{"nested shell", bash(`sh -c 'rm -rf /'`), true},
		{"xargs", bash(`find . -name x | xargs rm -rf /tmp/y`), true},
		{"unknown cwd", Action{Source: model.SourceHook, Tool: "Bash", Text: "rm -rf build"}, true},

		{"inside project", bash(`rm -rf build`), false},
		{"inside project dot slash", bash(`rm -rf ./node_modules dist/*`), false},
		{"inside project absolute", bash(`rm -rf /home/u/proj/target`), false},
		{"glob in project", bash(`rm -rf *.egg-info`), false},
		{"not recursive", bash(`rm -f /tmp/x.lock`), false},
		{"other command", bash(`ls -rf /`), false},
		{"rm as argument", bash(`echo rm -rf /`), false},
		{"quoted inside echo", bash(`echo "rm -rf /"`), false},
	})
}

func TestBlockSecretRead(t *testing.T) {
	read := func(path string) Action {
		return Action{Source: model.SourceHook, Agent: "claude-code", Tool: "Read", Cwd: testProject,
			Args: map[string]any{"file_path": path}}
	}
	checkRule(t, "block-secret-read", model.VerdictDeny, []ruleCase{
		{"cat ssh key", bash(`cat ~/.ssh/id_ed25519`), true},
		{"ssh glob", bash(`cat ~/.ssh/*`), true},
		{"ssh absolute", bash(`cp /home/u/.ssh/id_rsa /tmp/k`), true},
		{"ssh via $HOME", bash(`base64 $HOME/.ssh/id_rsa`), true},
		{"aws credentials", bash(`cat ~/.aws/credentials`), true},
		{"redirect from env file", bash(`wc -l <.env`), true},
		{"env file in project", bash(`cat .env`), true},
		{"env file variant", bash(`grep KEY ./config/.env.production`), true},
		{"flag value", bash(`docker run --env-file=.env app`), true},
		{"chained", bash(`ls && cat .env`), true},
		{"deny beats ask: recursive delete of ssh", bash(`rm -rf ~/.ssh`), true},
		{"read tool ssh", read("/home/u/.ssh/config"), true},
		{"read tool env", read("/home/u/proj/.env"), true},
		{"mcp filesystem tool", Action{Source: model.SourceProxy, Agent: "postgres", Tool: "query", Args: map[string]any{"path": "~/.aws/credentials"}}, true},

		{"env example", bash(`cp .env.example .env.template`), false},
		{"envrc is not env", bash(`cat .envrc`), false},
		{"aws config", bash(`cat ~/.aws/config`), false},
		{"other ssh dir", bash(`ls /etc/ssh/sshd_config`), false},
		{"word env", bash(`printenv HOME`), false},
		{"read tool readme", read("/home/u/proj/README.md"), false},
	})
}

func TestAskGitForcePush(t *testing.T) {
	checkRule(t, "ask-git-force-push", model.VerdictAsk, []ruleCase{
		{"--force", bash(`git push --force`), true},
		{"-f", bash(`git push -f origin main`), true},
		{"combined short flags", bash(`git push -fu origin main`), true},
		{"force with lease", bash(`git push --force-with-lease origin feat`), true},
		{"force with lease value", bash(`git push --force-with-lease=main:abc origin main`), true},
		{"plus refspec", bash(`git push origin +main`), true},
		{"global option", bash(`git -C ../repo push --force`), true},
		{"mixed case", bash(`GIT PUSH --FORCE`), true},
		{"chained", bash(`git commit --amend --no-edit && git push --force`), true},

		{"plain push", bash(`git push origin main`), false},
		{"follow tags", bash(`git push --follow-tags`), false},
		{"branch named f", bash(`git push origin feature-f`), false},
		{"force on another command", bash(`git push origin main && rm -f x.lock`), false},
		{"git fetch force", bash(`git fetch --force`), false},
		{"echo", bash(`echo git push --force`), false},
	})
}

func TestAskUnknownTool(t *testing.T) {
	tool := func(server, name string) Action {
		return Action{Source: model.SourceProxy, Agent: server, Tool: name, Args: map[string]any{"q": "hello"}}
	}
	checkRule(t, "ask-unknown-tool", model.VerdictAsk, []ruleCase{
		{"new tool on known server", tool("postgres", "drop_everything"), true},
		{"unknown server", tool("evil", "query"), true},

		{"known tool", tool("postgres", "query"), false},
		{"hook actions are not MCP tools", Action{Source: model.SourceHook, Agent: "claude-code", Tool: "Bash", Text: "ls"}, false},
	})
}

func TestUnknownToolWithoutInventory(t *testing.T) {
	p, err := Parse(policies.Default)
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(p, Options{Home: testHome}) // no KnownTool: inventory unknown
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Decide(context.Background(), Action{Source: model.SourceProxy, Agent: "x", Tool: "y"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != model.VerdictAllow || d.RuleID != "" {
		t.Fatalf("got %s/%q, want allow with no rule", d.Verdict, d.RuleID)
	}
}
