// @ts-nocheck
/*
  File: services/mockBackend.ts
  Purpose: Mock implementation of backend services for frontend development.
  Author: CodeTextor project
  Notes: This file provides simulated responses for testing the UI without a real backend.
          DEPRECATED: This file is no longer used. Use backend API instead.
*/

import type {
  Project,
  CreateProjectRequest,
  ProjectListResponse,
  Chunk,
  Symbol,
  OutlineNode,
  IndexingProgress,
  SearchRequest,
  SearchResponse,
  OutlineRequest,
  NodeSourceRequest,
  SymbolSearchRequest,
  ProjectStats,
  MCPServerConfig,
  MCPServerStatus,
  MCPTool
} from '../types';

type DirectorySelectionOptions = {
  prompt?: string;
  startPath?: string;
};

// Simulates network delay
const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

// Mock data generator for chunks
const generateMockChunk = (id: number, projectId: string, filePath: string, kind: string): Chunk => ({
  id: `chunk-${id}`,
  projectId,
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
const generateMockSymbol = (id: number, projectId: string): Symbol => ({
  id: `symbol-${id}`,
  projectId,
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
  // ===== Project Management =====

  private projects: Map<string, Project> = new Map();
  private currentProjectId: string | null = null;

  constructor() {
    this.loadProjectsFromStorage();
  }

  /**
   * Loads projects from localStorage.
   */
  private loadProjectsFromStorage(): void {
    try {
      const stored = localStorage.getItem('codetextor-projects');
      const currentId = localStorage.getItem('codetextor-current-project');

      if (stored) {
        const projectsArray: Project[] = JSON.parse(stored);
        projectsArray.forEach(p => {
          // Convert date strings back to Date objects
          p.createdAt = new Date(p.createdAt);
          if (p.lastIndexed) {
            p.lastIndexed = new Date(p.lastIndexed);
          }
          this.projects.set(p.id, p);
        });
      }

      if (currentId && this.projects.has(currentId)) {
        this.currentProjectId = currentId;
      }
    } catch (error) {
      console.error('Failed to load projects from storage:', error);
    }
  }

  /**
   * Saves projects to localStorage.
   */
  private saveProjectsToStorage(): void {
    try {
      const projectsArray = Array.from(this.projects.values());
      localStorage.setItem('codetextor-projects', JSON.stringify(projectsArray));

      if (this.currentProjectId) {
        localStorage.setItem('codetextor-current-project', this.currentProjectId);
      } else {
        localStorage.removeItem('codetextor-current-project');
      }
    } catch (error) {
      console.error('Failed to save projects to storage:', error);
    }
  }

  /**
   * Creates a new project.
   * @param request - Project creation parameters
   * @returns Created project
   */
  async createProject(request: CreateProjectRequest): Promise<Project> {
    await delay(300);

    const project: Project = {
      id: request.id || `project-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      name: request.name,
      path: request.path,
      description: request.description,
      createdAt: new Date()
    };

    this.projects.set(project.id, project);

    // Set as current project if it's the first one
    if (!this.currentProjectId) {
      this.currentProjectId = project.id;
    }

    this.saveProjectsToStorage();
    return project;
  }

  /**
   * Gets list of all projects.
   * @returns List of projects and current project ID
   */
  async listProjects(): Promise<ProjectListResponse> {
    await delay(100);

    return {
      projects: Array.from(this.projects.values()),
      currentProjectId: this.currentProjectId ?? undefined
    };
  }

  /**
   * Sets the current active project.
   * @param projectId - ID of project to set as current
   */
  async setCurrentProject(projectId: string): Promise<void> {
    await delay(50);

    if (!this.projects.has(projectId)) {
      throw new Error(`Project with ID ${projectId} not found`);
    }

    this.currentProjectId = projectId;
    this.saveProjectsToStorage();
  }

  /**
   * Gets the current active project.
   * @returns Current project or null if none selected
   */
  async getCurrentProject(): Promise<Project | null> {
    await delay(50);

    if (!this.currentProjectId) {
      return null;
    }

    return this.projects.get(this.currentProjectId) ?? null;
  }

  /**
   * Deletes a project.
   * @param projectId - ID of project to delete
   */
  async deleteProject(projectId: string): Promise<void> {
    await delay(200);

    this.projects.delete(projectId);

    // If deleted project was current, unset current project
    if (this.currentProjectId === projectId) {
      this.currentProjectId = null;
      // Set first available project as current if any exist
      const firstProject = this.projects.values().next().value;
      if (firstProject) {
        this.currentProjectId = firstProject.id;
      }
    }

    this.saveProjectsToStorage();
  }

  /**
   * Updates project metadata.
   * @param projectId - ID of project to update
   * @param updates - Partial project data to update
   * @returns Updated project
   */
  async updateProject(projectId: string, updates: Partial<Omit<Project, 'id' | 'createdAt'>>): Promise<Project> {
    await delay(150);

    const project = this.projects.get(projectId);
    if (!project) {
      throw new Error(`Project with ID ${projectId} not found`);
    }

    const updated: Project = {
      ...project,
      ...updates
    };

    this.projects.set(projectId, updated);
    this.saveProjectsToStorage();

    return updated;
  }

  // ===== Indexing Methods =====

  private indexingState: IndexingProgress = {
    totalFiles: 0,
    processedFiles: 0,
    currentFile: '',
    status: 'idle'
  };
  private currentIndexingRun: symbol | null = null;

  /**
   * Starts indexing a project directory.
   * Simulates incremental progress updates.
   * @param projectPath - Path to the project root
   * @returns Promise that resolves when indexing is complete
   */
  async startIndexing(projectPath: string): Promise<void> {
    const runId = Symbol('indexing-run');
    this.currentIndexingRun = runId;

    await delay(300);
    if (this.currentIndexingRun !== runId) {
      return;
    }

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
      if (this.currentIndexingRun !== runId) {
        return;
      }

      this.indexingState.processedFiles = i;
      this.indexingState.currentFile = `${projectPath}/src/file${i}.go`;
    }

    if (this.currentIndexingRun === runId) {
      this.indexingState.status = 'completed';
    }
  }

  /**
   * Stops the indexing process.
   */
  async stopIndexing(): Promise<void> {
    await delay(100);
    this.currentIndexingRun = null;
    this.indexingState = {
      totalFiles: 0,
      processedFiles: 0,
      currentFile: '',
      status: 'idle'
    };
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
        request.projectId,
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
      generateMockSymbol(i, request.projectId)
    );
  }

  /**
   * Retrieves project statistics and indexing metadata.
   * @returns Project statistics object
   */
  async getProjectStats(): Promise<ProjectStats> {
    await delay(150);

    const isIndexing = Math.random() > 0.5;
    const indexingProgress = isIndexing ? Math.min(1, 0.15 + Math.random() * 0.7) : 0;
    const now = Date.now();

    return {
      totalFiles: 50 + Math.floor(Math.random() * 12),
      totalChunks: 342 + Math.floor(Math.random() * 85),
      totalSymbols: 1205 + Math.floor(Math.random() * 220),
      databaseSize: 2458624 + Math.floor(Math.random() * 300000), // ~2.4 MB area
      lastIndexedAt: new Date(now - Math.floor(Math.random() * 45 * 60 * 1000)),
      isIndexing,
      indexingProgress
    };
  }

  /**
   * Opens a file browser dialog to select a project directory.
   * @returns Selected directory path
   */
  async selectProjectDirectory(): Promise<string> {
    const selection = await this.selectDirectory({
      prompt: 'Select the project root directory',
      startPath: '/home/user/projects'
    });

    return selection ?? '/home/user/projects/mock-project';
  }

  /**
   * Opens a folder selection dialog.
   * @param options - Prompt and initial directory (mocked)
   * @returns Selected directory path or null when cancelled
   */
  async selectDirectory(options?: DirectorySelectionOptions): Promise<string | null> {
    await delay(180);

    const base = options?.startPath ?? '/home/user/projects/mock-project';
    const normalizedBase = base.replace(/\/+$/, '');
    const candidates = [
      normalizedBase,
      `${normalizedBase}/src`,
      `${normalizedBase}/backend`,
      `${normalizedBase}/frontend`,
      `${normalizedBase}/tests`,
      `${normalizedBase}/docs`
    ];

    const pick = candidates[Math.floor(Math.random() * candidates.length)];
    return pick;
  }

  // ===== MCP Server Methods =====

  private mcpServerRunning = false;
  private mcpStartTime = 0;
  private mcpRequestCount = 0;

  /**
   * Gets current MCP server configuration.
   * @returns MCP server configuration
   */
  async getMCPConfig(): Promise<MCPServerConfig> {
    await delay(100);
    return {
      host: 'localhost',
      port: 3000,
      protocol: 'http',
      autoStart: false,
      maxConnections: 10
    };
  }

  /**
   * Updates MCP server configuration.
   * @param config - New configuration settings
   */
  async updateMCPConfig(config: Partial<MCPServerConfig>): Promise<void> {
    await delay(150);
    console.log('Updated MCP config:', config);
  }

  /**
   * Starts the MCP server.
   */
  async startMCPServer(): Promise<void> {
    await delay(500);
    this.mcpServerRunning = true;
    this.mcpStartTime = Date.now();
    this.mcpRequestCount = 0;
  }

  /**
   * Stops the MCP server.
   */
  async stopMCPServer(): Promise<void> {
    await delay(300);
    this.mcpServerRunning = false;
    this.mcpStartTime = 0;
  }

  /**
   * Gets current MCP server status.
   * @returns MCP server status
   */
  async getMCPStatus(): Promise<MCPServerStatus> {
    await delay(50);

    const uptime = this.mcpServerRunning && this.mcpStartTime > 0
      ? Math.floor((Date.now() - this.mcpStartTime) / 1000)
      : 0;

    // Simulate increasing request count
    if (this.mcpServerRunning) {
      this.mcpRequestCount += Math.floor(Math.random() * 3);
    }

    return {
      isRunning: this.mcpServerRunning,
      uptime,
      activeConnections: this.mcpServerRunning ? Math.floor(Math.random() * 3) : 0,
      totalRequests: this.mcpRequestCount,
      averageResponseTime: 45 + Math.random() * 20
    };
  }

  /**
   * Gets list of available MCP tools.
   * @returns Array of MCP tools
   */
  async getMCPTools(): Promise<MCPTool[]> {
    await delay(100);

    return [
      { name: 'search', description: 'Semantic chunk search', enabled: true, callCount: 142 },
      { name: 'outline', description: 'File outline tree', enabled: true, callCount: 87 },
      { name: 'nodeSource', description: 'Source snippet for a chunk/outline node', enabled: true, callCount: 98 }
    ];
  }

  /**
   * Toggles MCP tool enabled state.
   * @param toolName - Name of the tool to toggle
   */
  async toggleMCPTool(toolName: string): Promise<void> {
    await delay(100);
    console.log('Toggled MCP tool:', toolName);
  }
}

// Export singleton instance
export const mockBackend = new MockBackendService();
