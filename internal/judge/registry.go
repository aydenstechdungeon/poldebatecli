package judge

import (
	"sort"

	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/prompt"
)

type Registry struct {
	judges         map[config.JudgeType]Judge
	cfg            *config.Config
	client         *client.OpenRouterClientImpl
	templateEngine *prompt.TemplateEngine
}

func NewRegistry(cfg *config.Config, c *client.OpenRouterClientImpl, te *prompt.TemplateEngine) *Registry {
	r := &Registry{
		judges:         make(map[config.JudgeType]Judge),
		cfg:            cfg,
		client:         c,
		templateEngine: te,
	}

	for _, jt := range cfg.Judges.Types {
		j := r.createJudge(jt)
		if j != nil {
			r.judges[jt] = j
		}
	}

	return r
}

func (r *Registry) createJudge(jt config.JudgeType) Judge {
	model := r.cfg.Judges.Models[jt]
	temp := r.cfg.Judges.Temperatures[jt]
	if temp == 0 {
		temp = 0.2
	}

	bj := baseJudge{
		judgeType:      jt,
		model:          model,
		temperature:    temp,
		client:         r.client,
		templateEngine: r.templateEngine,
	}

	switch jt {
	case config.JudgeLogic:
		return &LogicJudge{baseJudge: bj}
	case config.JudgeEvidence:
		return &EvidenceJudge{baseJudge: bj}
	case config.JudgeClarity:
		return &ClarityJudge{baseJudge: bj}
	case config.JudgeAdversarial:
		return &AdversarialJudge{
			baseJudge:            bj,
			biasThreshold:        r.cfg.Judges.AdversarialConfig.BiasThreshold,
			contradictionPenalty: r.cfg.Judges.AdversarialConfig.ContradictionPenalty,
		}
	default:
		return nil
	}
}

func (r *Registry) Get(jt config.JudgeType) (Judge, bool) {
	j, ok := r.judges[jt]
	return j, ok
}

func (r *Registry) All() []Judge {
	judgeTypes := make([]config.JudgeType, 0, len(r.judges))
	for jt := range r.judges {
		judgeTypes = append(judgeTypes, jt)
	}
	sort.Slice(judgeTypes, func(i, j int) bool {
		return judgeTypes[i] < judgeTypes[j]
	})

	result := make([]Judge, 0, len(judgeTypes))
	for _, jt := range judgeTypes {
		result = append(result, r.judges[jt])
	}
	return result
}
