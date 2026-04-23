package client

import (
	"github.com/poldebatecli/internal/config"
)

func FindFallback(model string, fallbacks map[string]string) (string, bool) {
	if f, ok := fallbacks[model]; ok {
		return f, true
	}
	return "", false
}

func FindFallbackForRole(role config.AgentRole, cfg *config.Config) (string, bool) {
	defaultModel := cfg.Models.Defaults[role]
	return FindFallback(defaultModel, cfg.Models.Fallbacks)
}
