package config

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, err
	}

	// Unmarshal into a fresh struct, then merge non-zero fields into defaults.
	// This prevents partial config files from zeroing out default values.
	fileCfg := &Config{}
	if err := yaml.Unmarshal(data, fileCfg); err != nil {
		return nil, err
	}

	var overlay fileOverlay
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return nil, err
	}

	mergeConfig(cfg, fileCfg, &overlay)

	applyEnvOverrides(cfg)

	return cfg, nil
}

// mergeConfig overlays non-zero fields from src onto dst.
// Map fields are merged entry-by-entry so partial maps don't lose defaults.
func mergeConfig(dst, src *Config, overlay *fileOverlay) {
	mergeModelsConfig(&dst.Models, &src.Models)
	mergeTeamsConfig(&dst.Teams, &src.Teams)
	mergeRoundsConfig(&dst.Rounds, &src.Rounds)
	mergeJudgesConfig(&dst.Judges, &src.Judges)
	mergeOutputConfig(&dst.Output, &src.Output, overlay)
	mergeAPIConfig(&dst.APIClient, &src.APIClient)
	mergeLoggingConfig(&dst.Logging, &src.Logging)
	mergeSessionConfig(&dst.Session, &src.Session)
	dst.Teams.SetTeams(dst.Teams.EffectiveTeams())
}

type fileOverlay struct {
	Output *outputOverlay `yaml:"output"`
}

type outputOverlay struct {
	Transcript      *bool `yaml:"transcript"`
	TerminalDisplay *bool `yaml:"terminal_display"`
	Streaming       *bool `yaml:"streaming"`
}

func mergeModelsConfig(dst, src *ModelsConfig) {
	mergeStringRoleMap(&dst.Defaults, src.Defaults)
	mergeStringMap(&dst.Fallbacks, src.Fallbacks)
	mergeFloat64RoleMap(&dst.Temperatures, src.Temperatures)
	mergeIntRoleMap(&dst.MaxTokens, src.MaxTokens)
	mergeFloat64RoleMap(&dst.TopP, src.TopP)
	if src.ContextModel != "" {
		dst.ContextModel = src.ContextModel
	}
}

func mergeTeamsConfig(dst, src *TeamsConfig) {
	if src.Count > 0 {
		dst.Count = src.Count
	}
	mergeTeamConfig(&dst.TeamA, &src.TeamA)
	mergeTeamConfig(&dst.TeamB, &src.TeamB)
	if len(src.List) > 0 {
		dst.SetTeams(src.EffectiveTeams())
		return
	}
	teams := dst.EffectiveTeams()
	if len(teams) > 0 {
		teams[0].TeamConfig = dst.TeamA
	}
	if len(teams) > 1 {
		teams[1].TeamConfig = dst.TeamB
	}
	dst.SetTeams(teams)
}

func mergeTeamConfig(dst, src *TeamConfig) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Side != "" {
		dst.Side = src.Side
	}
	if src.PositionDescription != "" {
		dst.PositionDescription = src.PositionDescription
	}
	if len(src.Agents) > 0 {
		dst.Agents = src.Agents
	}
}

func mergeRoundsConfig(dst, src *RoundsConfig) {
	if src.Count > 0 {
		dst.Count = src.Count
	}
	if len(src.Sequence) > 0 {
		dst.Sequence = src.Sequence
	}
	mergeDurationMap(&dst.Timeouts, src.Timeouts)
}

func mergeJudgesConfig(dst, src *JudgesConfig) {
	if len(src.Types) > 0 {
		dst.Types = src.Types
	}
	mergeStringJudgeTypeMap(&dst.Models, src.Models)
	mergeStringMap(&dst.Fallbacks, src.Fallbacks)
	mergeFloat64JudgeTypeMap(&dst.Temperatures, src.Temperatures)
	if src.AdversarialConfig.BiasThreshold != 0 {
		dst.AdversarialConfig.BiasThreshold = src.AdversarialConfig.BiasThreshold
	}
	if src.AdversarialConfig.ContradictionPenalty != 0 {
		dst.AdversarialConfig.ContradictionPenalty = src.AdversarialConfig.ContradictionPenalty
	}
}

func mergeOutputConfig(dst, src *OutputConfig, overlay *fileOverlay) {
	if src.Format != "" {
		dst.Format = src.Format
	}
	if src.Path != "" {
		dst.Path = src.Path
	}
	if overlay != nil && overlay.Output != nil {
		if overlay.Output.Transcript != nil {
			dst.Transcript = src.Transcript
		}
		if overlay.Output.TerminalDisplay != nil {
			dst.TerminalDisplay = src.TerminalDisplay
		}
		if overlay.Output.Streaming != nil {
			dst.Streaming = src.Streaming
		}
	}
	if src.StreamBlockSize != "" {
		dst.StreamBlockSize = src.StreamBlockSize
	}
}

func mergeAPIConfig(dst, src *APIConfig) {
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	if src.APIKeyEnvVar != "" {
		dst.APIKeyEnvVar = src.APIKeyEnvVar
	}
	if src.Timeout != 0 {
		dst.Timeout = src.Timeout
	}
	if src.MaxRetries != 0 {
		dst.MaxRetries = src.MaxRetries
	}
	if src.RetryBaseDelay != 0 {
		dst.RetryBaseDelay = src.RetryBaseDelay
	}
	if src.RateLimitRPS != 0 {
		dst.RateLimitRPS = src.RateLimitRPS
	}
	if src.AllowUnsafeBaseURL {
		dst.AllowUnsafeBaseURL = true
	}
}

func mergeLoggingConfig(dst, src *LoggingConfig) {
	if src.Level != "" {
		dst.Level = src.Level
	}
	if src.Format != "" {
		dst.Format = src.Format
	}
}

func mergeSessionConfig(dst, src *SessionYAMLConfig) {
	if src.SaveInterval != 0 {
		dst.SaveInterval = src.SaveInterval
	}
	if src.CheckpointMinInterval != 0 {
		dst.CheckpointMinInterval = src.CheckpointMinInterval
	}
}

// Generic map merge helpers

func mergeStringMap(dst *map[string]string, src map[string]string) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[string]string)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func mergeStringRoleMap(dst *map[AgentRole]string, src map[AgentRole]string) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[AgentRole]string)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func mergeStringJudgeTypeMap(dst *map[JudgeType]string, src map[JudgeType]string) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[JudgeType]string)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func mergeFloat64RoleMap(dst *map[AgentRole]float64, src map[AgentRole]float64) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[AgentRole]float64)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func mergeFloat64JudgeTypeMap(dst *map[JudgeType]float64, src map[JudgeType]float64) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[JudgeType]float64)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func mergeIntRoleMap(dst *map[AgentRole]int, src map[AgentRole]int) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[AgentRole]int)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func mergeDurationMap(dst *map[RoundType]time.Duration, src map[RoundType]time.Duration) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[RoundType]time.Duration)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DEBATE_API_BASE_URL"); v != "" {
		cfg.APIClient.BaseURL = v
	}
	if v := os.Getenv("DEBATE_ALLOW_UNSAFE_BASE_URL"); v != "" {
		parsed, err := parseBoolEnv(v)
		if err != nil {
			slog.Warn("DEBATE_ALLOW_UNSAFE_BASE_URL must be boolean, ignoring", "value", v)
		} else {
			cfg.APIClient.AllowUnsafeBaseURL = parsed
		}
	}
	if v := os.Getenv("DEBATE_API_KEY_ENV_VAR"); v != "" {
		if !isValidEnvVarName(v) {
			slog.Warn("DEBATE_API_KEY_ENV_VAR contains invalid characters, ignoring", "value", v)
		} else {
			cfg.APIClient.APIKeyEnvVar = v
		}
	}
	if v := os.Getenv("DEBATE_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("DEBATE_OUTPUT_FORMAT"); v != "" {
		cfg.Output.Format = v
	}
	if v := os.Getenv("DEBATE_OUTPUT_PATH"); v != "" {
		cfg.Output.Path = v
	}
}

func ApplyFlagOverrides(cfg *Config, outputDir string, rounds int, noStream bool, streamBlock string, models string) {
	if outputDir != "" {
		cfg.Output.Path = outputDir
	}
	if rounds > 0 {
		cfg.Rounds.Count = rounds
	}
	if noStream {
		cfg.Output.Streaming = false
	}
	if streamBlock != "" {
		cfg.Output.StreamBlockSize = streamBlock
	}
	if models != "" {
		applyModelOverrides(cfg, models)
	}
}

func applyModelOverrides(cfg *Config, models string) {
	pairs := strings.Split(models, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			continue
		}
		role := AgentRole(strings.TrimSpace(parts[0]))
		model := strings.TrimSpace(parts[1])
		if isValidRole(role) {
			cfg.Models.Defaults[role] = model
		}
	}
}

func GetAPIKey(cfg *Config) string {
	key := os.Getenv(cfg.APIClient.APIKeyEnvVar)
	return key
}

var envVarNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func isValidEnvVarName(name string) bool {
	return envVarNameRegex.MatchString(name)
}

func parseBoolEnv(raw string) (bool, error) {
	return strconv.ParseBool(strings.TrimSpace(raw))
}

// ValidateAPIKey checks that the API key is present and doesn't match
// common placeholder values.
func ValidateAPIKey(cfg *Config) error {
	key := GetAPIKey(cfg)
	if key == "" {
		return fmt.Errorf("API key not set: environment variable %s must be defined", cfg.APIClient.APIKeyEnvVar)
	}
	placeholderValues := map[string]bool{
		"your_key_here": true,
		"your-api-key":  true,
		"sk-xxx":        true,
		"placeholder":   true,
		"changeme":      true,
	}
	if placeholderValues[strings.ToLower(key)] {
		return fmt.Errorf("API key appears to be a placeholder value, please set a real key in %s", cfg.APIClient.APIKeyEnvVar)
	}
	if len(key) < 4 {
		return fmt.Errorf("API key appears too short, please verify the value in %s", cfg.APIClient.APIKeyEnvVar)
	}
	return nil
}

func WriteDefault(path string) error {
	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
