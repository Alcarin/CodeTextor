/*
  File: types/index.ts
  Purpose: Central TypeScript definitions shared across the frontend.
  Author: CodeTextor project
*/

import type { models } from '../api/backend'

// Re-export backend generated types for convenience
export type { Project, ProjectConfig, ProjectStats } from '../api/backend'
export type IndexingProgress = models.IndexingProgress

// Represents a semantic chunk of code
export interface Chunk {
  id: string
  projectId: string
  filePath: string
  kind: string
  name: string
  content: string
  startLine: number
  endLine: number
  startByte: number
  endByte: number
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
