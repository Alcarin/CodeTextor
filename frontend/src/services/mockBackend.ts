/*
  File: services/mockBackend.ts
  Purpose: Mock implementation of backend services for frontend development.
  Author: CodeTextor project
  Notes: This file provides simulated responses for testing the UI without a real backend.
*/

import type {
  Chunk,
  Symbol,
  OutlineNode,
  IndexingProgress,
  SearchRequest,
  SearchResponse,
  OutlineRequest,
  NodeSourceRequest,
  SymbolSearchRequest,
  ProjectStats
} from '../types';

// Simulates network delay
const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

// Mock data generator for chunks
const generateMockChunk = (id: number, filePath: string, kind: string): Chunk => ({
  id: `chunk-${id}`,
  filePath,
  kind,
  name: `${kind}_${id}`,
  content: `// Mock ${kind} content\nfunction example${id}() {\n  return "mock data";\n}`,
  startLine: id * 10,
  endLine: id * 10 + 5,
  startByte: id * 100,
  endByte: id * 100 + 150,
  similarity: Math.random() * 0.5 + 0.5
});

// Mock data generator for symbols
const generateMockSymbol = (id: number): Symbol => ({
  id: `symbol-${id}`,
  name: `mockFunction${id}`,
  kind: ['function', 'class', 'interface', 'variable'][id % 4],
  filePath: `/src/module${Math.floor(id / 3)}.ts`,
  line: id * 5 + 1,
  column: 0
});

// Mock data generator for outline nodes
const generateMockOutline = (depth: number, maxDepth: number): OutlineNode[] => {
  if (depth > maxDepth) return [];

  return Array.from({ length: 3 }, (_, i) => ({
    id: `node-${depth}-${i}`,
    name: `Node_${depth}_${i}`,
    kind: depth === 0 ? 'class' : 'function',
    startLine: depth * 10 + i * 5,
    endLine: depth * 10 + i * 5 + 4,
    children: depth < maxDepth ? generateMockOutline(depth + 1, maxDepth) : undefined
  }));
};

/**
 * Mock backend service class that simulates the Go backend API.
 * All methods return promises to simulate async operations.
 */
export class MockBackendService {
  private indexingState: IndexingProgress = {
    totalFiles: 0,
    processedFiles: 0,
    currentFile: '',
    status: 'idle'
  };

  /**
   * Starts indexing a project directory.
   * Simulates incremental progress updates.
   * @param projectPath - Path to the project root
   * @returns Promise that resolves when indexing is complete
   */
  async startIndexing(projectPath: string): Promise<void> {
    await delay(300);

    const totalFiles = 50;
    this.indexingState = {
      totalFiles,
      processedFiles: 0,
      currentFile: `${projectPath}/src/main.go`,
      status: 'indexing'
    };

    // Simulate progressive indexing
    for (let i = 1; i <= totalFiles; i++) {
      await delay(100);
      this.indexingState.processedFiles = i;
      this.indexingState.currentFile = `${projectPath}/src/file${i}.go`;
    }

    this.indexingState.status = 'completed';
  }

  /**
   * Gets current indexing progress.
   * @returns Current indexing state
   */
  async getIndexingProgress(): Promise<IndexingProgress> {
    await delay(50);
    return { ...this.indexingState };
  }

  /**
   * Performs semantic search on the indexed codebase.
   * @param request - Search parameters including query and filters
   * @returns Search results with matching chunks
   */
  async semanticSearch(request: SearchRequest): Promise<SearchResponse> {
    await delay(500);

    const mockChunks = Array.from({ length: Math.min(request.k, 10) }, (_, i) =>
      generateMockChunk(
        i,
        `/src/module${i % 3}.ts`,
        ['function_declaration', 'class_declaration', 'method_definition'][i % 3]
      )
    );

    return {
      chunks: mockChunks,
      totalResults: mockChunks.length,
      queryTime: 45
    };
  }

  /**
   * Retrieves the structural outline of a file.
   * @param request - Outline parameters including file path and depth
   * @returns Tree structure of the file
   */
  async getOutline(request: OutlineRequest): Promise<OutlineNode[]> {
    await delay(200);
    const depth = request.depth ?? 2;
    return generateMockOutline(0, depth);
  }

  /**
   * Fetches the source code for a specific node.
   * @param request - Node ID and formatting options
   * @returns Source code string
   */
  async getNodeSource(request: NodeSourceRequest): Promise<string> {
    await delay(100);

    if (request.collapseBody) {
      return `function mockFunction() { ... }`;
    }

    return `function mockFunction() {\n  const result = processData();\n  return result;\n}`;
  }

  /**
   * Searches for symbols by name or pattern.
   * @param request - Symbol search parameters
   * @returns List of matching symbols
   */
  async searchSymbols(request: SymbolSearchRequest): Promise<Symbol[]> {
    await delay(300);

    const limit = request.limit ?? 20;
    return Array.from({ length: Math.min(limit, 15) }, (_, i) =>
      generateMockSymbol(i)
    );
  }

  /**
   * Retrieves project statistics and indexing metadata.
   * @returns Project statistics object
   */
  async getProjectStats(): Promise<ProjectStats> {
    await delay(150);

    return {
      totalFiles: 50,
      totalChunks: 342,
      totalSymbols: 1205,
      indexSize: 2458624, // ~2.4 MB
      lastIndexed: new Date()
    };
  }

  /**
   * Opens a file browser dialog to select a project directory.
   * @returns Selected directory path
   */
  async selectProjectDirectory(): Promise<string> {
    await delay(200);
    return '/home/user/projects/mock-project';
  }
}

// Export singleton instance
export const mockBackend = new MockBackendService();
