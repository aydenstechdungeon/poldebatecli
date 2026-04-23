package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/engine"
	"github.com/poldebatecli/internal/output"
	"github.com/poldebatecli/internal/planner"
	"github.com/spf13/cobra"
)

var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Run a simulated debate with mock responses",
	Long: `Run a debate using mock/stub responses instead of real API calls.

Useful for testing pipeline logic, config validation, and output formatting
without consuming API credits.`,
	Example: `  debate simulate
  debate simulate --seed 123 --output ./test_output/
  debate simulate --topic "AI regulation is necessary"`,
	RunE: runSimulate,
}

var (
	simSeed   int
	simOutput string
	simTopic  string
)

func init() {
	rootCmd.AddCommand(simulateCmd)
	simulateCmd.Flags().IntVar(&simSeed, "seed", 42, "Random seed for reproducible mock responses (reserved for future use)")
	simulateCmd.Flags().StringVar(&simOutput, "output", "", "Output directory path")
	simulateCmd.Flags().StringVar(&simTopic, "topic", "", "Debate topic (required)")
	_ = simulateCmd.MarkFlagRequired("topic")
}

func runSimulate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if simOutput != "" {
		cfg.Output.Path = simOutput
	}
	if _, err := planner.NewRuleBasedPlanner().Plan(simTopic, cfg); err != nil {
		return fmt.Errorf("team planner: %w", err)
	}

	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	setupLogger(cfg)
	slog.Info("running simulated debate", "topic", simTopic)

	deps, err := engine.NewEngineDeps(cfg)
	if err != nil {
		return fmt.Errorf("initialize engine: %w", err)
	}
	eng := engine.NewEngine(deps)

	ctx := context.Background()
	result, err := eng.Simulate(ctx, simTopic, cfg)
	if err != nil {
		return fmt.Errorf("simulation: %w", err)
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

	fmt.Fprintf(os.Stderr, "\nSimulation complete. Results written to %s\n", cfg.Output.Path)
	return nil
}
