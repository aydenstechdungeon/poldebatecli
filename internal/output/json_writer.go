package output

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/engine"
)

type JSONWriter struct{}

func NewJSONWriter() *JSONWriter {
	return &JSONWriter{}
}

func (w *JSONWriter) Write(ctx context.Context, result *engine.DebateResult, cfg *config.Config) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal debate result: %w", err)
	}

	outputPath := cfg.Output.Path
	if outputPath == "" {
		outputPath = "./debate_output"
	}

	if err := os.MkdirAll(outputPath, 0700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	jsonPath := filepath.Join(outputPath, "result.json")
	if err := os.WriteFile(jsonPath, data, 0600); err != nil {
		return fmt.Errorf("write result file: %w", err)
	}

	return nil
}
