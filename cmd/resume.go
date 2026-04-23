package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/engine"
	"github.com/poldebatecli/internal/output"
	"github.com/poldebatecli/internal/planner"
	"github.com/poldebatecli/internal/round"
	"github.com/poldebatecli/internal/session"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume <session-file>",
	Short: "Resume a saved debate session",
	Long:  `Resume a previously saved debate session from a session file created with --save-session.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runResume,
}

var (
	resumeTimeout time.Duration
	resumeVerbose bool
)

func init() {
	rootCmd.AddCommand(resumeCmd)
	resumeCmd.Flags().DurationVar(&resumeTimeout, "timeout", 10*time.Minute, "Global timeout for the resumed debate")
	resumeCmd.Flags().BoolVarP(&resumeVerbose, "verbose", "v", false, "Verbose logging")
}

func runResume(cmd *cobra.Command, args []string) error {
	snap, err := session.Load(args[0])
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	cfg := snap.Config
	if _, err := planner.NewRuleBasedPlanner().Plan(snap.Topic, cfg); err != nil {
		return fmt.Errorf("team planner: %w", err)
	}

	if resumeVerbose || verbose {
		cfg.Logging.Level = "debug"
	}
	setupLogger(cfg)

	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	if err := config.ValidateAPIKey(cfg); err != nil {
		return err
	}

	slog.Info("resuming debate session", "file", args[0],
		"cycle", snap.Progress.CurrentCycle, "round", snap.Progress.CurrentRoundIndex,
		"completed_rounds", len(snap.Rounds))

	deps, err := engine.NewEngineDeps(cfg)
	if err != nil {
		return fmt.Errorf("initialize engine: %w", err)
	}
	eng := engine.NewEngine(deps)

	ctx, cancel := context.WithTimeout(context.Background(), resumeTimeout)
	defer cancel()

	var liveCh chan round.LiveEvent
	var liveWg sync.WaitGroup

	if cfg.Output.Streaming {
		liveCh = make(chan round.LiveEvent, 128)
		liveWg.Add(1)
		go func() {
			defer liveWg.Done()
			consumeLiveEvents(liveCh)
		}()
	}

	effectiveInterval := cfg.Session.SaveInterval
	if effectiveInterval == 0 {
		effectiveInterval = 30 * time.Second
	}

	sessionCfg := &engine.SessionConfig{
		SavePath:              args[0],
		SaveInterval:          effectiveInterval,
		CheckpointMinInterval: cfg.Session.CheckpointMinInterval,
		ResumeFrom:            snap,
	}

	result, err := eng.Run(ctx, snap.Topic, cfg, liveCh, sessionCfg)
	if liveCh != nil {
		liveWg.Wait()
	}
	if err != nil {
		return fmt.Errorf("debate execution: %w", err)
	}

	jsonWriter := output.NewJSONWriter()
	if err := jsonWriter.Write(ctx, result, cfg); err != nil {
		slog.Warn("JSON write failed", "error", err)
	}

	transcriptWriter := output.NewTranscriptWriter()
	if err := transcriptWriter.Write(ctx, result, cfg); err != nil {
		slog.Warn("transcript write failed", "error", err)
	}

	termWriter := output.NewTermWriter()
	if err := termWriter.Write(ctx, result, cfg); err != nil {
		slog.Warn("terminal display failed", "error", err)
	}

	fmt.Fprintf(os.Stderr, "\nResumed debate complete. Results written to %s\n", cfg.Output.Path)
	return nil
}
