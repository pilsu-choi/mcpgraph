package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pilsu-choi/mcpgraph/internal/sink"
)

func newSinkCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sink",
		Short: "Configure the opt-in shipping of audit events to a collector",
		Long: `The event sink is off until you run "mcpgraph sink set". Nothing leaves this
machine before that (hard rule 3). Shipped events are the redacted audit
decisions only.`,
	}
	cmd.AddCommand(newSinkSetCmd(opts), newSinkOffCmd(opts), newSinkStatusCmd(opts), newSinkFlushCmd(opts))
	return cmd
}

func newSinkSetCmd(opts *options) *cobra.Command {
	var token, host string
	cmd := &cobra.Command{
		Use:   "set <url> [--token T] [--host NAME]",
		Short: "Turn the sink on: ship audit events to this collector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := opts.dataPath()
			if err != nil {
				return err
			}
			if host == "" {
				if host, err = os.Hostname(); err != nil {
					return fmt.Errorf("find host name (use --host): %w", err)
				}
			}
			warn, err := sink.CheckURL(args[0])
			if err != nil {
				return err
			}
			if warn != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", warn)
			}
			cfg := sink.Config{URL: args[0], Token: token, Host: host}
			if err := sink.SaveConfig(dir, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sink on: shipping to %s as %q\n", cfg.URL, cfg.Host)
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "bearer token the collector requires")
	cmd.Flags().StringVar(&host, "host", "", "name this machine reports as (default: host name)")
	return cmd
}

func newSinkOffCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Turn the sink off (removes sink.json)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := opts.dataPath()
			if err != nil {
				return err
			}
			if err := sink.RemoveConfig(dir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "sink off")
			return nil
		},
	}
}

func newSinkStatusCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the sink configuration and how many events are not shipped yet",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := opts.dataPath()
			if err != nil {
				return err
			}
			cfg, on, err := sink.LoadConfig(dir)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !on {
				fmt.Fprintln(out, "sink: off")
				return nil
			}
			pending, err := sink.Pending(dir)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "sink: on\nurl: %s\nhost: %s\ntoken: %s\nnot shipped yet: %d\n", cfg.URL, cfg.Host, cfg.MaskedToken(), pending)
			return nil
		},
	}
}

func newSinkFlushCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "flush",
		Short: "Ship the pending events now",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := opts.dataPath()
			if err != nil {
				return err
			}
			if _, on, err := sink.LoadConfig(dir); err != nil {
				return err
			} else if !on {
				return fmt.Errorf("sink is off (run: mcpgraph sink set <url>)")
			}
			// A batch may take several 2 s requests; bound the whole flush.
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if err := sink.Ship(ctx, dir); err != nil {
				return err
			}
			pending, _ := sink.Pending(dir)
			fmt.Fprintf(cmd.OutOrStdout(), "flushed; not shipped yet: %d\n", pending)
			return nil
		},
	}
}
