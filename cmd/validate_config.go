package cmd

import (
	"fmt"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/planner"
	"github.com/spf13/cobra"
)

var validateConfigCmd = &cobra.Command{
	Use:   "validate-config",
	Short: "Validate a debate configuration file",
	Long: `Validate a debate configuration file without executing a debate.

Checks: required fields, model availability, role assignments, round sequence
validity, timeout ranges, and judge type recognition.`,
	Example: `  debate validate-config --config ./my_debate.yaml
  debate validate-config`,
	RunE: runValidateConfig,
}

func init() {
	rootCmd.AddCommand(validateConfigCmd)
}

func runValidateConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := config.Validate(cfg); err != nil {
		return err
	}
	plan, err := planner.NewRuleBasedPlanner().Plan("", cfg)
	if err != nil {
		return fmt.Errorf("team planner: %w", err)
	}

	fmt.Println("Configuration is valid.")
	fmt.Printf("Mode: %s\n", plan.Mode)
	fmt.Println("Team Setup:")
	const maxPrintedTeams = 20
	for i, tm := range plan.Teams {
		if i >= maxPrintedTeams {
			fmt.Printf("... and %d more teams\n", len(plan.Teams)-maxPrintedTeams)
			break
		}
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
	fmt.Printf("Rounds: %d cycle(s), %d round type(s)\n", cfg.Rounds.Count, len(cfg.Rounds.Sequence))
	fmt.Printf("Judges: %d type(s)\n", len(cfg.Judges.Types))
	return nil
}
