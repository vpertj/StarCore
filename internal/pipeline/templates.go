package pipeline

var BuiltinPipelines = []Pipeline{
	{
		ID:          "code-review-pipeline",
		Name:        "代码审查",
		Description: "自动审查代码变更，检查安全漏洞、代码风格和最佳实践",
		Stages: []Stage{
			{ID: "analyze", Name: "分析变更", AgentID: "universal-assistant", Description: "分析代码变更范围和影响", Mode: "plan", MaxLoops: 10},
			{ID: "security", Name: "安全审查", AgentID: "universal-assistant", Description: "检查安全漏洞（OWASP Top 10、CWE）、敏感信息泄露、注入风险", Mode: "build", DependsOn: []string{"analyze"}, MaxLoops: 20},
			{ID: "quality", Name: "质量审查", AgentID: "universal-assistant", Description: "检查代码风格、复杂度、测试覆盖率、最佳实践", Mode: "build", DependsOn: []string{"analyze"}, Parallel: true, MaxLoops: 20},
			{ID: "summary", Name: "汇总报告", AgentID: "universal-assistant", Description: "汇总审查结果，生成改进建议和优先级排序", Mode: "chat", DependsOn: []string{"security", "quality"}, MaxLoops: 5},
		},
	},
	{
		ID:          "full-feature",
		Name:        "完整功能开发",
		Description: "从需求分析到测试验证的完整功能开发流程",
		Stages: []Stage{
			{ID: "plan", Name: "需求分析", AgentID: "universal-assistant", Description: "分析需求，制定实施计划，识别影响范围", Mode: "plan", MaxLoops: 15, RequiresGate: true},
			{ID: "implement", Name: "代码实现", AgentID: "universal-assistant", Description: "按计划逐步实现代码变更", Mode: "build", DependsOn: []string{"plan"}, MaxLoops: 50},
			{ID: "test", Name: "测试编写", AgentID: "universal-assistant", Description: "为新功能编写单元测试和集成测试", Mode: "build", DependsOn: []string{"implement"}, MaxLoops: 30},
			{ID: "verify", Name: "验证闭环", AgentID: "universal-assistant", Description: "运行测试和代码检查，确保所有变更通过验证", Mode: "build", DependsOn: []string{"test"}, MaxLoops: 20},
			{ID: "review", Name: "代码审查", AgentID: "universal-assistant", Description: "审查代码质量、安全性和最佳实践", Mode: "chat", DependsOn: []string{"verify"}, MaxLoops: 10, RequiresGate: true},
		},
	},
	{
		ID:          "bug-fix",
		Name:        "Bug修复",
		Description: "定位、修复和验证Bug的完整流程",
		Stages: []Stage{
			{ID: "reproduce", Name: "复现问题", AgentID: "universal-assistant", Description: "分析Bug描述，定位相关代码，尝试复现问题", Mode: "plan", MaxLoops: 15},
			{ID: "fix", Name: "修复实现", AgentID: "universal-assistant", Description: "实现修复方案", Mode: "build", DependsOn: []string{"reproduce"}, MaxLoops: 30, RequiresGate: true},
			{ID: "verify", Name: "验证修复", AgentID: "universal-assistant", Description: "运行测试验证修复，确保无回归", Mode: "build", DependsOn: []string{"fix"}, MaxLoops: 20},
		},
	},
	{
		ID:          "refactor",
		Name:        "代码重构",
		Description: "安全重构代码，保证行为不变",
		Stages: []Stage{
			{ID: "analyze", Name: "重构分析", AgentID: "universal-assistant", Description: "分析重构目标，识别依赖关系和影响范围", Mode: "plan", MaxLoops: 15, RequiresGate: true},
			{ID: "refactor", Name: "执行重构", AgentID: "universal-assistant", Description: "逐步执行重构，保持小步提交", Mode: "build", DependsOn: []string{"analyze"}, MaxLoops: 40},
			{ID: "verify", Name: "行为验证", AgentID: "universal-assistant", Description: "运行全量测试，确保重构后行为不变", Mode: "build", DependsOn: []string{"refactor"}, MaxLoops: 20},
		},
	},
	{
		ID:          "test-gen",
		Name:        "测试生成",
		Description: "为现有代码生成全面的测试用例",
		Stages: []Stage{
			{ID: "analyze", Name: "代码分析", AgentID: "universal-assistant", Description: "分析代码结构和接口，识别测试场景", Mode: "plan", MaxLoops: 15},
			{ID: "generate", Name: "生成测试", AgentID: "universal-assistant", Description: "生成单元测试，覆盖正常路径、边界条件和错误路径", Mode: "build", DependsOn: []string{"analyze"}, MaxLoops: 30},
			{ID: "verify", Name: "验证测试", AgentID: "universal-assistant", Description: "运行生成的测试，确保全部通过", Mode: "build", DependsOn: []string{"generate"}, MaxLoops: 15},
		},
	},
	{
		ID:          "doc-gen",
		Name:        "文档生成",
		Description: "为代码生成API文档和使用说明",
		Stages: []Stage{
			{ID: "analyze", Name: "代码分析", AgentID: "universal-assistant", Description: "分析代码接口、类型定义和关键逻辑", Mode: "plan", MaxLoops: 10},
			{ID: "generate", Name: "生成文档", AgentID: "universal-assistant", Description: "生成API文档、README和使用示例", Mode: "build", DependsOn: []string{"analyze"}, MaxLoops: 20},
		},
	},
	{
		ID:          "migration",
		Name:        "技术迁移",
		Description: "框架升级或技术栈迁移流程",
		Stages: []Stage{
			{ID: "assess", Name: "影响评估", AgentID: "universal-assistant", Description: "评估迁移范围、依赖影响和兼容性风险", Mode: "plan", MaxLoops: 15, RequiresGate: true},
			{ID: "migrate", Name: "执行迁移", AgentID: "universal-assistant", Description: "逐步执行迁移，优先处理核心模块", Mode: "build", DependsOn: []string{"assess"}, MaxLoops: 50},
			{ID: "verify", Name: "迁移验证", AgentID: "universal-assistant", Description: "运行测试和构建，确保迁移后功能正常", Mode: "build", DependsOn: []string{"migrate"}, MaxLoops: 20},
		},
	},
}

func GetBuiltinPipeline(id string) *Pipeline {
	for i := range BuiltinPipelines {
		if BuiltinPipelines[i].ID == id {
			return &BuiltinPipelines[i]
		}
	}
	return nil
}
