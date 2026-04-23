package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// sentenceEndRegex matches sentence boundaries: punctuation followed by whitespace or end-of-string.
var sentenceEndRegex = regexp.MustCompile(`[.!?](\s+|\s*$)`)

// commonAbbreviations are patterns where a period does NOT end a sentence.
var commonAbbreviations = []string{
	"Dr.", "Mr.", "Mrs.", "Ms.", "Prof.", "Sr.", "Jr.",
	"Inc.", "Ltd.", "Corp.", "vs.", "etc.", "e.g.", "i.e.",
	"U.S.", "U.K.", "U.N.",
}

func isAbbreviation(text string, periodPos int) bool {
	// Check for decimal number: digit before the period
	if periodPos > 0 && text[periodPos-1] >= '0' && text[periodPos-1] <= '9' {
		return true
	}
	// Check for common abbreviations
	for _, abbr := range commonAbbreviations {
		start := periodPos - len(abbr) + 1
		if start >= 0 && strings.HasPrefix(text[start:], abbr) {
			return true
		}
	}
	return false
}

type SentenceBlocker struct {
	buffer      strings.Builder
	out         chan<- StreamChunk
	sentenceEnd *regexp.Regexp
}

func NewSentenceBlocker(out chan<- StreamChunk) *SentenceBlocker {
	return &SentenceBlocker{
		out:         out,
		sentenceEnd: sentenceEndRegex,
	}
}

func (sb *SentenceBlocker) Feed(chunk StreamChunk) {
	if chunk.Done {
		if sb.buffer.Len() > 0 {
			sb.out <- StreamChunk{Content: sb.buffer.String(), Done: false}
			sb.buffer.Reset()
		}
		sb.out <- StreamChunk{Done: true}
		return
	}

	sb.buffer.WriteString(chunk.Content)
	text := sb.buffer.String()

	loc := sb.sentenceEnd.FindStringIndex(text)
	if loc != nil {
		// Check if the punctuation that triggered this match is part of an abbreviation
		punctPos := loc[0] // position of the punctuation char
		if text[punctPos] == '.' && isAbbreviation(text, punctPos) {
			return // not a real sentence boundary
		}
		sentence := text[:loc[1]]
		sb.buffer.Reset()
		sb.buffer.WriteString(text[loc[1]:])
		sb.out <- StreamChunk{Content: sentence, Done: false}
	}
}

type ParagraphBlocker struct {
	buffer strings.Builder
	out    chan<- StreamChunk
}

func NewParagraphBlocker(out chan<- StreamChunk) *ParagraphBlocker {
	return &ParagraphBlocker{out: out}
}

func (pb *ParagraphBlocker) Feed(chunk StreamChunk) {
	if chunk.Done {
		if pb.buffer.Len() > 0 {
			pb.out <- StreamChunk{Content: pb.buffer.String(), Done: false}
			pb.buffer.Reset()
		}
		pb.out <- StreamChunk{Done: true}
		return
	}

	pb.buffer.WriteString(chunk.Content)
	text := pb.buffer.String()

	idx := strings.Index(text, "\n\n")
	if idx >= 0 {
		paragraph := text[:idx+2]
		pb.buffer.Reset()
		pb.buffer.WriteString(text[idx+2:])
		pb.out <- StreamChunk{Content: paragraph, Done: false}
	}
}

type BlockSize string

const (
	BlockSentence  BlockSize = "sentence"
	BlockParagraph BlockSize = "paragraph"
	BlockToken     BlockSize = "token"
)

type StreamOrchestrator struct {
	client    OpenRouterClient
	blockSize BlockSize
}

func NewStreamOrchestrator(client OpenRouterClient, blockSize BlockSize) *StreamOrchestrator {
	return &StreamOrchestrator{
		client:    client,
		blockSize: blockSize,
	}
}

func (so *StreamOrchestrator) StreamAgent(ctx context.Context, req CompletionRequest) (*AgentStreamResult, error) {
	req.Stream = true
	rawStream, err := so.client.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	blockedStream := make(chan StreamChunk, 16)

	go func() {
		defer close(blockedStream)
		switch so.blockSize {
		case BlockSentence:
			blocker := NewSentenceBlocker(blockedStream)
			for chunk := range rawStream {
				blocker.Feed(chunk)
			}
		case BlockParagraph:
			blocker := NewParagraphBlocker(blockedStream)
			for chunk := range rawStream {
				blocker.Feed(chunk)
			}
		default:
			for chunk := range rawStream {
				blockedStream <- chunk
			}
		}
	}()

	return &AgentStreamResult{
		Stream: blockedStream,
		Model:  req.Model,
	}, nil
}

type AgentStreamResult struct {
	Stream <-chan StreamChunk
	Model  string
}

type TerminalDisplay struct {
	width int
}

func NewTerminalDisplay(width int) *TerminalDisplay {
	if width <= 0 {
		width = 72
	}
	return &TerminalDisplay{width: width}
}

func (td *TerminalDisplay) FormatAgentBox(agentID, teamName, content string) string {
	header := fmt.Sprintf("┌─ %s (%s) ", agentID, teamName)
	if len(header)+1 > td.width {
		// Truncate header to fit within terminal width
		maxHeaderLen := td.width - 1
		if maxHeaderLen > 0 {
			header = header[:maxHeaderLen]
		}
	}
	padding := td.width - len(header) - 1
	if padding < 0 {
		padding = 0
	}
	top := header + strings.Repeat("─", padding) + "┐"

	contentWidth := td.width - 4
	if contentWidth <= 0 {
		contentWidth = 20
	}
	lines := td.wrapText(content, contentWidth)
	bottom := "└" + strings.Repeat("─", td.width-2) + "┘"

	var sb strings.Builder
	sb.WriteString(top)
	sb.WriteString("\n")
	for _, line := range lines {
		sb.WriteString("│ ")
		sb.WriteString(line)
		pad := td.width - 4 - len(line)
		if pad > 0 {
			sb.WriteString(strings.Repeat(" ", pad))
		}
		sb.WriteString(" │\n")
	}
	sb.WriteString(bottom)
	sb.WriteString("\n")

	return sb.String()
}

func (td *TerminalDisplay) wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	var lines []string
	words := strings.Fields(text)
	var current string
	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= maxWidth {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func (c *OpenRouterClientImpl) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	if err := c.RateLimiter.Wait(ctx, req.Model); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	req.Stream = true
	body, err := json.Marshal(openRouterRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	})
	if err != nil {
		return nil, &APIError{Type: "marshal", Err: err}
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, &APIError{Type: "request", Err: err}
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/poldebatecli")
	httpReq.Header.Set("X-Title", "PolDebateCLI")

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &APIError{Type: "network", Err: err}
	}

	if httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		closeErr := httpResp.Body.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("stream request failed (%d): %s (close error: %v)", httpResp.StatusCode, string(respBody), closeErr)
		}
		return nil, fmt.Errorf("stream request failed (%d): %s", httpResp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer func() { _ = httpResp.Body.Close() }()

		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				ch <- StreamChunk{Done: true, Err: ctx.Err()}
				return
			default:
			}
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- StreamChunk{Done: true}
				return
			}

			var sseResp struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
				Model string `json:"model"`
				Usage *struct {
					TotalTokens int `json:"total_tokens"`
				} `json:"usage"`
			}

			if err := json.Unmarshal([]byte(data), &sseResp); err != nil {
				slog.Warn("failed to parse SSE response", "data", data, "error", err)
				continue
			}

			if len(sseResp.Choices) > 0 {
				content := sseResp.Choices[0].Delta.Content
				if content != "" {
					ch <- StreamChunk{Content: content}
				}
				if sseResp.Choices[0].FinishReason != nil {
					tokens := 0
					if sseResp.Usage != nil {
						tokens = sseResp.Usage.TotalTokens
					}
					ch <- StreamChunk{
						Done:       true,
						Model:      sseResp.Model,
						TokensUsed: tokens,
					}
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Done: true, Err: fmt.Errorf("stream read failed: %w", err)}
			return
		}
		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}
