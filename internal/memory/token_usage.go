package memory

import (
	"time"
)

type TokenUsageEntry struct {
	ID             string  `json:"id"`
	ConversationID string  `json:"conversationId"`
	ProviderID     string  `json:"providerId"`
	Model          string  `json:"model"`
	TokensIn       int     `json:"tokensIn"`
	TokensOut      int     `json:"tokensOut"`
	Cost           float64 `json:"cost"`
	CreatedAt      string  `json:"createdAt"`
}

type TokenUsageStats struct {
	TotalTokensIn  int                       `json:"totalTokensIn"`
	TotalTokensOut int                       `json:"totalTokensOut"`
	TotalCost      float64                   `json:"totalCost"`
	ByProvider     map[string]ProviderUsage  `json:"byProvider"`
}

type ProviderUsage struct {
	TokensIn  int     `json:"tokensIn"`
	TokensOut int     `json:"tokensOut"`
	Cost      float64 `json:"cost"`
}

func (s *Store) SaveTokenUsage(entry *TokenUsageEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO token_usage
		(id, conversation_id, provider_id, model, tokens_in, tokens_out, cost, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.ConversationID, entry.ProviderID, entry.Model,
		entry.TokensIn, entry.TokensOut, entry.Cost, entry.CreatedAt)
	return err
}

func (s *Store) GetTokenUsage(projectPath string, period string) (*TokenUsageStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	since := periodToTime(period)
	stats := &TokenUsageStats{
		ByProvider: make(map[string]ProviderUsage),
	}
	query := `SELECT tu.provider_id, SUM(tu.tokens_in), SUM(tu.tokens_out), SUM(tu.cost)
		FROM token_usage tu
		JOIN conversations c ON tu.conversation_id = c.id
		WHERE c.project_path = ? AND tu.created_at >= ?
		GROUP BY tu.provider_id`
	rows, err := s.db.Query(query, projectPath, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var providerID string
		var tokensIn, tokensOut int
		var cost float64
		if err := rows.Scan(&providerID, &tokensIn, &tokensOut, &cost); err != nil {
			return nil, err
		}
		stats.ByProvider[providerID] = ProviderUsage{
			TokensIn:  tokensIn,
			TokensOut: tokensOut,
			Cost:      cost,
		}
		stats.TotalTokensIn += tokensIn
		stats.TotalTokensOut += tokensOut
		stats.TotalCost += cost
	}
	return stats, rows.Err()
}

func periodToTime(period string) string {
	now := time.Now()
	var t time.Time
	switch period {
	case "today":
		y, m, d := now.Date()
		t = time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	case "week":
		t = now.AddDate(0, 0, -7)
	case "month":
		t = now.AddDate(0, -1, 0)
	case "all":
		t = time.Time{}
	default:
		t = time.Time{}
	}
	return t.Format(time.RFC3339)
}
