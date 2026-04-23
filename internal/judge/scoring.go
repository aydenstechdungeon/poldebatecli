package judge

import "github.com/poldebatecli/internal/config"

type AggregatedScores struct {
	Teams map[config.TeamID]TeamAggregated `json:"teams"`
}

type TeamAggregated struct {
	LogicalConsistency float64                      `json:"logical_consistency"`
	EvidenceQuality    float64                      `json:"evidence_quality"`
	Responsiveness     float64                      `json:"responsiveness"`
	StrategicStrength  float64                      `json:"strategic_strength"`
	Total              float64                      `json:"total"`
	PerRound           map[config.RoundType]float64 `json:"per_round"`
}

func AggregateScores(judgeResults []JudgeResult) AggregatedScores {
	if len(judgeResults) == 0 {
		return AggregatedScores{Teams: map[config.TeamID]TeamAggregated{}}
	}

	totals := make(map[config.TeamID]TeamAggregated)
	roundAggregates := make(map[config.TeamID]map[config.RoundType]struct {
		sum   float64
		count int
	})
	counts := make(map[config.TeamID]float64)

	for _, jr := range judgeResults {
		for teamID, sb := range jr.Scores.Teams {
			t := totals[teamID]
			t.LogicalConsistency += sb.LogicalConsistency
			t.EvidenceQuality += sb.EvidenceQuality
			t.Responsiveness += sb.Responsiveness
			t.StrategicStrength += sb.StrategicStrength
			totals[teamID] = t
			counts[teamID] += 1

			if jr.RoundType != "" {
				if _, ok := roundAggregates[teamID]; !ok {
					roundAggregates[teamID] = make(map[config.RoundType]struct {
						sum   float64
						count int
					})
				}
				ra := roundAggregates[teamID][jr.RoundType]
				ra.sum += sb.LogicalConsistency + sb.EvidenceQuality + sb.Responsiveness + sb.StrategicStrength
				ra.count++
				roundAggregates[teamID][jr.RoundType] = ra
			}
		}
	}

	agg := AggregatedScores{Teams: make(map[config.TeamID]TeamAggregated, len(totals))}
	for teamID, t := range totals {
		n := counts[teamID]
		if n == 0 {
			continue
		}
		totalSum := t.LogicalConsistency + t.EvidenceQuality + t.Responsiveness + t.StrategicStrength
		perRound := make(map[config.RoundType]float64)
		for rt, ra := range roundAggregates[teamID] {
			if ra.count > 0 {
				perRound[rt] = ra.sum / float64(ra.count) / 4
			}
		}
		agg.Teams[teamID] = TeamAggregated{
			LogicalConsistency: t.LogicalConsistency / n,
			EvidenceQuality:    t.EvidenceQuality / n,
			Responsiveness:     t.Responsiveness / n,
			StrategicStrength:  t.StrategicStrength / n,
			Total:              totalSum / n / 4,
			PerRound:           perRound,
		}
	}
	return agg
}

func DetermineWinner(scores AggregatedScores) config.TeamID {
	if len(scores.Teams) == 0 {
		return config.Tie
	}
	var winner config.TeamID
	best := -1.0
	tie := false
	for teamID, score := range scores.Teams {
		if score.Total > best {
			winner = teamID
			best = score.Total
			tie = false
			continue
		}
		if score.Total == best {
			tie = true
		}
	}
	if tie {
		return config.Tie
	}
	return winner
}
