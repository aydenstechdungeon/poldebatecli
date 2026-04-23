package output

import (
	"context"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/engine"
)

type OutputWriter interface {
	Write(ctx context.Context, result *engine.DebateResult, cfg *config.Config) error
}

type StreamBlockSize string

const (
	BlockSentence  StreamBlockSize = "sentence"
	BlockParagraph StreamBlockSize = "paragraph"
	BlockToken     StreamBlockSize = "token"
)
