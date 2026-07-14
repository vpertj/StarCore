package ai

import (
	"regexp"
	"strings"

	"StarCore/internal/agent"
	"StarCore/internal/provider"
)

type TaskComplexity int

const (
	ComplexitySimple TaskComplexity = iota
	ComplexityModerate
	ComplexityComplex
)

type SubTaskSpec struct {
	Description string
	AgentID     string
	Files       []string
	DependsOn   []int
}

type TaskRoute struct {
	Complexity TaskComplexity
	Intent     *agent.IntentResult
	Route      string
	SubTasks   []SubTaskSpec
}

type TaskRouter struct{}

func NewTaskRouter() *TaskRouter {
	return &TaskRouter{}
}

func (r *TaskRouter) Route(messages []provider.Message, intent *agent.IntentResult) *TaskRoute {
	if intent == nil {
		return &TaskRoute{Complexity: ComplexitySimple, Route: "direct"}
	}

	complexity := evaluateComplexity(messages, intent)

	switch complexity {
	case ComplexitySimple:
		return &TaskRoute{Complexity: complexity, Intent: intent, Route: "direct"}
	case ComplexityModerate:
		return &TaskRoute{Complexity: complexity, Intent: intent, Route: "agent"}
	case ComplexityComplex:
		lastMsg := getLastUserMessage(messages)
		subTasks := decomposeTask(lastMsg, intent)
		if len(subTasks) > 0 {
			return &TaskRoute{Complexity: complexity, Intent: intent, Route: "decompose", SubTasks: subTasks}
		}
		return &TaskRoute{Complexity: complexity, Intent: intent, Route: "agent"}
	}

	return &TaskRoute{Complexity: ComplexitySimple, Intent: intent, Route: "direct"}
}

func evaluateComplexity(messages []provider.Message, intent *agent.IntentResult) TaskComplexity {
	lastMsg := getLastUserMessage(messages)
	score := 0

	if len(lastMsg) > 500 {
		score += 2
	} else if len(lastMsg) > 200 {
		score += 1
	}

	switch intent.Intent {
	case agent.IntentChat, agent.IntentCodeExplain:
		score += 0
	case agent.IntentCodeEdit, agent.IntentDebug, agent.IntentTest:
		score += 1
	case agent.IntentRefactor, agent.IntentPlan:
		score += 2
	}

	complexKeywords := []string{"所有", "全部", "整个", "多个", "重构", "迁移",
		"all", "every", "multiple", "refactor", "migrate"}
	lower := strings.ToLower(lastMsg)
	for _, kw := range complexKeywords {
		if strings.Contains(lower, kw) {
			score += 2
			break
		}
	}

	fileRefs := countFileReferences(lastMsg)
	if fileRefs >= 3 {
		score += 2
	} else if fileRefs >= 1 {
		score += 1
	}

	switch {
	case score <= 1:
		return ComplexitySimple
	case score <= 3:
		return ComplexityModerate
	default:
		return ComplexityComplex
	}
}

func getLastUserMessage(messages []provider.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

var fileRefPattern = regexp.MustCompile("(`[^`]+\\.[a-zA-Z]{1,5}`|[\\w./\\\\-]+\\.[a-zA-Z]{1,5})")

func extractFileReferences(message string) []string {
	matches := fileRefPattern.FindAllString(message, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		m = strings.Trim(m, "`")
		if len(m) < 3 {
			continue
		}
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	return result
}

func countFileReferences(message string) int {
	return len(extractFileReferences(message))
}

func splitByActions(message string) []string {
	connectors := []string{"然后", "接着", "之后", "并且", "并", "然后", "then", "and then", "after that", "also"}
	segments := []string{message}

	for _, conn := range connectors {
		var newSegments []string
		for _, seg := range segments {
			parts := strings.Split(seg, conn)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if len(p) >= 5 {
					newSegments = append(newSegments, p)
				}
			}
		}
		if len(newSegments) > len(segments) {
			segments = newSegments
		}
	}

	return segments
}

func decomposeTask(message string, intent *agent.IntentResult) []SubTaskSpec {
	fileRefs := extractFileReferences(message)
	if len(fileRefs) >= 2 {
		var subTasks []SubTaskSpec
		for _, f := range fileRefs {
			subTasks = append(subTasks, SubTaskSpec{
				Description: "处理文件 " + f + " 的相关修改",
				Files:       []string{f},
			})
		}
		return subTasks
	}

	actionSegments := splitByActions(message)
	if len(actionSegments) >= 2 {
		var subTasks []SubTaskSpec
		for i, seg := range actionSegments {
			var deps []int
			if i > 0 {
				deps = []int{i - 1}
			}
			subTasks = append(subTasks, SubTaskSpec{
				Description: seg,
				DependsOn:   deps,
			})
		}
		return subTasks
	}

	return nil
}
