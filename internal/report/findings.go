package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// Findings writes findings grouped by severity, most severe first.
func Findings(w io.Writer, findings []model.Finding) error {
	counts := map[model.Severity]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	var b strings.Builder
	fmt.Fprintf(&b, "FINDINGS: %d high, %d medium, %d low\n",
		counts[model.SeverityHigh], counts[model.SeverityMedium], counts[model.SeverityLow])

	for _, sev := range []model.Severity{model.SeverityHigh, model.SeverityMedium, model.SeverityLow} {
		if counts[sev] == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s (%d)\n", strings.ToUpper(string(sev)), counts[sev])
		for _, f := range findings {
			if f.Severity != sev {
				continue
			}
			fmt.Fprintf(&b, "  [%s] %s\n", f.RuleID, f.Title)
			fmt.Fprintf(&b, "       %s\n", f.Message)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}
