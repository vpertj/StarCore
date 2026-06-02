package memory

type ContextInfo struct {
	CurrentFile    string   `json:"currentFile"`
	SelectedCode   string   `json:"selectedCode"`
	Diagnostics    []string `json:"diagnostics"`
	ProjectPath    string   `json:"projectPath"`
	KnowledgeHints []string `json:"knowledgeHints"`
}

type ContextCollector struct {
	store *Store
}

func NewContextCollector(store *Store) *ContextCollector {
	return &ContextCollector{store: store}
}

func (c *ContextCollector) Collect(projectPath string, filePath string, selectedCode string, diagnostics []string) (*ContextInfo, error) {
	info := &ContextInfo{
		CurrentFile:  filePath,
		SelectedCode: selectedCode,
		Diagnostics:  diagnostics,
		ProjectPath:  projectPath,
	}
	entries, err := c.store.GetKnowledge(projectPath)
	if err != nil {
		return info, err
	}
	hints := make([]string, 0, len(entries))
	for _, e := range entries {
		hints = append(hints, e.Category+":"+e.Key+"="+e.Value)
	}
	info.KnowledgeHints = hints
	return info, nil
}
