package agent

import (
	"strings"
)

type IntentType string

const (
	IntentCodeEdit    IntentType = "code_edit"
	IntentCodeExplain IntentType = "code_explain"
	IntentDebug       IntentType = "debug"
	IntentRefactor    IntentType = "refactor"
	IntentSearch      IntentType = "search"
	IntentGit         IntentType = "git"
	IntentChat        IntentType = "chat"
	IntentPlan        IntentType = "plan"
	IntentTest        IntentType = "test"
	IntentDoc         IntentType = "doc"
)

const (
	HighConfidence   = 0.6
	MediumConfidence = 0.4
	LowConfidence    = 0.0
)

type IntentResult struct {
	Intent     IntentType
	Confidence float64
	Keywords   []string
	Language   string
}

type intentRule struct {
	intent   IntentType
	keywords []string
	weight   float64
}

type IntentClassifier struct {
	rules []intentRule
}

func NewIntentClassifier() *IntentClassifier {
	return &IntentClassifier{
		rules: buildDefaultRules(),
	}
}

func buildDefaultRules() []intentRule {
	return []intentRule{
		{
			intent: IntentCodeEdit,
			keywords: []string{
				"修改", "添加", "删除", "编辑", "改", "写", "实现", "创建",
				"edit", "add", "delete", "write", "implement", "create", "change", "update", "insert",
			},
			weight: 1.0,
		},
		{
			intent: IntentDebug,
			keywords: []string{
				"修复", "报错", "错误", "异常", "bug", "问题", "不工作", "失败", "崩溃",
				"fix", "error", "bug", "crash", "broken", "not working", "fail", "issue",
			},
			weight: 1.2,
		},
		{
			intent: IntentRefactor,
			keywords: []string{
				"重构", "优化", "改进", "提取", "拆分", "合并", "简化",
				"refactor", "optimize", "improve", "extract", "split", "simplify", "clean",
			},
			weight: 1.0,
		},
		{
			intent: IntentSearch,
			keywords: []string{
				"搜索", "查找", "找", "定位", "哪里", "哪个文件",
				"search", "find", "locate", "where", "which file",
			},
			weight: 0.8,
		},
		{
			intent: IntentGit,
			keywords: []string{
				"提交", "推送", "拉取", "分支", "合并", "commit", "push", "pull", "branch", "merge", "git",
			},
			weight: 1.0,
		},
		{
			intent: IntentCodeExplain,
			keywords: []string{
				"解释", "说明", "分析", "理解", "什么意思", "怎么工作",
				"explain", "describe", "analyze", "understand", "what does", "how does",
			},
			weight: 0.9,
		},
		{
			intent: IntentTest,
			keywords: []string{
				"测试", "单测", "单元测试", "test", "unit test", "coverage",
			},
			weight: 1.0,
		},
		{
			intent: IntentPlan,
			keywords: []string{
				"规划", "设计", "方案", "计划", "架构", "plan", "design", "architecture", "strategy",
			},
			weight: 0.9,
		},
		{
			intent: IntentDoc,
			keywords: []string{
				"文档", "注释", "document", "comment", "documentation",
			},
			weight: 0.8,
		},
		{
			intent: IntentChat,
			keywords: []string{
				"你好", "谢谢", "hello", "hi", "thanks", "help",
			},
			weight: 0.5,
		},
	}
}

func (c *IntentClassifier) Classify(message string) *IntentResult {
	msg := strings.ToLower(message)
	scores := make(map[IntentType]float64)
	matchedKeywords := make(map[IntentType][]string)

	for _, rule := range c.rules {
		score := 0.0
		for _, kw := range rule.keywords {
			if strings.Contains(msg, strings.ToLower(kw)) {
				score += rule.weight
				matchedKeywords[rule.intent] = append(matchedKeywords[rule.intent], kw)
			}
		}
		if score > 0 {
			scores[rule.intent] = score
		}
	}

	var bestIntent IntentType = IntentChat
	bestScore := 0.0
	for intent, score := range scores {
		if score > bestScore {
			bestScore = score
			bestIntent = intent
		}
	}

	confidence := 0.0
	if bestScore > 0 {
		totalScore := 0.0
		for _, score := range scores {
			totalScore += score
		}
		confidence = bestScore / totalScore
	}

	return &IntentResult{
		Intent:     bestIntent,
		Confidence: confidence,
		Keywords:   matchedKeywords[bestIntent],
		Language:   detectLanguage(message),
	}
}

func detectLanguage(message string) string {
	chineseCount := 0
	englishCount := 0
	for _, r := range message {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			englishCount++
		}
	}
	if chineseCount > englishCount {
		return "zh"
	}
	return "en"
}
