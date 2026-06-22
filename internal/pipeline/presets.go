package pipeline

func SDDPipeline() Pipeline {
	return Pipeline{
		ID:          "sdd",
		Name:        "SDD 规范驱动开发",
		Description: "Spec → Design → Task 三阶段规范驱动开发流水线",
		Stages: []Stage{
			{
				ID:          "spec",
				Name:        "需求规格",
				AgentID:     "product-manager",
				Description: "分析用户需求，生成EARS格式的需求规格文档(spec.md)。明确功能需求、非功能需求、约束条件和验收标准。输出必须是结构化的需求文档，使用Markdown格式。",
				Mode:        "build",
				DependsOn:   []string{},
				MaxLoops:    25,
			},
			{
				ID:          "design",
				Name:        "技术设计",
				AgentID:     "backend-architect",
				Description: "基于需求规格(spec.md)，设计技术方案(design.md)。包括：架构选型、模块划分、接口定义、数据模型、关键算法、错误处理策略。输出必须是结构化的设计文档。注意：只做设计，不写实现代码。",
				Mode:        "build",
				DependsOn:   []string{"spec"},
				MaxLoops:    30,
			},
			{
				ID:          "task",
				Name:        "任务分解",
				AgentID:     "universal-assistant",
				Description: "基于技术设计(design.md)，分解为可执行的开发任务(tasks.md)。每个任务包含：具体文件路径、代码变更描述、依赖关系、验证方法。任务必须足够具体，可以直接按步骤编码实现。输出必须是结构化的任务列表。",
				Mode:        "build",
				DependsOn:   []string{"design"},
				MaxLoops:    20,
			},
			{
				ID:          "implement",
				Name:        "编码实现",
				AgentID:     "universal-assistant",
				Description: "按照任务列表(tasks.md)逐步实现代码。严格遵循设计文档的架构和接口定义。每完成一个任务立即运行验证（编译/lint/测试），确保代码质量。所有任务完成后做最终验证。",
				Mode:        "build",
				DependsOn:   []string{"task"},
				MaxLoops:    60,
			},
		},
	}
}

func CodeReviewPipeline() Pipeline {
	return Pipeline{
		ID:          "code-review",
		Name:        "代码审查流水线",
		Description: "并行审查代码质量、安全性和性能",
		Stages: []Stage{
			{
				ID:          "quality",
				Name:        "代码质量审查",
				AgentID:     "compliance-checker",
				Description: "审查代码质量：命名规范、代码重复、复杂度、可维护性。输出具体的问题列表和改进建议。",
				Mode:        "plan",
				DependsOn:   []string{},
				Parallel:    true,
				MaxLoops:    15,
			},
			{
				ID:          "security",
				Name:        "安全审查",
				AgentID:     "compliance-checker",
				Description: "安全审计：注入漏洞、敏感数据泄露、权限问题、依赖安全。输出按严重程度排序的漏洞列表。",
				Mode:        "plan",
				DependsOn:   []string{},
				Parallel:    true,
				MaxLoops:    15,
			},
			{
				ID:          "performance",
				Name:        "性能审查",
				AgentID:     "performance-expert",
				Description: "性能分析：N+1查询、内存泄漏、不必要的计算、缓存机会。输出性能瓶颈和优化建议。",
				Mode:        "plan",
				DependsOn:   []string{},
				Parallel:    true,
				MaxLoops:    15,
			},
			{
				ID:          "summary",
				Name:        "审查汇总",
				AgentID:     "product-manager",
				Description: "汇总所有审查结果，按优先级排序，生成可执行的改进计划。标注关键问题和建议的修复顺序。",
				Mode:        "build",
				DependsOn:   []string{"quality", "security", "performance"},
				MaxLoops:    10,
			},
		},
	}
}

func FullStackPipeline() Pipeline {
	return Pipeline{
		ID:          "fullstack",
		Name:        "全栈开发流水线",
		Description: "前后端并行开发 + 集成验证",
		Stages: []Stage{
			{
				ID:          "api-design",
				Name:        "API设计",
				AgentID:     "backend-architect",
				Description: "设计API接口规范：端点、请求/响应格式、错误码、认证方式。输出OpenAPI风格的接口文档。",
				Mode:        "build",
				DependsOn:   []string{},
				MaxLoops:    20,
			},
			{
				ID:          "backend",
				Name:        "后端实现",
				AgentID:     "backend-architect",
				Description: "基于API设计实现后端代码：路由、控制器、服务层、数据模型、中间件。实现后运行编译和测试验证。",
				Mode:        "build",
				DependsOn:   []string{"api-design"},
				Parallel:    true,
				MaxLoops:    40,
			},
			{
				ID:          "frontend",
				Name:        "前端实现",
				AgentID:     "frontend-architect",
				Description: "基于API设计实现前端代码：页面组件、API调用、状态管理、路由。实现后运行构建验证。",
				Mode:        "build",
				DependsOn:   []string{"api-design"},
				Parallel:    true,
				MaxLoops:    40,
			},
			{
				ID:          "integration",
				Name:        "集成验证",
				AgentID:     "api-test-engineer",
				Description: "集成测试：验证前后端接口对接、端到端流程、错误处理。生成测试报告。",
				Mode:        "build",
				DependsOn:   []string{"backend", "frontend"},
				MaxLoops:    25,
			},
		},
	}
}

func AllPipelines() []Pipeline {
	builtins := []Pipeline{
		SDDPipeline(),
		CodeReviewPipeline(),
		FullStackPipeline(),
	}
	builtins = append(builtins, BuiltinPipelines...)
	return builtins
}
