/*
  File: types/index.ts
  Purpose: TypeScript type definitions for CodeTextor frontend.
  Author: CodeTextor project
  Notes: Defines interfaces for backend data structures and API responses.
         Imports Project types from backend bindings for type safety.
*/

// Re-export backend generated types
export type { Project, ProjectConfig, ProjectStats } from '../api/backend'

// Represents a semantic chunk of code
export interface Chunk {
  id: string;
  projectId: string; // Added project namespace
  filePath: string;
  kind: string; // function_declaration, class_declaration, etc.
  name: string;
  content: string;
  startLine: number;
  endLine: number;
  startByte: number;
  endByte: number;
  similarity?: number; // For search results
}

// Represents a symbol in the codebase
export interface Symbol {
  id: string;
  projectId: string; // Added project namespace
  name: string;
  kind: string;
  filePath: string;
  line: number;
  column: number;
}

// Outline node for file structure
export interface OutlineNode {
  id: string;
  name: string;
  kind: string;
  startLine: number;
  endLine: number;
  children?: OutlineNode[];
}

// Indexing progress information
export interface IndexingProgress {
  totalFiles: number;
  processedFiles: number;
  currentFile: string;
  status: 'idle' | 'indexing' | 'completed' | 'error';
  error?: string;
}

// Search filters
export interface SearchFilters {
  filePatterns?: string[];
  symbolKinds?: string[];
  minSimilarity?: number;
}

// Search request
export interface SearchRequest {
  projectId: string; // Added project namespace
  query: string;
  k: number;
  filters?: SearchFilters;
}

// Search response
export interface SearchResponse {
  chunks: Chunk[];
  totalResults: number;
  queryTime: number; // milliseconds
}

// Outline request
export interface OutlineRequest {
  projectId: string; // Added project namespace
  path: string;
  depth?: number;
}

// Node source request
export interface NodeSourceRequest {
  id: string;
  collapseBody?: boolean;
}

// Symbol search request
export interface SymbolSearchRequest {
  projectId: string; // Added project namespace
  query: string;
  kinds?: string[];
  limit?: number;
}

// Note: ProjectStats is now imported from backend types above
// It includes: totalFiles, totalChunks, totalSymbols, databaseSize, lastIndexedAt, isIndexing, indexingProgress

// MCP Server configuration
export interface MCPServerConfig {
  host: string;
  port: number;
  protocol: 'http' | 'stdio';
  autoStart: boolean;
  maxConnections: number;
}

// MCP Server status
export interface MCPServerStatus {
  isRunning: boolean;
  uptime: number; // seconds
  activeConnections: number;
  totalRequests: number;
  averageResponseTime: number; // milliseconds
  lastError?: string;
}

// MCP Tool definition
export interface MCPTool {
  name: string;
  description: string;
  enabled: boolean;
  callCount: number;
}

// Note: Project, ProjectConfig, and ProjectStats are imported from backend types
// Project includes: id, name, description, createdAt, updatedAt, config, stats
