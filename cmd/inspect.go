package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/planner"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect a past debate result or current configuration",
	Long: `Inspect a past debate result or current configuration.

Displays structured information about debate results, configuration state,
or model availability.`,
	Example: `  debate inspect --result ./debate_output/result.json
  debate inspect --models
  debate inspect --config ./my_debate.yaml --format json`,
	RunE: runInspect,
}

var (
	inspectResult string
	inspectModels bool
	inspectFormat string
)

func init() {
	rootCmd.AddCommand(inspectCmd)
	inspectCmd.Flags().StringVar(&inspectResult, "result", "", "Path to a past debate result JSON file")
	inspectCmd.Flags().BoolVar(&inspectModels, "models", false, "List available models from OpenRouter (requires API key)")
	inspectCmd.Flags().StringVar(&inspectFormat, "format", "table", "Output format: table, json, yaml")
}

func runInspect(cmd *cobra.Command, args []string) error {
	validFormats := map[string]bool{"table": true, "json": true, "yaml": true}
	if !validFormats[inspectFormat] {
		return fmt.Errorf("invalid format %q: must be table, json, or yaml", inspectFormat)
	}

	if inspectResult != "" {
		return inspectResultFile(inspectResult)
	}

	if inspectModels {
		return inspectModelsList()
	}

	return inspectConfig()
}

func inspectResultFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read result file: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse result JSON: %w", err)
	}

	switch inspectFormat {
	case "json":
		indented, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(indented))
	case "yaml":
		yamlData, err := yaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal result to yaml: %w", err)
		}
		fmt.Print(string(yamlData))
	default:
		printResultSummary(result)
	}

	return nil
}

func printResultSummary(result map[string]interface{}) {
	if topic, ok := result["topic"].(string); ok {
		fmt.Printf("Topic:    %s\n", topic)
	}
	if winner, ok := result["winner"].(string); ok {
		fmt.Printf("Winner:   %s\n", winner)
	}
	if metadata, ok := result["metadata"].(map[string]interface{}); ok {
		if ts, ok := metadata["timestamp"].(string); ok {
			fmt.Printf("Date:     %s\n", ts)
		}
		if dur, ok := metadata["duration"].(string); ok {
			fmt.Printf("Duration: %s\n", dur)
		}
		if tokens, ok := metadata["total_tokens"].(float64); ok {
			fmt.Printf("Tokens:   %d\n", int(tokens))
		}
	}

	if scores, ok := result["scores"].(map[string]interface{}); ok {
		fmt.Println("\nScores:")
		if teams, ok := scores["teams"].(map[string]interface{}); ok {
			for teamID, raw := range teams {
				teamScores, _ := raw.(map[string]interface{})
				fmt.Printf("  %s: total=%.1f\n", teamID, asFloat(teamScores["total"]))
			}
		} else {
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "  Metric\tTeam A\tTeam B")
			_, _ = fmt.Fprintln(tw, "  ─────\t──────\t──────")
			printScoreRow(tw, scores, "logical_consistency", "Logical Cons.")
			printScoreRow(tw, scores, "evidence_quality", "Evidence")
			printScoreRow(tw, scores, "responsiveness", "Responsiveness")
			printScoreRow(tw, scores, "strategic_strength", "Strategic")
			printScoreRow(tw, scores, "total", "TOTAL")
			_ = tw.Flush()
		}
	}

	if rounds, ok := result["rounds"].([]interface{}); ok {
		fmt.Printf("\nRounds: %d\n", len(rounds))
		for i, r := range rounds {
			if rm, ok := r.(map[string]interface{}); ok {
				if rt, ok := rm["type"].(string); ok {
					msgCount := 0
					if msgs, ok := rm["messages"].([]interface{}); ok {
						msgCount = len(msgs)
					}
					fmt.Printf("  %d. %s (%d messages)\n", i+1, rt, msgCount)
				}
			}
		}
	}

	if degraded, ok := result["degraded"].([]interface{}); ok && len(degraded) > 0 {
		fmt.Printf("\nDegraded responses: %d\n", len(degraded))
		for _, d := range degraded {
			if dm, ok := d.(map[string]interface{}); ok {
				agentID, _ := dm["agent_id"].(string)
				reason, _ := dm["reason"].(string)
				fmt.Printf("  - %s: %s\n", agentID, reason)
			}
		}
	}
}

func printScoreRow(tw *tabwriter.Writer, scores map[string]interface{}, field, label string) {
	var teamA, teamB float64
	if ta, ok := scores["team_a"].(map[string]interface{}); ok {
		if v, ok := ta[field].(float64); ok {
			teamA = v
		}
	}
	if tb, ok := scores["team_b"].(map[string]interface{}); ok {
		if v, ok := tb[field].(float64); ok {
			teamB = v
		}
	}
	_, _ = fmt.Fprintf(tw, "  %s\t%.1f\t%.1f\n", label, teamA, teamB)
}

func inspectModelsList() error {
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := config.ValidateAPIKey(cfg); err != nil {
		return err
	}

	c := client.NewOpenRouterClient(cfg.APIClient)
	models, err := c.ListModels(context.Background())
	if err != nil {
		return fmt.Errorf("fetch models: %w", err)
	}

	switch inspectFormat {
	case "json":
		data, _ := json.MarshalIndent(models, "", "  ")
		fmt.Println(string(data))
	case "yaml":
		yamlData, err := yaml.Marshal(models)
		if err != nil {
			return fmt.Errorf("marshal models to yaml: %w", err)
		}
		fmt.Print(string(yamlData))
	default:
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "  ID\tName\tContext")
		_, _ = fmt.Fprintln(tw, "  ──\t────\t───────")
		for _, m := range models {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%d\n", m.ID, m.Name, m.ContextLength)
		}
		_ = tw.Flush()
	}

	return nil
}

func inspectConfig() error {
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	switch inspectFormat {
	case "json":
		data, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(data))
	case "yaml":
		yamlData, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshal config to yaml: %w", err)
		}
		fmt.Print(string(yamlData))
	default:
		printConfigSummary(cfg)
	}

	return nil
}

func printConfigSummary(cfg *config.Config) {
	plan, _ := planner.NewRuleBasedPlanner().Plan("", cfg)
	fmt.Printf("Config:      %s\n", getConfigPath())
	fmt.Printf("\nTeams:\n")
	if plan != nil {
		fmt.Printf("  Mode: %s\n", plan.Mode)
		for _, tm := range plan.Teams {
			fmt.Printf("  %s: %s (side: %s, agents: %d)\n", config.TeamLabel(tm.ID), tm.Name, tm.Side, len(tm.Agents))
			fmt.Printf("    Position: %s\n", tm.PositionDescription)
			for _, a := range tm.Agents {
				model := a.Model
				if model == "" {
					model = cfg.Models.Defaults[a.Role]
				}
				fmt.Printf("    - %s (%s) → %s\n", a.ID, a.Role, model)
			}
		}
	}

	fmt.Printf("\nRounds:\n")
	fmt.Printf("  Cycles:   %d\n", cfg.Rounds.Count)
	fmt.Printf("  Sequence: %s\n", strings.Join(func() []string {
		s := make([]string, len(cfg.Rounds.Sequence))
		for i, rt := range cfg.Rounds.Sequence {
			s[i] = string(rt)
		}
		return s
	}(), " → "))

	fmt.Printf("\nJudges: %s\n", strings.Join(func() []string {
		s := make([]string, len(cfg.Judges.Types))
		for i, jt := range cfg.Judges.Types {
			s[i] = string(jt)
		}
		return s
	}(), ", "))

	fmt.Printf("\nOutput:\n")
	fmt.Printf("  Format:     %s\n", cfg.Output.Format)
	fmt.Printf("  Path:       %s\n", cfg.Output.Path)
	fmt.Printf("  Transcript: %v\n", cfg.Output.Transcript)
	fmt.Printf("  Streaming:  %v\n", cfg.Output.Streaming)

	fmt.Printf("\nAPI:\n")
	fmt.Printf("  Base URL:     %s\n", cfg.APIClient.BaseURL)
	fmt.Printf("  Key env var:  %s\n", cfg.APIClient.APIKeyEnvVar)
	fmt.Printf("  Max retries:  %d\n", cfg.APIClient.MaxRetries)
	fmt.Printf("  Rate limit:   %.1f RPS\n", cfg.APIClient.RateLimitRPS)
}

func asFloat(v interface{}) float64 {
	f, _ := v.(float64)
	return f
}
