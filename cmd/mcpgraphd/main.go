// Command mcpgraphd watches AI app configs, keeps snapshots and serves a
// read-only API on 127.0.0.1.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/pilsuchoi/mcpgraph/internal/daemon"
	"github.com/pilsuchoi/mcpgraph/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		home        = flag.String("home", "", "home directory to scan (default: current user's home)")
		dataDir     = flag.String("data-dir", "", "where snapshots are stored (default: per-OS user data dir)")
		interval    = flag.Duration("interval", time.Minute, "full re-scan interval")
		noProcesses = flag.Bool("no-processes", false, "skip process discovery")
		verbose     = flag.Bool("v", false, "debug logging")
	)
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	// The daemon logs JSON so log shippers can parse it.
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	if *home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home directory: %w", err)
		}
		*home = h
	}
	if *dataDir == "" {
		*dataDir = defaultDataDir(*home)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, filepath.Join(*dataDir, "mcpgraph.db"))
	if err != nil {
		return err
	}
	// Deferred calls run last-in, first-out: st.Close runs before stop().
	defer st.Close()

	d := daemon.New(daemon.Options{
		Home:      *home,
		GOOS:      runtime.GOOS,
		Interval:  *interval,
		Processes: !*noProcesses,
		Store:     st,
		Log:       log,
	})
	log.Info("mcpgraphd starting", "home", *home, "data_dir", *dataDir)
	return d.Run(ctx)
}

// defaultDataDir follows each OS's convention for per-user app data.
func defaultDataDir(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "mcpgraph")
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "mcpgraph")
	}
	return filepath.Join(home, ".local", "share", "mcpgraph")
}
