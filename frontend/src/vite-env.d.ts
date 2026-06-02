/// <reference types="svelte" />
/// <reference types="vite/client" />

interface FileInfo {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  mode: number;
}

interface SearchResult {
  filePath: string;
  line: number;
  content: string;
}

interface SearchOptions {
  caseSensitive: boolean;
  wholeWord: boolean;
  useRegex: boolean;
  includePattern: string;
  excludePattern: string;
}

interface Backend {
  OpenFolder(): Promise<string>;
  ListDir(path: string): Promise<FileInfo[]>;
  ReadFile(path: string): Promise<string>;
  WriteFile(path: string, content: string): Promise<void>;
  CreateFile(path: string): Promise<void>;
  DeleteFile(path: string): Promise<void>;
  RenameFile(oldPath: string, newPath: string): Promise<void>;
  CreateDir(path: string): Promise<void>;
  ExecuteCommand(command: string): Promise<string>;
  SearchFiles(query: string, options: SearchOptions): Promise<SearchResult[]>;
  ReplaceInFiles(query: string, replacement: string, options: SearchOptions): Promise<void>;
  MinimizeWindow(): Promise<void>;
  MaximizeWindow(): Promise<void>;
  CloseWindow(): Promise<void>;
  Greet(name: string): Promise<string>;
}

declare global {
  interface Window {
    backend: Backend;
  }
}

export {};