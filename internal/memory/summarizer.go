package memory

import "strings"

type Summarizer struct {
	maxTokens int
}

func NewSummarizer() *Summarizer {
	return &Summarizer{maxTokens: 4000}
}

func (s *Summarizer) Summarize(messages []Message) string {
	totalLen := 0
	for _, m := range messages {
		totalLen += len(m.Content)
	}
	if totalLen < s.maxTokens {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("摘要：\n")
	recentCount := 5
	cutoff := len(messages) - recentCount
	if cutoff < 0 {
		cutoff = 0
	}
	for i, m := range messages {
		if i < cutoff {
			trunc := m.Content
			if len(trunc) > 100 {
				trunc = trunc[:100] + "..."
			}
			sb.WriteString("[" + m.Role + "] " + trunc + "\n")
		}
	}
	sb.WriteString("--- 最近对话 ---\n")
	for i := cutoff; i < len(messages); i++ {
		m := messages[i]
		sb.WriteString("[" + m.Role + "] " + m.Content + "\n")
	}
	return sb.String()
}
