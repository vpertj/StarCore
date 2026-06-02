export namespace agent {
	
	export class AgentDef {
	    id: string;
	    name: string;
	    icon: string;
	    description: string;
	    systemPrompt: string;
	    defaultModel: string;
	    tools: string[];
	    skills: string[];
	    category: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	        this.systemPrompt = source["systemPrompt"];
	        this.defaultModel = source["defaultModel"];
	        this.tools = source["tools"];
	        this.skills = source["skills"];
	        this.category = source["category"];
	    }
	}
	export class ToolCall {
	    id: string;
	    name: string;
	    args: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ToolCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.args = source["args"];
	    }
	}
	export class ToolParamProp {
	    type: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolParamProp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.description = source["description"];
	    }
	}
	export class ToolParameters {
	    type: string;
	    properties: Record<string, ToolParamProp>;
	    required: string[];
	
	    static createFrom(source: any = {}) {
	        return new ToolParameters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.properties = this.convertValues(source["properties"], ToolParamProp, true);
	        this.required = source["required"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolDef {
	    id: string;
	    name: string;
	    description: string;
	    parameters: ToolParameters;
	    requiresApproval: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ToolDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.parameters = this.convertValues(source["parameters"], ToolParameters);
	        this.requiresApproval = source["requiresApproval"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ToolResult {
	    callId: string;
	    name: string;
	    result: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.name = source["name"];
	        this.result = source["result"];
	        this.error = source["error"];
	    }
	}

}

export namespace files {
	
	export class DiffHunk {
	    oldStart: number;
	    oldCount: number;
	    newStart: number;
	    newCount: number;
	    oldLines: string[];
	    newLines: string[];
	
	    static createFrom(source: any = {}) {
	        return new DiffHunk(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldStart = source["oldStart"];
	        this.oldCount = source["oldCount"];
	        this.newStart = source["newStart"];
	        this.newCount = source["newCount"];
	        this.oldLines = source["oldLines"];
	        this.newLines = source["newLines"];
	    }
	}
	export class FileInfo {
	    name: string;
	    path: string;
	    isDir: boolean;
	    size: number;
	    mode: number;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.mode = source["mode"];
	    }
	}
	export class SearchOptions {
	    caseSensitive: boolean;
	    wholeWord: boolean;
	    useRegex: boolean;
	    includePattern: string;
	    excludePattern: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.caseSensitive = source["caseSensitive"];
	        this.wholeWord = source["wholeWord"];
	        this.useRegex = source["useRegex"];
	        this.includePattern = source["includePattern"];
	        this.excludePattern = source["excludePattern"];
	    }
	}
	export class SearchResult {
	    filePath: string;
	    line: number;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.line = source["line"];
	        this.content = source["content"];
	    }
	}

}

export namespace git {
	
	export class LogEntry {
	    hash: string;
	    message: string;
	    author: string;
	    date: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.message = source["message"];
	        this.author = source["author"];
	        this.date = source["date"];
	    }
	}
	export class StatusEntry {
	    path: string;
	    status: string;
	    staged: boolean;
	    added: boolean;
	    deleted: boolean;
	    renamed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StatusEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.status = source["status"];
	        this.staged = source["staged"];
	        this.added = source["added"];
	        this.deleted = source["deleted"];
	        this.renamed = source["renamed"];
	    }
	}

}

export namespace lsp {
	
	export class FrontendCompletion {
	    label: string;
	    insertText: string;
	    kind: number;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new FrontendCompletion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.insertText = source["insertText"];
	        this.kind = source["kind"];
	        this.detail = source["detail"];
	    }
	}
	export class Position {
	    line: number;
	    character: number;
	
	    static createFrom(source: any = {}) {
	        return new Position(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.character = source["character"];
	    }
	}
	export class Range {
	    start: Position;
	    end: Position;
	
	    static createFrom(source: any = {}) {
	        return new Range(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = this.convertValues(source["start"], Position);
	        this.end = this.convertValues(source["end"], Position);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MarkupContent {
	    kind: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new MarkupContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.value = source["value"];
	    }
	}
	export class Hover {
	    contents: MarkupContent;
	    range?: Range;
	
	    static createFrom(source: any = {}) {
	        return new Hover(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contents = this.convertValues(source["contents"], MarkupContent);
	        this.range = this.convertValues(source["range"], Range);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Location {
	    uri: string;
	    range: Range;
	
	    static createFrom(source: any = {}) {
	        return new Location(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	        this.range = this.convertValues(source["range"], Range);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

export namespace main {
	
	export class ApplyDiffRequest {
	    filePath: string;
	    hunks: files.DiffHunk[];
	
	    static createFrom(source: any = {}) {
	        return new ApplyDiffRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.hunks = this.convertValues(source["hunks"], files.DiffHunk);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CustomModelEntry {
	    id: string;
	    modelId: string;
	    name: string;
	    providerId: string;
	    providerName: string;
	    apiKey: string;
	    endpoint: string;
	    enabled: boolean;
	    maxTokens: number;
	
	    static createFrom(source: any = {}) {
	        return new CustomModelEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.modelId = source["modelId"];
	        this.name = source["name"];
	        this.providerId = source["providerId"];
	        this.providerName = source["providerName"];
	        this.apiKey = source["apiKey"];
	        this.endpoint = source["endpoint"];
	        this.enabled = source["enabled"];
	        this.maxTokens = source["maxTokens"];
	    }
	}

}

export namespace mcp {
	
	export class MCPServerConfig {
	    id: string;
	    name: string;
	    command: string;
	    args: string[];
	    endpoint: string;
	    transport: string;
	    env: Record<string, string>;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.endpoint = source["endpoint"];
	        this.transport = source["transport"];
	        this.env = source["env"];
	        this.enabled = source["enabled"];
	    }
	}

}

export namespace memory {
	
	export class Conversation {
	    id: string;
	    projectPath: string;
	    agentId: string;
	    model: string;
	    providerId: string;
	    title: string;
	    summary: string;
	    createdAt: string;
	    updatedAt: string;
	    messageCount: number;
	
	    static createFrom(source: any = {}) {
	        return new Conversation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectPath = source["projectPath"];
	        this.agentId = source["agentId"];
	        this.model = source["model"];
	        this.providerId = source["providerId"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.messageCount = source["messageCount"];
	    }
	}
	export class Knowledge {
	    id: string;
	    projectPath: string;
	    category: string;
	    key: string;
	    value: string;
	    source: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Knowledge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectPath = source["projectPath"];
	        this.category = source["category"];
	        this.key = source["key"];
	        this.value = source["value"];
	        this.source = source["source"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class Message {
	    id: string;
	    conversationId: string;
	    seq: number;
	    role: string;
	    content: string;
	    thinking?: string;
	    tokensIn: number;
	    tokensOut: number;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.seq = source["seq"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.thinking = source["thinking"];
	        this.tokensIn = source["tokensIn"];
	        this.tokensOut = source["tokensOut"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ProviderUsage {
	    tokensIn: number;
	    tokensOut: number;
	    cost: number;
	
	    static createFrom(source: any = {}) {
	        return new ProviderUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tokensIn = source["tokensIn"];
	        this.tokensOut = source["tokensOut"];
	        this.cost = source["cost"];
	    }
	}
	export class TokenUsageEntry {
	    id: string;
	    conversationId: string;
	    providerId: string;
	    model: string;
	    tokensIn: number;
	    tokensOut: number;
	    cost: number;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new TokenUsageEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.providerId = source["providerId"];
	        this.model = source["model"];
	        this.tokensIn = source["tokensIn"];
	        this.tokensOut = source["tokensOut"];
	        this.cost = source["cost"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class TokenUsageStats {
	    totalTokensIn: number;
	    totalTokensOut: number;
	    totalCost: number;
	    byProvider: Record<string, ProviderUsage>;
	
	    static createFrom(source: any = {}) {
	        return new TokenUsageStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalTokensIn = source["totalTokensIn"];
	        this.totalTokensOut = source["totalTokensOut"];
	        this.totalCost = source["totalCost"];
	        this.byProvider = this.convertValues(source["byProvider"], ProviderUsage, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace provider {
	
	export class ToolFunction {
	    name: string;
	    description: string;
	    parameters: any;
	
	    static createFrom(source: any = {}) {
	        return new ToolFunction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.parameters = source["parameters"];
	    }
	}
	export class ToolDefinition {
	    type: string;
	    function: ToolFunction;
	
	    static createFrom(source: any = {}) {
	        return new ToolDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.function = this.convertValues(source["function"], ToolFunction);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolCallFunc {
	    name: string;
	    arguments: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolCallFunc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.arguments = source["arguments"];
	    }
	}
	export class ToolCall {
	    id: string;
	    type: string;
	    function: ToolCallFunc;
	
	    static createFrom(source: any = {}) {
	        return new ToolCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.function = this.convertValues(source["function"], ToolCallFunc);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Message {
	    role: string;
	    content: string;
	    tool_calls?: ToolCall[];
	    tool_call_id?: string;
	    name?: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.tool_calls = this.convertValues(source["tool_calls"], ToolCall);
	        this.tool_call_id = source["tool_call_id"];
	        this.name = source["name"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChatRequest {
	    providerId: string;
	    model: string;
	    messages: Message[];
	    temperature: number;
	    maxTokens: number;
	    stream: boolean;
	    agentId?: string;
	    contextFiles?: string[];
	    contextCode?: string;
	    projectPath?: string;
	    activeFile?: string;
	    activeFileContent?: string;
	    selectedCode?: string;
	    tools?: ToolDefinition[];
	    mode?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providerId = source["providerId"];
	        this.model = source["model"];
	        this.messages = this.convertValues(source["messages"], Message);
	        this.temperature = source["temperature"];
	        this.maxTokens = source["maxTokens"];
	        this.stream = source["stream"];
	        this.agentId = source["agentId"];
	        this.contextFiles = source["contextFiles"];
	        this.contextCode = source["contextCode"];
	        this.projectPath = source["projectPath"];
	        this.activeFile = source["activeFile"];
	        this.activeFileContent = source["activeFileContent"];
	        this.selectedCode = source["selectedCode"];
	        this.tools = this.convertValues(source["tools"], ToolDefinition);
	        this.mode = source["mode"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CompletionRequest {
	    file: string;
	    content: string;
	    cursorPos: number;
	    language: string;
	    model?: string;
	    temperature?: number;
	
	    static createFrom(source: any = {}) {
	        return new CompletionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.content = source["content"];
	        this.cursorPos = source["cursorPos"];
	        this.language = source["language"];
	        this.model = source["model"];
	        this.temperature = source["temperature"];
	    }
	}
	
	export class Model {
	    id: string;
	    name: string;
	    providerId: string;
	    maxTokens: number;
	    supportsVision: boolean;
	    supportsTool: boolean;
	    supportsThinking: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Model(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.providerId = source["providerId"];
	        this.maxTokens = source["maxTokens"];
	        this.supportsVision = source["supportsVision"];
	        this.supportsTool = source["supportsTool"];
	        this.supportsThinking = source["supportsThinking"];
	    }
	}
	export class ProviderConfig {
	    id: string;
	    name: string;
	    apiKey: string;
	    endpoint: string;
	    isDefault: boolean;
	    enabled: boolean;
	    timeoutSecs?: number;
	
	    static createFrom(source: any = {}) {
	        return new ProviderConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.apiKey = source["apiKey"];
	        this.endpoint = source["endpoint"];
	        this.isDefault = source["isDefault"];
	        this.enabled = source["enabled"];
	        this.timeoutSecs = source["timeoutSecs"];
	    }
	}
	export class ProviderInfo {
	    id: string;
	    name: string;
	    endpoint: string;
	    enabled: boolean;
	    isDefault: boolean;
	    models: Model[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.endpoint = source["endpoint"];
	        this.enabled = source["enabled"];
	        this.isDefault = source["isDefault"];
	        this.models = this.convertValues(source["models"], Model);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

export namespace skill {
	
	export class SkillContext {
	    selectedCode: string;
	    filePath: string;
	    fileContent: string;
	    diagnostics: string[];
	    language: string;
	    projectPath: string;
	    contextFiles?: string[];
	    userInput?: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selectedCode = source["selectedCode"];
	        this.filePath = source["filePath"];
	        this.fileContent = source["fileContent"];
	        this.diagnostics = source["diagnostics"];
	        this.language = source["language"];
	        this.projectPath = source["projectPath"];
	        this.contextFiles = source["contextFiles"];
	        this.userInput = source["userInput"];
	    }
	}
	export class SkillDef {
	    id: string;
	    name: string;
	    icon: string;
	    description: string;
	    trigger: string;
	    promptTemplate: string;
	    resultType: string;
	    associatedAgents: string[];
	    category: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	        this.trigger = source["trigger"];
	        this.promptTemplate = source["promptTemplate"];
	        this.resultType = source["resultType"];
	        this.associatedAgents = source["associatedAgents"];
	        this.category = source["category"];
	    }
	}

}

