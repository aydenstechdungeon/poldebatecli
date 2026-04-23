package config

import "time"

func DefaultConfig() *Config {
	return &Config{
		Models: ModelsConfig{
			Defaults: map[AgentRole]string{
				RoleEconomist:  "x-ai/grok-4.20",
				RoleHistorian:  "x-ai/grok-4.1-fast",
				RoleStrategist: "openai/gpt-5-mini",
			},
			Fallbacks: map[string]string{
				"x-ai/grok-4.20":     "anthropic/claude-haiku-4.5",
				"x-ai/grok-4.1-fast": "openai/gpt-5.4-nano",
				"openai/gpt-5-mini":  "anthropic/claude-haiku-4.5",
			},
			Temperatures: map[AgentRole]float64{
				RoleEconomist:  0.7,
				RoleHistorian:  0.5,
				RoleStrategist: 0.8,
			},
			MaxTokens: map[AgentRole]int{
				RoleEconomist:  2048,
				RoleHistorian:  2048,
				RoleStrategist: 2048,
			},
			TopP: map[AgentRole]float64{
				RoleEconomist:  0.95,
				RoleHistorian:  0.90,
				RoleStrategist: 0.95,
			},
		},
		Teams: TeamsConfig{
			Count: 2,
			TeamA: TeamConfig{
				Name:                "Proponents",
				Side:                "for",
				PositionDescription: "Affirm the proposition with practical, evidence-backed policy and implementation arguments.",
				Agents: []AgentConfig{
					{ID: "economist_a", Role: RoleEconomist},
					{ID: "historian_a", Role: RoleHistorian},
					{ID: "strategist_a", Role: RoleStrategist},
				},
			},
			TeamB: TeamConfig{
				Name:                "Opponents",
				Side:                "against",
				PositionDescription: "Challenge the proposition by exposing risks, tradeoffs, and weaker assumptions.",
				Agents: []AgentConfig{
					{ID: "economist_b", Role: RoleEconomist},
					{ID: "historian_b", Role: RoleHistorian},
					{ID: "strategist_b", Role: RoleStrategist},
				},
			},
			List: []TeamConfigWithID{
				{ID: TeamA, TeamConfig: TeamConfig{
					Name:                "Proponents",
					Side:                "for",
					PositionDescription: "Affirm the proposition with practical, evidence-backed policy and implementation arguments.",
					Agents:              []AgentConfig{{ID: "economist_a", Role: RoleEconomist}, {ID: "historian_a", Role: RoleHistorian}, {ID: "strategist_a", Role: RoleStrategist}},
				}},
				{ID: TeamB, TeamConfig: TeamConfig{
					Name:                "Opponents",
					Side:                "against",
					PositionDescription: "Challenge the proposition by exposing risks, tradeoffs, and weaker assumptions.",
					Agents:              []AgentConfig{{ID: "economist_b", Role: RoleEconomist}, {ID: "historian_b", Role: RoleHistorian}, {ID: "strategist_b", Role: RoleStrategist}},
				}},
			},
		},
		Rounds: RoundsConfig{
			Count: 1,
			Sequence: []RoundType{
				RoundOpening,
				RoundSteelman,
				RoundRebuttal,
				RoundCrossExam,
				RoundFactCheck,
				RoundPositionSwap,
				RoundClosing,
			},
			Timeouts: map[RoundType]time.Duration{
				RoundOpening:      120 * time.Second,
				RoundSteelman:     120 * time.Second,
				RoundRebuttal:     120 * time.Second,
				RoundCrossExam:    180 * time.Second,
				RoundFactCheck:    120 * time.Second,
				RoundPositionSwap: 180 * time.Second,
				RoundClosing:      120 * time.Second,
			},
		},
		Judges: JudgesConfig{
			Types: []JudgeType{JudgeLogic, JudgeEvidence, JudgeClarity, JudgeAdversarial},
			Models: map[JudgeType]string{
				JudgeLogic:       "x-ai/grok-4.20",
				JudgeEvidence:    "openai/gpt-5-mini",
				JudgeClarity:     "x-ai/grok-4.1-fast",
				JudgeAdversarial: "anthropic/claude-haiku-4.5",
			},
			Fallbacks: map[string]string{
				"x-ai/grok-4.20":             "anthropic/claude-haiku-4.5",
				"x-ai/grok-4.1-fast":         "openai/gpt-5.4-nano",
				"openai/gpt-5-mini":          "anthropic/claude-haiku-4.5",
				"anthropic/claude-haiku-4.5": "openai/gpt-5.4-nano",
			},
			Temperatures: map[JudgeType]float64{
				JudgeLogic:       0.2,
				JudgeEvidence:    0.2,
				JudgeClarity:     0.3,
				JudgeAdversarial: 0.4,
			},
			AdversarialConfig: AdversarialConfig{
				BiasThreshold:        0.3,
				ContradictionPenalty: 2.0,
			},
		},
		Output: OutputConfig{
			Format:          "json",
			Path:            "./debate_output",
			Transcript:      true,
			TerminalDisplay: true,
			Streaming:       true,
			StreamBlockSize: "sentence",
		},
		APIClient: APIConfig{
			BaseURL:        "https://openrouter.ai/api/v1",
			APIKeyEnvVar:   "OPENROUTER_API_KEY",
			Timeout:        120 * time.Second,
			MaxRetries:     3,
			RetryBaseDelay: 2 * time.Second,
			RateLimitRPS:   10.0,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Session: SessionYAMLConfig{
			SaveInterval:          30 * time.Second,
			CheckpointMinInterval: 3 * time.Second,
		},
	}
}
