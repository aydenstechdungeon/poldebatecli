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
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute a debate between AI agent teams",
	Long: `Execute a structured multi-agent AI debate.

This command runs the full debate pipeline: opening statements through closing,
with judge evaluation after each round. Results are written to the configured
output path in JSON format, with optional terminal display.`,
	Example: `  debate run --topic "AI regulation is necessary"
  debate run --topic "AI regulation is necessary" --config ./my_debate.yaml
  debate run --dry-run
  debate run --models "economist=x-ai/grok-4.20,historian=x-ai/grok-4.1-fast"
  debate run --topic "AI regulation is necessary" --rounds 3 --output ./results/
  debate run --no-stream
  debate run --stream-block paragraph`,
	RunE: runDebate,
}

var (
	runTopic        string
	runModels       string
	runRounds       int
	runOutput       string
	runDryRun       bool
	runNoJudge      bool
	runNoStream     bool
	runStreamBlock  string
	runTimeout      time.Duration
	runVerbose      bool
	runSaveSession  string
	runSaveInterval time.Duration
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&runTopic, "topic", "", "Debate topic (required)")
	runCmd.Flags().StringVar(&runModels, "models", "", "Comma-separated model overrides (role=model pairs)")
	runCmd.Flags().IntVar(&runRounds, "rounds", 0, "Number of full debate cycles (default 1)")
	runCmd.Flags().StringVar(&runOutput, "output", "", "Output directory path (overrides config)")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Validate and show plan without executing API calls")
	runCmd.Flags().BoolVar(&runNoJudge, "no-judge", false, "Skip judge evaluation rounds")
	runCmd.Flags().BoolVar(&runNoStream, "no-stream", false, "Disable streaming output (wait for full responses)")
	runCmd.Flags().StringVar(&runStreamBlock, "stream-block", "", "Streaming block size: sentence, paragraph, token (default \"sentence\")")
	_ = runCmd.MarkFlagRequired("topic")
	runCmd.Flags().DurationVar(&runTimeout, "timeout", 10*time.Minute, "Global timeout for the entire debate")
	runCmd.Flags().BoolVarP(&runVerbose, "verbose", "v", false, "Verbose logging")
	runCmd.Flags().StringVar(&runSaveSession, "save-session", "", "Save session state to file for later resumption")
	runCmd.Flags().DurationVar(&runSaveInterval, "save-interval", 0, "Auto-save interval when --save-session is set (default 30s, or session.save_interval from config)")
}

func runDebate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	config.ApplyFlagOverrides(cfg, runOutput, runRounds, runNoStream, runStreamBlock, runModels)
	if runNoJudge {
		cfg.Output.SkipJudges = true
	}

	if runVerbose || verbose {
		cfg.Logging.Level = "debug"
	}
	setupLogger(cfg)

	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}
	plan, err := planner.NewRuleBasedPlanner().Plan(runTopic, cfg)
	if err != nil {
		return fmt.Errorf("team planner: %w", err)
	}
	if runVerbose || verbose {
		fmt.Printf("Planner: mode=%s teams=%d\n", plan.Mode, len(plan.Teams))
		for _, tm := range plan.Teams {
			fmt.Printf("- %s %s (%s)\n", config.TeamLabel(tm.ID), tm.Name, tm.Side)
			fmt.Printf("  Position: %s\n", tm.PositionDescription)
		}
	}

	if runDryRun {
		fmt.Println("Dry run: configuration valid")
		fmt.Printf("Topic: %s\n", runTopic)
		fmt.Printf("Mode: %s\n", plan.Mode)
		fmt.Printf("Rounds: %d\n", cfg.Rounds.Count)
		fmt.Printf("Sequence: %v\n", cfg.Rounds.Sequence)
		fmt.Println("Team Setup:")
		for _, tm := range plan.Teams {
			fmt.Printf("- %s: %s (side: %s)\n", config.TeamLabel(tm.ID), tm.Name, tm.Side)
			fmt.Printf("  Position: %s\n", tm.PositionDescription)
			for _, ag := range tm.Agents {
				model := ag.Model
				if model == "" {
					model = cfg.Models.Defaults[ag.Role]
				}
				fmt.Printf("  - %s (%s) -> %s\n", ag.ID, ag.Role, model)
			}
		}
		fmt.Printf("Judges: %v\n", cfg.Judges.Types)
		return nil
	}

	if err := config.ValidateAPIKey(cfg); err != nil {
		return err
	}

	deps, err := engine.NewEngineDeps(cfg)
	if err != nil {
		return fmt.Errorf("initialize engine: %w", err)
	}
	eng := engine.NewEngine(deps)

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
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

	var sessionCfg *engine.SessionConfig
	if runSaveSession != "" {
		effectiveInterval := runSaveInterval
		if effectiveInterval == 0 {
			effectiveInterval = cfg.Session.SaveInterval
		}
		if effectiveInterval == 0 {
			effectiveInterval = 30 * time.Second
		}
		sessionCfg = &engine.SessionConfig{
			SavePath:              runSaveSession,
			SaveInterval:          effectiveInterval,
			CheckpointMinInterval: cfg.Session.CheckpointMinInterval,
		}
	}

	result, err := eng.Run(ctx, runTopic, cfg, liveCh, sessionCfg)
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

	return nil
}

func setupLogger(cfg *config.Config) {
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Logging.Format == "text" {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}

func consumeLiveEvents(ch <-chan round.LiveEvent) {
	for event := range ch {
		switch event.Type {
		case round.LiveEventRoundStart:
			fmt.Printf("\n─── %s ───\n", liveRoundDisplayName(event.RoundType))
		case round.LiveEventAgentStart:
			fmt.Printf("\n[%s · %s]\n", event.AgentID, event.TeamName)
		case round.LiveEventAgentChunk:
			fmt.Print(event.Content)
		case round.LiveEventAgentDone:
			if event.Failed {
				fmt.Printf(" [FAILED]\n")
			} else {
				fmt.Println()
			}
		case round.LiveEventTeamDone:
			fmt.Printf("\n[%s complete]\n", event.TeamName)
		case round.LiveEventRoundDone:
		}
	}
}

func liveRoundDisplayName(rt config.RoundType) string {
	displayNames := map[config.RoundType]string{
		config.RoundOpening:      "Opening",
		config.RoundSteelman:     "Steelman",
		config.RoundRebuttal:     "Rebuttal",
		config.RoundCrossExam:    "Cross-Exam",
		config.RoundFactCheck:    "Fact-Check",
		config.RoundPositionSwap: "Position Swap",
		config.RoundClosing:      "Closing",
	}
	if name, ok := displayNames[rt]; ok {
		return name
	}
	return string(rt)
}
