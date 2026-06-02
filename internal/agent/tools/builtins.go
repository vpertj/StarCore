package tools

import "StarCore/internal/agent"

func AllTools() []agent.Tool {
	return []agent.Tool{
		NewReadFileTool(),
		NewWriteFileTool(),
		NewEditFileTool(),
		NewSearchFilesTool(),
		NewGlobTool(),
		NewListDirectoryTool(),
		NewExecuteCommandTool(),
		NewGetDiagnosticsTool(),
		NewGetGitDiffTool(),
		NewGitCommitTool(),
		NewGitPullTool(),
		NewGitPushTool(),
		NewHTTPRequestTool(),
		NewWebFetchTool(),
		NewSubAgentTool(),
	}
}
