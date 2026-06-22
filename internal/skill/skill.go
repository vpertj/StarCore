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

type SkillPipelineStep struct {
	SkillID   string `json:"skillId"`
	Condition string `json:"condition,omitempty"`
	InputFrom string `json:"inputFrom,omitempty"`
	Optional  bool   `json:"optional,omitempty"`
}

type SkillPipeline struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Steps       []SkillPipelineStep `json:"steps"`
}

type PipelineStepResult struct {
	StepIndex int          `json:"stepIndex"`
	SkillID   string       `json:"skillId"`
	Result    *SkillResult `json:"result"`
	Skipped   bool         `json:"skipped,omitempty"`
	Error     string       `json:"error,omitempty"`
}

type PipelineExecutionResult struct {
	PipelineID string               `json:"pipelineId"`
	Steps      []PipelineStepResult `json:"steps"`
	Success    bool                 `json:"success"`
	Error      string               `json:"error,omitempty"`
}
