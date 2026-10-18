package process

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// gopsutilLister reads the real process table with gopsutil (pure Go, no
// cgo: it parses /proc on Linux and uses sysctl on macOS).
type gopsutilLister struct{}

// List implements Lister. Processes can exit while we read them, and some
// fields need privileges we do not have, so per-process errors are
// ignored and the field is left empty.
func (gopsutilLister) List(ctx context.Context) ([]Info, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(procs))
	for _, p := range procs {
		// Stop early if the scan was cancelled; the list can be long.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		info := Info{PID: p.Pid}
		info.PPID, _ = p.PpidWithContext(ctx)
		info.Name, _ = p.NameWithContext(ctx)
		info.Exe, _ = p.ExeWithContext(ctx)
		info.Args, _ = p.CmdlineSliceWithContext(ctx)
		info.Args = normalizeArgs(info.Args)
		info.CreateTime, _ = p.CreateTimeWithContext(ctx)
		out = append(out, info)
	}
	return out, nil
}

// normalizeArgs splits a single-element command line that contains spaces.
// Programs that rewrite their process title (npm does: "npm exec pkg")
// leave the whole command line in argv[0].
func normalizeArgs(args []string) []string {
	if len(args) == 1 && strings.Contains(args[0], " ") {
		return strings.Fields(args[0])
	}
	return args
}
