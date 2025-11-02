/*
  File: types/index.ts
  Purpose: TypeScript type definitions for CodeTextor frontend.
  Author: CodeTextor project
  Notes: Defines interfaces for backend data structures and API responses.
*/

// Represents a semantic chunk of code
export interface Chunk {
  id: string;
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
  query: string;
  kinds?: string[];
  limit?: number;
}

// Project statistics
export interface ProjectStats {
  totalFiles: number;
  totalChunks: number;
  totalSymbols: number;
  indexSize: number; // bytes
  lastIndexed?: Date;
}
