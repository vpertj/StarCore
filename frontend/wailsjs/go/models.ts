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
	export class FileMeta {
	    operation: string;
	    filePath: string;
	    startLine?: number;
	    endLine?: number;
	    summary?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operation = source["operation"];
	        this.filePath = source["filePath"];
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
	        this.summary = source["summary"];
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
	    fileMeta?: FileMeta;
	
	    static createFrom(source: any = {}) {
	        return new ToolResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.name = source["name"];
	        this.result = source["result"];
	        this.error = source["error"];
	        this.fileMeta = this.convertValues(source["fileMeta"], FileMeta);
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

export namespace codescan {
	
	export class Finding {
	    ruleId: string;
	    ruleName: string;
	    file: string;
	    line: number;
	    column: number;
	    severity: string;
	    message: string;
	    owasp?: string;
	    cwe?: string;
	    category: string;
	    suggestion?: string;
	
	    static createFrom(source: any = {}) {
	        return new Finding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ruleId = source["ruleId"];
	        this.ruleName = source["ruleName"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.column = source["column"];
	        this.severity = source["severity"];
	        this.message = source["message"];
	        this.owasp = source["owasp"];
	        this.cwe = source["cwe"];
	        this.category = source["category"];
	        this.suggestion = source["suggestion"];
	    }
	}
	export class Rule {
	    id: string;
	    name: string;
	    category: string;
	    owasp?: string;
	    cwe?: string;
	    severity: string;
	    description: string;
	    languages: string[];
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.owasp = source["owasp"];
	        this.cwe = source["cwe"];
	        this.severity = source["severity"];
	        this.description = source["description"];
	        this.languages = source["languages"];
	    }
	}
	export class ScanResult {
	    findings: Finding[];
	    total: number;
	    critical: number;
	    high: number;
	    medium: number;
	    low: number;
	    info: number;
	    healthScore: number;
	    file: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.findings = this.convertValues(source["findings"], Finding);
	        this.total = source["total"];
	        this.critical = source["critical"];
	        this.high = source["high"];
	        this.medium = source["medium"];
	        this.low = source["low"];
	        this.info = source["info"];
	        this.healthScore = source["healthScore"];
	        this.file = source["file"];
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

export namespace completion {
	
	export class Suggestion {
	    text: string;
	    type: string;
	    rank: number;
	
	    static createFrom(source: any = {}) {
	        return new Suggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.type = source["type"];
	        this.rank = source["rank"];
	    }
	}

}

export namespace debug {
	
	export class Breakpoint {
	    id: number;
	    file: string;
	    line: number;
	    function?: string;
	    enabled: boolean;
	    hitCount: number;
	    condition?: string;
	
	    static createFrom(source: any = {}) {
	        return new Breakpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.function = source["function"];
	        this.enabled = source["enabled"];
	        this.hitCount = source["hitCount"];
	        this.condition = source["condition"];
	    }
	}
	export class ConsoleResult {
	    output: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConsoleResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output = source["output"];
	        this.error = source["error"];
	    }
	}
	export class Goroutine {
	    id: number;
	    stack: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new Goroutine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.stack = source["stack"];
	        this.state = source["state"];
	    }
	}
	export class Variable {
	    name: string;
	    value: string;
	    type: string;
	    children?: Variable[];
	
	    static createFrom(source: any = {}) {
	        return new Variable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	        this.type = source["type"];
	        this.children = this.convertValues(source["children"], Variable);
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
	export class StackFrame {
	    id: number;
	    function: string;
	    file: string;
	    line: number;
	    package: string;
	
	    static createFrom(source: any = {}) {
	        return new StackFrame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.function = source["function"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.package = source["package"];
	    }
	}
	export class SessionState {
	    status: string;
	    reason: string;
	    file: string;
	    line: number;
	    expr?: string;
	    goroutines: Goroutine[];
	    stack: StackFrame[];
	    variables: Variable[];
	
	    static createFrom(source: any = {}) {
	        return new SessionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.expr = source["expr"];
	        this.goroutines = this.convertValues(source["goroutines"], Goroutine);
	        this.stack = this.convertValues(source["stack"], StackFrame);
	        this.variables = this.convertValues(source["variables"], Variable);
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

export namespace extension {
	
	export class CommandContribution {
	    id: string;
	    label: string;
	    shortcut?: string;
	    category?: string;
	
	    static createFrom(source: any = {}) {
	        return new CommandContribution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.shortcut = source["shortcut"];
	        this.category = source["category"];
	    }
	}
	export class Extension {
	    id: string;
	    name: string;
	    version: string;
	    description: string;
	    author: string;
	    entryPoint: string;
	    enabled: boolean;
	    commands?: CommandContribution[];
	    menus?: Record<string, Array<string>>;
	    config?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Extension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.author = source["author"];
	        this.entryPoint = source["entryPoint"];
	        this.enabled = source["enabled"];
	        this.commands = this.convertValues(source["commands"], CommandContribution);
	        this.menus = source["menus"];
	        this.config = source["config"];
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
	
	export class BlameLine {
	    hash: string;
	    author: string;
	    date: string;
	    line: number;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new BlameLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.author = source["author"];
	        this.date = source["date"];
	        this.line = source["line"];
	        this.content = source["content"];
	    }
	}
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
	
	export class Command {
	    title: string;
	    command: string;
	    arguments?: any[];
	
	    static createFrom(source: any = {}) {
	        return new Command(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.command = source["command"];
	        this.arguments = source["arguments"];
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
	export class CodeLens {
	    range: Range;
	    command?: Command;
	    data?: any;
	
	    static createFrom(source: any = {}) {
	        return new CodeLens(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.range = this.convertValues(source["range"], Range);
	        this.command = this.convertValues(source["command"], Command);
	        this.data = source["data"];
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
	
	export class DocumentSymbol {
	    name: string;
	    kind: number;
	    range: Range;
	    selectionRange: Range;
	    children?: DocumentSymbol[];
	
	    static createFrom(source: any = {}) {
	        return new DocumentSymbol(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.range = this.convertValues(source["range"], Range);
	        this.selectionRange = this.convertValues(source["selectionRange"], Range);
	        this.children = this.convertValues(source["children"], DocumentSymbol);
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
	export class FoldingRange {
	    startLine: number;
	    endLine: number;
	    kind?: string;
	
	    static createFrom(source: any = {}) {
	        return new FoldingRange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
	        this.kind = source["kind"];
	    }
	}
	export class FrontendCodeAction {
	    title: string;
	    kind?: string;
	    edit?: Record<string, Array<FrontendTextEdit>>;
	
	    static createFrom(source: any = {}) {
	        return new FrontendCodeAction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.kind = source["kind"];
	        this.edit = this.convertValues(source["edit"], Array<FrontendTextEdit>, true);
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
	export class FrontendLocation {
	    filePath: string;
	    line: number;
	    col: number;
	    endLine: number;
	    endCol: number;
	
	    static createFrom(source: any = {}) {
	        return new FrontendLocation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.line = source["line"];
	        this.col = source["col"];
	        this.endLine = source["endLine"];
	        this.endCol = source["endCol"];
	    }
	}
	export class FrontendServerInfo {
	    languageId: string;
	    command: string;
	    args: string[];
	    extensions: string[];
	    custom: boolean;
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FrontendServerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.languageId = source["languageId"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.extensions = source["extensions"];
	        this.custom = source["custom"];
	        this.running = source["running"];
	    }
	}
	export class FrontendTextEdit {
	    newText: string;
	    startLine: number;
	    startCol: number;
	    endLine: number;
	    endCol: number;
	
	    static createFrom(source: any = {}) {
	        return new FrontendTextEdit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.newText = source["newText"];
	        this.startLine = source["startLine"];
	        this.startCol = source["startCol"];
	        this.endLine = source["endLine"];
	        this.endCol = source["endCol"];
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
	export class InlayHint {
	    position: Position;
	    label: any;
	    kind?: number;
	    tooltip?: any;
	    paddingLeft?: boolean;
	    paddingRight?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InlayHint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.position = this.convertValues(source["position"], Position);
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.tooltip = source["tooltip"];
	        this.paddingLeft = source["paddingLeft"];
	        this.paddingRight = source["paddingRight"];
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
	export class LanguagePackage {
	    id: string;
	    name: string;
	    languageId: string;
	    command: string;
	    args: string[];
	    extensions: string[];
	    installCmd: string;
	    downloadUrl: string;
	    downloadFile: string;
	    description: string;
	    category: string;
	    hasHighlight: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LanguagePackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.languageId = source["languageId"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.extensions = source["extensions"];
	        this.installCmd = source["installCmd"];
	        this.downloadUrl = source["downloadUrl"];
	        this.downloadFile = source["downloadFile"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.hasHighlight = source["hasHighlight"];
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
	
	export class ParameterInfo {
	    label: string;
	    documentation?: string;
	
	    static createFrom(source: any = {}) {
	        return new ParameterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.documentation = source["documentation"];
	    }
	}
	
	
	export class RenameResult {
	    changes: Record<string, Array<TextEdit>>;
	
	    static createFrom(source: any = {}) {
	        return new RenameResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changes = this.convertValues(source["changes"], Array<TextEdit>, true);
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
	export class SignatureInfo {
	    label: string;
	    documentation?: string;
	    parameters?: ParameterInfo[];
	
	    static createFrom(source: any = {}) {
	        return new SignatureInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.documentation = source["documentation"];
	        this.parameters = this.convertValues(source["parameters"], ParameterInfo);
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
	export class SignatureHelp {
	    signatures: SignatureInfo[];
	    activeSignature: number;
	    activeParameter: number;
	
	    static createFrom(source: any = {}) {
	        return new SignatureHelp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.signatures = this.convertValues(source["signatures"], SignatureInfo);
	        this.activeSignature = source["activeSignature"];
	        this.activeParameter = source["activeParameter"];
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
	
	export class TextEdit {
	    range: Range;
	    newText: string;
	
	    static createFrom(source: any = {}) {
	        return new TextEdit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.range = this.convertValues(source["range"], Range);
	        this.newText = source["newText"];
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
	export class WorkspaceSymbol {
	    name: string;
	    kind: number;
	    containerName?: string;
	    location: Location;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceSymbol(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.containerName = source["containerName"];
	        this.location = this.convertValues(source["location"], Location);
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
	export class CodeCompleteRequest {
	    beforeCursor: string;
	    afterCursor: string;
	    fileName: string;
	    language: string;
	    maxTokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new CodeCompleteRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.beforeCursor = source["beforeCursor"];
	        this.afterCursor = source["afterCursor"];
	        this.fileName = source["fileName"];
	        this.language = source["language"];
	        this.maxTokens = source["maxTokens"];
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
	export class PipelineInfo {
	    id: string;
	    name: string;
	    description: string;
	    stageCount: number;
	
	    static createFrom(source: any = {}) {
	        return new PipelineInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.stageCount = source["stageCount"];
	    }
	}
	export class RunPipelineRequest {
	    pipelineId: string;
	    userInput: string;
	    projectPath: string;
	
	    static createFrom(source: any = {}) {
	        return new RunPipelineRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pipelineId = source["pipelineId"];
	        this.userInput = source["userInput"];
	        this.projectPath = source["projectPath"];
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
	    metadata?: string;
	
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
	        this.metadata = source["metadata"];
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
	export class SessionState {
	    activeConvId: string;
	    projectPath: string;
	    agentId: string;
	    mode: string;
	    providerId: string;
	    model: string;
	    lastMessageAt: string;
	    unsavedContent?: string;
	    crashed: boolean;
	    // Go type: time
	    savedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SessionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.activeConvId = source["activeConvId"];
	        this.projectPath = source["projectPath"];
	        this.agentId = source["agentId"];
	        this.mode = source["mode"];
	        this.providerId = source["providerId"];
	        this.model = source["model"];
	        this.lastMessageAt = source["lastMessageAt"];
	        this.unsavedContent = source["unsavedContent"];
	        this.crashed = source["crashed"];
	        this.savedAt = this.convertValues(source["savedAt"], null);
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

export namespace pipeline {
	
	export class Artifact {
	    type: string;
	    name: string;
	    content: string;
	    path?: string;
	
	    static createFrom(source: any = {}) {
	        return new Artifact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.path = source["path"];
	    }
	}
	export class StageResult {
	    stageId: string;
	    agentId: string;
	    status: string;
	    output: string;
	    artifacts?: Artifact[];
	    error?: string;
	    startedAt?: string;
	    endedAt?: string;
	    tokensIn: number;
	    tokensOut: number;
	
	    static createFrom(source: any = {}) {
	        return new StageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stageId = source["stageId"];
	        this.agentId = source["agentId"];
	        this.status = source["status"];
	        this.output = source["output"];
	        this.artifacts = this.convertValues(source["artifacts"], Artifact);
	        this.error = source["error"];
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	        this.tokensIn = source["tokensIn"];
	        this.tokensOut = source["tokensOut"];
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
	export class PipelineRun {
	    pipelineId: string;
	    status: string;
	    stageResults: Record<string, StageResult>;
	    startedAt?: string;
	    endedAt?: string;
	    error?: string;
	    snapshot?: number[];
	
	    static createFrom(source: any = {}) {
	        return new PipelineRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pipelineId = source["pipelineId"];
	        this.status = source["status"];
	        this.stageResults = this.convertValues(source["stageResults"], StageResult, true);
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	        this.error = source["error"];
	        this.snapshot = source["snapshot"];
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
	
	export class Attachment {
	    type: string;
	    name: string;
	    mimeType: string;
	    data: string;
	    url?: string;
	
	    static createFrom(source: any = {}) {
	        return new Attachment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.mimeType = source["mimeType"];
	        this.data = source["data"];
	        this.url = source["url"];
	    }
	}
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
	    conversationId?: string;
	    attachments?: Attachment[];
	
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
	        this.conversationId = source["conversationId"];
	        this.attachments = this.convertValues(source["attachments"], Attachment);
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
	export class ImageContent {
	    type: string;
	    url?: string;
	    mediaType?: string;
	    data?: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.url = source["url"];
	        this.mediaType = source["mediaType"];
	        this.data = source["data"];
	    }
	}
	
	export class Model {
	    id: string;
	    name: string;
	    providerId: string;
	    maxTokens: number;
	    contextWindow: number;
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
	        this.contextWindow = source["contextWindow"];
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

export namespace rag {
	
	export class Document {
	    id: string;
	    content: string;
	    metadata: Record<string, string>;
	    embedding?: number[];
	
	    static createFrom(source: any = {}) {
	        return new Document(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.content = source["content"];
	        this.metadata = source["metadata"];
	        this.embedding = source["embedding"];
	    }
	}
	export class SearchResult {
	    document?: Document;
	    score: number;
	    chunkText: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.document = this.convertValues(source["document"], Document);
	        this.score = source["score"];
	        this.chunkText = source["chunkText"];
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

export namespace remote {
	
	export class Connection {
	    id: string;
	    name: string;
	    type: string;
	    host: string;
	    port: number;
	    user: string;
	    container?: string;
	    workDir: string;
	    status: string;
	    lastError?: string;
	    // Go type: time
	    connectedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.container = source["container"];
	        this.workDir = source["workDir"];
	        this.status = source["status"];
	        this.lastError = source["lastError"];
	        this.connectedAt = this.convertValues(source["connectedAt"], null);
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

export namespace tools {
	
	export class AskUserResponse {
	    id: string;
	    answer: string;
	
	    static createFrom(source: any = {}) {
	        return new AskUserResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.answer = source["answer"];
	    }
	}

}

export namespace verify {
	
	export class Diagnostic {
	    file: string;
	    line: number;
	    column: number;
	    severity: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.line = source["line"];
	        this.column = source["column"];
	        this.severity = source["severity"];
	        this.message = source["message"];
	    }
	}
	export class CheckResult {
	    type: string;
	    passed: boolean;
	    output: string;
	    errors?: Diagnostic[];
	    duration: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.passed = source["passed"];
	        this.output = source["output"];
	        this.errors = this.convertValues(source["errors"], Diagnostic);
	        this.duration = source["duration"];
	        this.command = source["command"];
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
	
	export class TestCaseResult {
	    name: string;
	    status: string;
	    duration: string;
	    output?: string;
	    file?: string;
	    line?: number;
	
	    static createFrom(source: any = {}) {
	        return new TestCaseResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.duration = source["duration"];
	        this.output = source["output"];
	        this.file = source["file"];
	        this.line = source["line"];
	    }
	}
	export class TestSuiteResult {
	    name: string;
	    total: number;
	    passed: number;
	    failed: number;
	    skipped: number;
	    duration: string;
	    testCases: TestCaseResult[];
	
	    static createFrom(source: any = {}) {
	        return new TestSuiteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.total = source["total"];
	        this.passed = source["passed"];
	        this.failed = source["failed"];
	        this.skipped = source["skipped"];
	        this.duration = source["duration"];
	        this.testCases = this.convertValues(source["testCases"], TestCaseResult);
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
	export class VerificationResult {
	    allPassed: boolean;
	    checks: CheckResult[];
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new VerificationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allPassed = source["allPassed"];
	        this.checks = this.convertValues(source["checks"], CheckResult);
	        this.summary = source["summary"];
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

export namespace workspace {
	
	export class WorkspaceRoot {
	    path: string;
	    name: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceRoot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.active = source["active"];
	    }
	}

}

