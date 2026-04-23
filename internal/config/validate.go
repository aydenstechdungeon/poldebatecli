package config

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	MaxTeamsCount  = 32
	MaxRoundsCount = 20
)

var allowedAPIHosts = map[string]bool{
	"openrouter.ai": true,
}

func Validate(cfg *Config) error {
	var errs []string
	teams := cfg.Teams.EffectiveTeams()
	if len(teams) < 2 {
		errs = append(errs, "at least two teams are required")
	}
	if cfg.Teams.Count > 0 && cfg.Teams.Count < 2 {
		errs = append(errs, "teams.count must be >= 2")
	}
	if cfg.Teams.Count > MaxTeamsCount {
		errs = append(errs, fmt.Sprintf("teams.count must be <= %d", MaxTeamsCount))
	}

	validSides := map[string]bool{"for": true, "against": true}
	for idx, team := range teams {
		if team.ID == "" {
			errs = append(errs, fmt.Sprintf("teams[%d] is missing id", idx))
		}
		if len(team.Agents) == 0 {
			errs = append(errs, fmt.Sprintf("%s must have at least one agent", team.ID))
		}
		if !validSides[team.Side] {
			errs = append(errs, fmt.Sprintf("%s.side must be 'for' or 'against'", team.ID))
		}
	}
	var forCount, againstCount int
	agentIDs := make(map[string]bool)
	for _, team := range teams {
		if team.Side == "for" {
			forCount++
		}
		if team.Side == "against" {
			againstCount++
		}
		for _, a := range team.Agents {
			if a.ID == "" {
				errs = append(errs, fmt.Sprintf("%s agent missing id", team.ID))
			}
			if agentIDs[a.ID] {
				errs = append(errs, fmt.Sprintf("duplicate agent id: %s", a.ID))
			}
			agentIDs[a.ID] = true
			if !isValidRole(a.Role) {
				errs = append(errs, fmt.Sprintf("invalid agent role: %s", a.Role))
			}
		}
	}
	if forCount == 0 || againstCount == 0 {
		errs = append(errs, "teams must include at least one 'for' and one 'against' side")
	}

	if len(cfg.Rounds.Sequence) == 0 {
		errs = append(errs, "rounds.sequence must have at least one round")
	}
	if cfg.Rounds.Count < 1 {
		errs = append(errs, "rounds.count must be >= 1")
	}
	if cfg.Rounds.Count > MaxRoundsCount {
		errs = append(errs, fmt.Sprintf("rounds.count must be <= %d", MaxRoundsCount))
	}
	for _, rt := range cfg.Rounds.Sequence {
		if !isValidRoundType(rt) {
			errs = append(errs, fmt.Sprintf("invalid round type: %s", rt))
		}
		timeout, ok := cfg.Rounds.Timeouts[rt]
		if !ok {
			errs = append(errs, fmt.Sprintf("rounds.timeouts missing entry for round: %s", rt))
			continue
		}
		if timeout <= 0 {
			errs = append(errs, fmt.Sprintf("rounds.timeouts.%s must be > 0", rt))
		}
	}

	for _, jt := range cfg.Judges.Types {
		if !isValidJudgeType(jt) {
			errs = append(errs, fmt.Sprintf("invalid judge type: %s", jt))
		}
	}

	if cfg.APIClient.BaseURL == "" {
		errs = append(errs, "api_client.base_url is required")
	} else if err := validateAPIBaseURL(cfg.APIClient.BaseURL, cfg.APIClient.AllowUnsafeBaseURL); err != nil {
		errs = append(errs, err.Error())
	}
	if cfg.APIClient.APIKeyEnvVar == "" {
		errs = append(errs, "api_client.api_key_env_var is required")
	}
	if cfg.APIClient.MaxRetries < 0 {
		errs = append(errs, "api_client.max_retries must be >= 0")
	}
	if cfg.APIClient.RateLimitRPS <= 0 {
		errs = append(errs, "api_client.rate_limit_rps must be > 0")
	}

	validOutputFormats := map[string]bool{"json": true, "text": true}
	if !validOutputFormats[cfg.Output.Format] {
		errs = append(errs, "output.format must be 'json' or 'text'")
	}

	validBlockSizes := map[string]bool{"sentence": true, "paragraph": true, "token": true}
	if !validBlockSizes[cfg.Output.StreamBlockSize] {
		errs = append(errs, "output.stream_block_size must be 'sentence', 'paragraph', or 'token'")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.Logging.Level] {
		errs = append(errs, "logging.level must be 'debug', 'info', 'warn', or 'error'")
	}

	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[cfg.Logging.Format] {
		errs = append(errs, "logging.format must be 'json' or 'text'")
	}

	if cfg.Judges.AdversarialConfig.BiasThreshold < 0 || cfg.Judges.AdversarialConfig.BiasThreshold > 1 {
		errs = append(errs, "judges.adversarial_config.bias_threshold must be between 0 and 1")
	}
	if cfg.Judges.AdversarialConfig.ContradictionPenalty < 0 {
		errs = append(errs, "judges.adversarial_config.contradiction_penalty must be >= 0")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func validateAPIBaseURL(raw string, allowUnsafe bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("api_client.base_url must be a valid absolute URL")
	}

	if allowUnsafe {
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("api_client.base_url must use https unless api_client.allow_unsafe_base_url is true")
	}
	if !allowedAPIHosts[strings.ToLower(u.Hostname())] {
		return fmt.Errorf("api_client.base_url host %q is not allowed unless api_client.allow_unsafe_base_url is true", u.Hostname())
	}
	return nil
}

func isValidRole(r AgentRole) bool {
	switch r {
	case RoleEconomist, RoleHistorian, RoleStrategist:
		return true
	}
	return false
}

func isValidRoundType(rt RoundType) bool {
	switch rt {
	case RoundOpening, RoundSteelman, RoundRebuttal, RoundCrossExam,
		RoundFactCheck, RoundPositionSwap, RoundClosing:
		return true
	}
	return false
}

func isValidJudgeType(jt JudgeType) bool {
	switch jt {
	case JudgeLogic, JudgeEvidence, JudgeClarity, JudgeAdversarial:
		return true
	}
	return false
}
