package prompt

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/poldebatecli/internal/config"
)

var RoleDescriptions = map[string]string{
	"economist":  "economic analysis, cost-benefit reasoning, market dynamics, fiscal impact, resource allocation",
	"historian":  "historical precedent, longitudinal patterns, institutional evolution, path dependency, civilizational outcomes",
	"strategist": "strategic positioning, coalition dynamics, implementation feasibility, adversarial analysis, second-order effects",
}

type TemplateEngine struct {
	templates *template.Template
}

func sanitizeTemplateInput(v any) string {
	s := fmt.Sprint(v)
	s = strings.ReplaceAll(s, "{{", "")
	s = strings.ReplaceAll(s, "}}", "")
	return s
}

func NewTemplateEngine(templatesFS embed.FS) (*TemplateEngine, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"sanitize": sanitizeTemplateInput,
	}).ParseFS(templatesFS, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("template parsing failed: %w", err)
	}

	// Verify all expected templates are present
	expectedTemplates := []string{
		"system.tmpl", "opening.tmpl", "steelman.tmpl", "rebuttal.tmpl",
		"cross_exam.tmpl", "fact_check.tmpl", "position_swap.tmpl", "closing.tmpl",
		"judge_logic.tmpl", "judge_evidence.tmpl", "judge_clarity.tmpl", "judge_adversarial.tmpl",
	}
	for _, name := range expectedTemplates {
		if tmpl.Lookup(name) == nil {
			return nil, fmt.Errorf("required template %q not found", name)
		}
	}

	return &TemplateEngine{templates: tmpl}, nil
}

type SystemPromptParams struct {
	AgentID             string
	Role                string
	RoleDescription     string
	TeamName            string
	Side                string
	PositionDescription string
	Topic               string
	RoundName           string
	RoundDescription    string
	ContextSummary      string
}

func (e *TemplateEngine) BuildSystemPrompt(p SystemPromptParams) (string, error) {
	var buf bytes.Buffer
	if err := e.templates.ExecuteTemplate(&buf, "system.tmpl", p); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type RoundPromptParams struct {
	AgentID             string
	Role                string
	RoleDescription     string
	TeamName            string
	Side                string
	PositionDescription string
	Topic               string
	RoundName           string
	RoundDescription    string
	ContextSummary      string
	OppositeSide        string
	FactCheckData       []string
}

func (e *TemplateEngine) BuildOpeningPrompt(p RoundPromptParams) (string, error) {
	return e.buildRoundPrompt("opening.tmpl", p)
}

func (e *TemplateEngine) BuildSteelmanPrompt(p RoundPromptParams) (string, error) {
	return e.buildRoundPrompt("steelman.tmpl", p)
}

func (e *TemplateEngine) BuildRebuttalPrompt(p RoundPromptParams) (string, error) {
	return e.buildRoundPrompt("rebuttal.tmpl", p)
}

func (e *TemplateEngine) BuildCrossExamPrompt(p RoundPromptParams) (string, error) {
	return e.buildRoundPrompt("cross_exam.tmpl", p)
}

func (e *TemplateEngine) BuildFactCheckPrompt(p RoundPromptParams) (string, error) {
	return e.buildRoundPrompt("fact_check.tmpl", p)
}

func (e *TemplateEngine) BuildPositionSwapPrompt(p RoundPromptParams) (string, error) {
	return e.buildRoundPrompt("position_swap.tmpl", p)
}

func (e *TemplateEngine) BuildClosingPrompt(p RoundPromptParams) (string, error) {
	return e.buildRoundPrompt("closing.tmpl", p)
}

func (e *TemplateEngine) BuildRoundPrompt(roundName string, p RoundPromptParams) (string, error) {
	tmplName := roundTmplName(roundName)
	if tmplName == "" {
		return "", fmt.Errorf("unknown round type: %s", roundName)
	}
	return e.buildRoundPrompt(tmplName, p)
}

func (e *TemplateEngine) buildRoundPrompt(tmplName string, p RoundPromptParams) (string, error) {
	var buf bytes.Buffer
	if err := e.templates.ExecuteTemplate(&buf, tmplName, p); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type JudgePromptParams struct {
	RoundName            string
	Topic                string
	TeamSections         string
	TeamIDs              []config.TeamID
	ContradictionPenalty float64
	BiasThreshold        float64
}

func (e *TemplateEngine) BuildJudgeLogicPrompt(p JudgePromptParams) (string, error) {
	return e.buildJudgePrompt("judge_logic.tmpl", p)
}

func (e *TemplateEngine) BuildJudgeEvidencePrompt(p JudgePromptParams) (string, error) {
	return e.buildJudgePrompt("judge_evidence.tmpl", p)
}

func (e *TemplateEngine) BuildJudgeClarityPrompt(p JudgePromptParams) (string, error) {
	return e.buildJudgePrompt("judge_clarity.tmpl", p)
}

func (e *TemplateEngine) BuildJudgeAdversarialPrompt(p JudgePromptParams) (string, error) {
	return e.buildJudgePrompt("judge_adversarial.tmpl", p)
}

func (e *TemplateEngine) BuildJudgePrompt(judgeType string, p JudgePromptParams) (string, error) {
	tmplName := judgeTmplName(judgeType)
	if tmplName == "" {
		return "", fmt.Errorf("unknown judge type: %s", judgeType)
	}
	return e.buildJudgePrompt(tmplName, p)
}

func (e *TemplateEngine) buildJudgePrompt(tmplName string, p JudgePromptParams) (string, error) {
	var buf bytes.Buffer
	if err := e.templates.ExecuteTemplate(&buf, tmplName, p); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func roundTmplName(roundName string) string {
	switch roundName {
	case "opening":
		return "opening.tmpl"
	case "steelman":
		return "steelman.tmpl"
	case "rebuttal":
		return "rebuttal.tmpl"
	case "cross_exam", "cross_examination":
		return "cross_exam.tmpl"
	case "fact_check":
		return "fact_check.tmpl"
	case "position_swap":
		return "position_swap.tmpl"
	case "closing":
		return "closing.tmpl"
	default:
		return ""
	}
}

func judgeTmplName(judgeType string) string {
	switch judgeType {
	case "logic":
		return "judge_logic.tmpl"
	case "evidence":
		return "judge_evidence.tmpl"
	case "clarity":
		return "judge_clarity.tmpl"
	case "adversarial":
		return "judge_adversarial.tmpl"
	default:
		return ""
	}
}
