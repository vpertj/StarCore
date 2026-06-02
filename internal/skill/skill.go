package skill

type SkillContext struct {
	SelectedCode string   `json:"selectedCode"`
	FilePath     string   `json:"filePath"`
	FileContent  string   `json:"fileContent"`
	Diagnostics  []string `json:"diagnostics"`
	Language     string   `json:"language"`
	ProjectPath  string   `json:"projectPath"`
	ContextFiles []string `json:"contextFiles,omitempty"`
	UserInput    string   `json:"userInput,omitempty"`
}

type SkillDef struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Icon             string   `json:"icon"`
	Description      string   `json:"description"`
	Trigger          string   `json:"trigger"`
	PromptTemplate   string   `json:"promptTemplate"`
	ResultType       string   `json:"resultType"`
	AssociatedAgents []string `json:"associatedAgents"`
	Category         string   `json:"category"`
}

type SkillResult struct {
	SkillID    string `json:"skillId"`
	Content    string `json:"content"`
	ResultType string `json:"resultType"`
}
