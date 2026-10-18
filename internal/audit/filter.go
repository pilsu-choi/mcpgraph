package audit

import (
	"strings"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// Filter selects decisions. Zero fields match everything.
type Filter struct {
	Since   time.Time     // decisions at or after this time
	Verdict model.Verdict // allow, deny or ask
	Source  string        // proxy or hook
	Agent   string        // exact, case-insensitive
	Tool    string        // exact, case-insensitive
	RuleID  string        // exact
	Limit   int           // keep only the newest N (0 = all)
}

// Match reports whether d passes every set field.
func (f Filter) Match(d model.Decision) bool {
	switch {
	case !f.Since.IsZero() && d.Time.Before(f.Since):
		return false
	case f.Verdict != "" && d.Verdict != f.Verdict:
		return false
	case f.Source != "" && d.Source != f.Source:
		return false
	case f.Agent != "" && !strings.EqualFold(d.Agent, f.Agent):
		return false
	case f.Tool != "" && !strings.EqualFold(d.Tool, f.Tool):
		return false
	case f.RuleID != "" && d.RuleID != f.RuleID:
		return false
	}
	return true
}
