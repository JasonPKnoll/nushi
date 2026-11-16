package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/afrase/claude-acp-go/claudeacp"
	"github.com/afrase/claude-acp-go/claudeagent"

	"github.com/afrase/nushi/internal/batchfile"
	"github.com/afrase/nushi/internal/shellenv"
)

// runClaudeAgent runs Claude's Go agent, the claude-acp subcommand, as
// claudeacp.Main does, with each claude started through spawnClaude:
// Main takes no Spawner, and claude installed by npm is a batch file.
func runClaudeAgent() error {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := claudeacp.Config{CLIPath: os.Getenv("CLAUDE_CODE_EXECUTABLE"), Log: log, SharedGroup: true, Spawn: spawnClaude}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigs := make(chan os.Signal, 1)
	// SIGTERM is how Go delivers a console's close, a logoff and a
	// shutdown; os.Interrupt, Ctrl+Break (Ctrl+C is off in its group).
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)
	go func() {
		select {
		case sig := <-sigs:
			signal.Stop(sigs) // the next one has its default effect
			log.Info("closing the sessions on a signal", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()
	err := claudeacp.Serve(ctx, os.Stdin, os.Stdout, cfg)
	if ctx.Err() != nil {
		return nil // a signal: the end the agent was asked for
	}
	if err != nil {
		log.Error("serving the client", "err", err)
	}
	return err
}

// spawnClaude starts claude as claudeagent.Exec does, guarded against
// cmd.exe (batchfile.Command): npm's claude.cmd runs as node and its
// script, so neither a project's node.exe nor the arguments' quotes and
// `%` (an MCP config's JSON, a settings path) reach cmd.exe.
func spawnClaude(ctx context.Context, s claudeagent.Spec) (*claudeagent.Proc, error) {
	if !filepath.IsAbs(s.Path) {
		if p := shellenv.LookPath(s.Path, s.Env); p != "" {
			s.Path = p
		}
	}
	s.Env = batchfile.Environ(s.Env)
	path, args, env, err := batchfile.Command(s.Path, s.Args, s.Env)
	if err != nil {
		return nil, err
	}
	s.Path, s.Args, s.Env = path, args, env
	return claudeagent.Exec(ctx, s)
}
