package tools

import (
	"context"
	"fmt"
	"strings"

	"StarCore/internal/agent"
	"StarCore/internal/skill"
)

// SkillToolRegistry is set by app.go to give the Skill tool access to the skill system.
var SkillToolRegistry *skill.Registry
var SkillToolExecutor *skill.Executor

type SkillTool struct{}

func NewSkillTool() *SkillTool { return &SkillTool{} }

func (t *SkillTool) ID() string             { return "skill" }
func (t *SkillTool) Name() string           { return "Invoke Skill" }
func (t *SkillTool) RequiresApproval() bool { return false }

func (t *SkillTool) Description() string {
	return "Invoke a StarCore skill by its ID. Use this to delegate work to specialized skills like code-review, refactor, explain-code, etc. Pass the skill ID and optionally additional context for the skill."
}

func (t *SkillTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"skillId":   {Type: "string", Description: "The ID of the skill to invoke (e.g., 'code-review', 'refactor', 'explain-code')"},
			"userInput": {Type: "string", Description: "Additional context or specific instructions for the skill"},
		},
		Required: []string{"skillId"},
	}
}

func (t *SkillTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if SkillToolRegistry == nil || SkillToolExecutor == nil {
		return "", fmt.Errorf("skill system not available")
	}

	skillID, ok := args["skillId"].(string)
	if !ok || skillID == "" {
		return "", fmt.Errorf("skillId is required")
	}

	sk, ok := SkillToolRegistry.Get(skillID)
	if !ok {
		// List available skills for the error message
		allSkills := SkillToolRegistry.List()
		ids := make([]string, 0, len(allSkills))
		for _, s := range allSkills {
			ids = append(ids, s.ID)
		}
		return "", fmt.Errorf("skill not found: %s. Available skills: %s", skillID, strings.Join(ids, ", "))
	}

	userInput, _ := args["userInput"].(string)

	sctx := skill.SkillContext{
		UserInput: userInput,
	}

	// Execute requested skill with limited read-only agent loop (max 3 rounds)
	resolvedProviderID := ""
	model := ""
	if SkillToolExecutor != nil {
		eventCh, err := SkillToolExecutor.Execute(ctx, sk.ID, sctx, resolvedProviderID, model)
		if err != nil {
			return "", fmt.Errorf("skill %s failed: %s", skillID, err.Error())
		}
		var result strings.Builder
		round := 0
		const maxChainedRounds = 4
		for event := range eventCh {
			switch event.Type {
			case "data":
				result.WriteString(event.Content)
			case "tool_call":
				round++
				if round >= maxChainedRounds {
					result.WriteString("\n[Skill reached max chained rounds]")
					// drain remaining events
					for range eventCh {}
				}
			case "error":
				return "", fmt.Errorf("skill error: %s", event.Content)
			case "done":
				// finished
			}
		}
		output := result.String()
		if output == "" {
			return fmt.Sprintf("Skill %s completed.", sk.Name), nil
		}
		return output, nil
	}

	return "", fmt.Errorf("skill executor not available")
}
