package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/central"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/netutil"
)

func newCollectorCmd() *cobra.Command {
	var listen, dir, token string
	cmd := &cobra.Command{
		Use:   "collector --data DIR [--listen 127.0.0.1:7600] [--token T]",
		Short: "Run a minimal collector that stores events shipped by `mcpgraph sink`",
		Long: `Run an HTTP collector. Machines that ran "mcpgraph sink set" POST their
decisions to /v1/events; the collector drops duplicates and appends the
rest to DIR/events.jsonl. Read them with GET /v1/events and /v1/hosts, or
"mcpgraph events --collector URL".

It listens on loopback by default. Pass a non-loopback address
explicitly to accept other machines, and set --token when you do.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return fmt.Errorf("--data DIR is required")
			}
			host, _, err := net.SplitHostPort(listen)
			if err != nil {
				return fmt.Errorf("--listen %q: %w", listen, err)
			}
			if !netutil.IsLoopbackHost(host) && token == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: listening on %s without --token: anyone who can reach it can read and write events\n", listen)
			}
			store, err := central.OpenStore(dir)
			if err != nil {
				return err
			}
			defer store.Close()
			var lc net.ListenConfig // Listen with a context, as the noctx linter wants
			ln, err := lc.Listen(cmd.Context(), "tcp", listen)
			if err != nil {
				return err
			}
			// Stop on Ctrl-C or SIGTERM: cancelling ctx makes Serve shut down.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			fmt.Fprintf(cmd.ErrOrStderr(), "collector listening on http://%s, storing in %s\n", ln.Addr(), dir)
			return central.Serve(ctx, ln, central.Handler(store, token))
		},
	}
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:7600", "address to listen on")
	cmd.Flags().StringVar(&dir, "data", "", "directory for events.jsonl")
	cmd.Flags().StringVar(&token, "token", "", "require this bearer token (recommended off loopback)")
	return cmd
}

// collectorFlags are the flags of `mcpgraph events --collector`.
type collectorFlags struct{ url, token, host string }

// query asks the collector for events matching f and returns the decisions.
func (c collectorFlags) query(ctx context.Context, f audit.Filter) ([]model.Decision, error) {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("host", c.host)
	set("verdict", string(f.Verdict))
	set("source", f.Source)
	if !f.Since.IsZero() {
		set("since", f.Since.UTC().Format("2006-01-02T15:04:05Z07:00"))
	}
	if f.Limit > 0 {
		set("limit", strconv.Itoa(f.Limit))
	}
	events, err := central.Query(ctx, c.url, c.token, q)
	if err != nil {
		return nil, fmt.Errorf("query collector: %w", err)
	}
	out := make([]model.Decision, 0, len(events))
	for _, e := range events {
		if f.Match(e.Decision) { // agent, tool and rule filters run client side
			out = append(out, e.Decision)
		}
	}
	return out, nil
}
