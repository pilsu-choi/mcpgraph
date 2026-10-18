package policy

import (
	"regexp"
	"strings"
)

// SQL statement classes the `sql:` matcher understands.
const (
	SQLDropTable          = "drop_table"
	SQLDropDatabase       = "drop_database" // also DROP SCHEMA
	SQLTruncate           = "truncate"
	SQLDeleteWithoutWhere = "delete_without_where"
)

var sqlClasses = map[string]bool{
	SQLDropTable: true, SQLDropDatabase: true, SQLTruncate: true, SQLDeleteWithoutWhere: true,
}

// sqlGap matches what may legally sit between two SQL keywords: whitespace,
// /* block comments */ and -- line comments. Using it instead of \s+ means
// "DROP/**/TABLE" and "DROP -- x\nTABLE" are still caught.
const sqlGap = `(?:\s|/\*[\s\S]*?\*/|--[^\n]*(?:\n|$))+`

// Compiled once at package load. (?i) makes them case-insensitive.
var (
	reDropTable    = regexp.MustCompile(`(?i)\bdrop` + sqlGap + `table\b`)
	reDropDatabase = regexp.MustCompile(`(?i)\bdrop` + sqlGap + `(?:database|schema)\b`)
	// TRUNCATE [TABLE] [ONLY] name. Requiring a name-like character after
	// it keeps the shell command `truncate -s 0 file` out.
	reTruncate = regexp.MustCompile(`(?i)\btruncate` + sqlGap + `(?:table` + sqlGap + `)?(?:only` + sqlGap + `)?["a-z_]`)
	// DELETE [target] FROM name: the optional target covers MySQL's
	// "DELETE t FROM t JOIN ...". A table name must follow FROM, so prose
	// such as `git log --grep "delete from"` does not count.
	reDelete = regexp.MustCompile(`(?i)\bdelete` + sqlGap + `(?:[^\s;]+` + sqlGap + `)?from` + sqlGap + "[\"`\\[a-z_]")

	reBlockComment = regexp.MustCompile(`/\*[\s\S]*?\*/`)
	reLineComment  = regexp.MustCompile(`--[^\n]*`)
	reWhere        = regexp.MustCompile(`(?i)\bwhere\b`)
	// Clauses that may follow a WHERE condition.
	reAfterWhere = regexp.MustCompile(`(?i)\b(?:returning|order\s+by|limit)\b`)
	reOr         = regexp.MustCompile(`(?i)\bor\b`)
	reComparison = regexp.MustCompile(`^(.+?)\s*(?:==|=|>=|<=|\bis\b)\s*(.+)$`)
)

// classifySQL returns the destructive statement classes found in text.
// text can be a bare query or a whole shell command such as
// `psql -c "DELETE FROM users"`; the classifier only looks for keyword
// patterns, so it does not need to know which.
//
// This is a small classifier, not a SQL parser. It knows comments, mixed
// case, statements separated by ";" and tautological WHERE clauses
// (WHERE 1=1, WHERE true, ... OR 'a'='a'). It does not understand string
// literals, so a keyword inside a string can still count.
func classifySQL(text string) map[string]bool {
	found := map[string]bool{}
	if reDropTable.MatchString(text) {
		found[SQLDropTable] = true
	}
	if reDropDatabase.MatchString(text) {
		found[SQLDropDatabase] = true
	}
	if reTruncate.MatchString(text) {
		found[SQLTruncate] = true
	}
	for _, loc := range reDelete.FindAllStringIndex(text, -1) {
		if !hasRealWhere(deleteStatement(text[loc[0]:])) {
			found[SQLDeleteWithoutWhere] = true
			break
		}
	}
	return found
}

// deleteStatement returns the DELETE statement at the start of s: up to the
// next ";" or shell "&&" / "||", with comments removed.
//
// Comments must go before looking for WHERE, or `DELETE FROM t -- WHERE
// id = 1` would look safe.
func deleteStatement(s string) string {
	s = reBlockComment.ReplaceAllString(s, " ")
	s = reLineComment.ReplaceAllString(s, " ")
	end := len(s)
	for _, sep := range []string{";", "&&", "||"} {
		if i := strings.Index(s, sep); i >= 0 && i < end {
			end = i
		}
	}
	return s[:end]
}

// hasRealWhere reports whether a DELETE statement has a WHERE clause that
// actually limits the rows. "WHERE 1=1" and "WHERE id=5 OR true" do not.
func hasRealWhere(stmt string) bool {
	loc := reWhere.FindStringIndex(stmt)
	if loc == nil {
		return false
	}
	cond := stmt[loc[1]:]
	if i := reAfterWhere.FindStringIndex(cond); i != nil {
		cond = cond[:i[0]]
	}
	cond = trimUnbalancedQuote(strings.TrimSpace(cond))
	if strings.TrimSpace(cond) == "" {
		return false
	}
	for _, part := range reOr.Split(cond, -1) {
		if tautology(part) {
			return false
		}
	}
	return true
}

// trimUnbalancedQuote drops a final quote left over from shell quoting,
// e.g. the closing " of psql -c "DELETE ... WHERE id = 1". A balanced
// quote, as in 'a'='a', is part of the SQL and stays.
func trimUnbalancedQuote(s string) string {
	for _, q := range []string{`"`, `'`} {
		if strings.HasSuffix(s, q) && strings.Count(s, q)%2 == 1 {
			s = strings.TrimSpace(s[:len(s)-1])
		}
	}
	return s
}

// tautology reports whether a condition is always true: true, a non-zero
// number, or "X = X" with both sides written the same way.
func tautology(cond string) bool {
	c := strings.ToLower(strings.TrimSpace(cond))
	for strings.HasPrefix(c, "(") && strings.HasSuffix(c, ")") {
		c = strings.TrimSpace(c[1 : len(c)-1])
	}
	c = strings.TrimSpace(strings.Trim(c, "()"))
	switch c {
	case "true", "not false", "1", "'1'", "'t'":
		return true
	}
	m := reComparison.FindStringSubmatch(c)
	return m != nil && strings.TrimSpace(m[1]) == strings.TrimSpace(m[2])
}
