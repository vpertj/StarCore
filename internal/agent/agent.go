package agent

type AgentDef struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Icon         string       `json:"icon"`
	Description  string       `json:"description"`
	SystemPrompt string       `json:"systemPrompt"`
	DefaultModel string       `json:"defaultModel"`
	Tools        []string     `json:"tools"`
	Skills       []string     `json:"skills"`
	Category     string       `json:"category"`
	Capabilities []IntentType `json:"capabilities"`
	Priority     int          `json:"priority"`
}

type AgentConfig struct {
	Temperature        float64 `json:"temperature"`
	MaxTokens          int     `json:"maxTokens"`
	AutoApproveTools   bool    `json:"autoApproveTools"`
	CustomPromptAppend string  `json:"customPromptAppend"`
}

func DefaultConfig() AgentConfig {
	return AgentConfig{
		Temperature: 0.7,
		MaxTokens:   4096,
	}
}
