/*
  File: types/index.ts
  Purpose: Central TypeScript definitions shared across the frontend.
  Author: CodeTextor project
*/

import type { models } from '../api/backend'

// Re-export backend generated types, but relax strict class requirements for configs
export type ProjectConfig = Omit<models.ProjectConfig, 'convertValues'> & {
  convertValues?: (...args: any[]) => any
}
export type Project = Omit<models.Project, 'config' | 'convertValues'> & {
  config: ProjectConfig
  convertValues?: (...args: any[]) => any
}
export type ProjectStats = models.ProjectStats
export type IndexingProgress = models.IndexingProgress
export type EmbeddingModelInfo = models.EmbeddingModelInfo
export type EmbeddingCapabilities = models.EmbeddingCapabilities
export interface ONNXRuntimeSettings {
  sharedLibraryPath: string
  activePath?: string
  runtimeAvailable: boolean
  requiresRestart: boolean
}
export interface ONNXRuntimeTestResult {
  success: boolean
  message: string
  error?: string
}

// Represents a semantic chunk of code with metadata
export interface Chunk {
  id: string
  projectId: string
  filePath: string
  content: string
  embedding: number[]
  embeddingModelId?: string
  lineStart: number
  lineEnd: number
  charStart: number
  charEnd: number
  createdAt: number
  updatedAt: number
  // Semantic metadata
  language?: string
  symbolName?: string
  symbolKind?: string
  parent?: string
  signature?: string
  visibility?: string
  packageName?: string
  docString?: string
  tokenCount?: number
  isCollapsed?: boolean
  sourceCode?: string
  // For search results
  similarity?: number
}

// Represents a symbol in the codebase
export interface Symbol {
  id: string
  projectId: string
  name: string
  kind: string
  filePath: string
  line: number
  column: number
}

// Outline node for file structure
export interface OutlineNode {
  id: string
  name: string
  kind: string
  startLine: number
  endLine: number
  children?: OutlineNode[]
}

// Represents a file preview for the indexing scope
export interface FilePreview {
  absolutePath: string
  relativePath: string
  extension: string
  size: string
  hidden: boolean
  lastModified: number
}

// Search filters
export interface SearchFilters {
  filePatterns?: string[]
  symbolKinds?: string[]
  minSimilarity?: number
}

export type OutlineLoadingStatus = 'idle' | 'loading' | 'ready' | 'error'

// Directory tree node that drives the outline explorer
export interface FileTreeNode {
  name: string
  path: string
  isDirectory: boolean
  children: FileTreeNode[]
  expanded: boolean
  outlineNodes?: OutlineNode[]
  outlineStatus?: OutlineLoadingStatus
  outlineError?: string
  chunks?: Chunk[]
}

// Search request
export interface SearchRequest {
  projectId: string
  query: string
  k: number
  filters?: SearchFilters
}

// Search response
export interface SearchResponse {
  chunks: Chunk[]
  totalResults: number
  queryTime: number
}

// Outline request
export interface OutlineRequest {
  projectId: string
  path: string
  depth?: number
}

// Node source request
export interface NodeSourceRequest {
  id: string
  collapseBody?: boolean
}

// Symbol search request
export interface SymbolSearchRequest {
  projectId: string
  query: string
  kinds?: string[]
  limit?: number
}

// MCP Server configuration/state/types
export interface MCPServerConfig {
  host: string
  port: number
  protocol: 'http' | 'stdio'
  autoStart: boolean
  maxConnections: number
}

export interface MCPServerStatus {
  isRunning: boolean
  uptime: number
  activeConnections: number
  totalRequests: number
  averageResponseTime: number
  lastError?: string
}

export interface MCPTool {
  name: string
  description: string
  enabled: boolean
  callCount: number
}
