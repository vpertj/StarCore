interface Backend {
  ApplyDiff(arg1: any): Promise<void>;
  ComputeDiff(arg1: string, arg2: string): Promise<any[]>;
  AIChat(arg1: any): Promise<any>;
  AIChatStream(arg1: any): Promise<void>;
  RespondToAsk(arg1: any): Promise<boolean>;
  AICompletion(arg1: any, arg2: any): Promise<any>;
  CloseWindow(): Promise<void>;
  CreateDir(arg1: string): Promise<void>;
  CreateFile(arg1: string): Promise<void>;
  DeleteFile(arg1: string): Promise<void>;
  DeleteConversation(arg1: string): Promise<void>;
  DeleteKnowledge(arg1: string): Promise<void>;
  ExecuteCommand(arg1: string): Promise<string>;
  StartTerminal(arg1: string): Promise<void>;
  TerminalWrite(arg1: string): Promise<void>;
  TerminalResize(arg1: number, arg2: number): Promise<void>;
  KillTerminal(): Promise<void>;
  ExecuteSkill(arg1: string, arg2: any, arg3: string, arg4: string): Promise<void>;
  GetAgentConfig(arg1: string): Promise<any>;
  GetAgents(): Promise<any[]>;
  GetConversations(arg1: string): Promise<any[]>;
  GetKnowledge(arg1: string): Promise<any[]>;
  GetMessages(arg1: string): Promise<any[]>;
  GetModels(arg1: string): Promise<any[]>;
  GetProviders(): Promise<any[]>;
  GetTokenUsage(arg1: string): Promise<any>;
  SaveTokenUsageEntry(arg1: any): Promise<void>;
  GetSkills(): Promise<any[]>;
  GetTools(): Promise<any[]>;
  ExecuteToolCall(arg1: any): Promise<any>;
  SetToolAutoApprove(arg1: string, arg2: boolean): Promise<void>;
  GetMCPServers(): Promise<any[]>;
  AddMCPServer(arg1: any): Promise<void>;
  RemoveMCPServer(arg1: string): Promise<void>;
  StartMCPServer(arg1: string): Promise<void>;
  StopMCPServer(arg1: string): Promise<void>;
  Greet(arg1: string): Promise<string>;
  ListDir(arg1: string): Promise<any[]>;
  MaximizeWindow(): Promise<void>;
  MinimizeWindow(): Promise<void>;
  OpenFolder(): Promise<string>;
  ReadFile(arg1: string): Promise<string>;
  RenameFile(arg1: string, arg2: string): Promise<void>;
  ReplaceInFiles(arg1: string, arg2: string, arg3: any): Promise<void>;
  SaveConversation(arg1: any): Promise<void>;
  SaveKnowledge(arg1: any): Promise<void>;
  SaveMessage(arg1: any): Promise<void>;
  SearchFiles(arg1: string, arg2: any): Promise<any[]>;
  SetProviderConfig(arg1: string, arg2: any): Promise<void>;
  TestProvider(arg1: string): Promise<void>;
  WriteFile(arg1: string, arg2: string): Promise<void>;
}

interface Window {
  backend: Backend;
  go: {
    main: {
      App: Backend;
    };
  };
}
