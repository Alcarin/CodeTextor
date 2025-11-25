/**
 * File: backend.ts
 * Purpose: TypeScript wrapper for Wails backend API calls
 * Author: CodeTextor project
 * Notes: Provides type-safe access to Go backend methods
 */

import * as App from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import type { ONNXRuntimeSettings, ONNXRuntimeTestResult } from '../types'

type ProjectConfigInput = Omit<models.ProjectConfig, 'convertValues'> & {
  convertValues?: () => void
}

const toBackendConfig = (config: ProjectConfigInput): models.ProjectConfig => {
  if (config instanceof models.ProjectConfig) {
    return config
  }
  const converted = models.ProjectConfig.createFrom(config)
  if (!converted.convertValues) {
    converted.convertValues = () => {}
  }
  return converted
}

/**
 * Backend API wrapper providing type-safe access to Wails bindings.
 * All methods return promises that resolve with backend data or reject with errors.
 */
export const backend = {
  /**
   * Creates a new project with the given name, description, and optional slug.
   * @param name - Project name
   * @param description - Project description
   * @param slug - Optional slug (leave empty to auto-generate from name)
   * @returns Promise resolving to the created project
   */
  async createProject(name: string, description: string, slug: string = '', rootPath: string): Promise<models.Project> {
    return App.CreateProject(name, description, slug, rootPath)
  },

  /**
   * Retrieves a project by its ID.
   * @param projectId - Unique project identifier
   * @returns Promise resolving to the project or null if not found
   */
  async getProject(projectId: string): Promise<models.Project> {
    return App.GetProject(projectId)
  },

  /**
   * Lists all projects ordered by creation time (newest first).
   * @returns Promise resolving to array of projects
   */
  async listProjects(): Promise<models.Project[]> {
    return App.ListProjects()
  },

  /**
   * Updates a project's name and description.
   * @param projectId - Project identifier
   * @param name - New project name
   * @param description - New project description
   * @returns Promise resolving to the updated project
   */
  async updateProject(
    projectId: string,
    name: string,
    description: string
  ): Promise<models.Project> {
    return App.UpdateProject(projectId, name, description)
  },

  /**
   * Updates a project's configuration settings.
   * @param projectId - Project identifier
   * @param config - New project configuration
   * @returns Promise resolving to the updated project
   */
  async updateProjectConfig(
    projectId: string,
    config: ProjectConfigInput
  ): Promise<models.Project> {
    return App.UpdateProjectConfig(projectId, toBackendConfig(config))
  },

  /**
   * Deletes a project from the database.
   * Note: This does not delete the project's index database file.
   * @param projectId - Project identifier
   * @returns Promise that resolves when deletion is complete
   */
  async deleteProject(projectId: string): Promise<void> {
    return App.DeleteProject(projectId)
  },

  /**
   * Checks if a project with the given ID exists.
   * @param projectId - Project identifier
   * @returns Promise resolving to true if project exists, false otherwise
   */
  async projectExists(projectId: string): Promise<boolean> {
    return App.ProjectExists(projectId)
  },

  /**
   * Sets the currently selected project.
   * Only one project can be selected at a time.
   * @param projectId - Project identifier
   * @returns Promise that resolves when selection is updated
   */
  async setSelectedProject(projectId: string): Promise<void> {
    return App.SetSelectedProject(projectId)
  },

  /**
   * Gets the currently selected project.
   * Automatically selects the oldest project if none is selected.
   * @returns Promise resolving to the selected project or null if no projects exist
   */
  async getSelectedProject(): Promise<models.Project | null> {
    return App.GetSelectedProject()
  },

  /**
   * Clears the currently selected project.
   * @returns Promise that resolves when selection is cleared
   */
  async clearSelectedProject(): Promise<void> {
    return App.ClearSelectedProject()
  },

  /**
   * Enables or disables continuous indexing for a project.
   * @param projectId - Project identifier
   * @param enabled - true to enable indexing, false to disable
   * @returns Promise that resolves when indexing state is updated
   */
  async setProjectIndexing(projectId: string, enabled: boolean): Promise<void> {
    return App.SetProjectIndexing(projectId, enabled)
  },

  /**
   * Retrieves a preview of files to be indexed based on project configuration.
   * @param projectId - Project identifier
   * @param config - Indexing configuration
   * @returns Promise resolving to an array of file previews
   */
  async getFilePreviews(
    projectId: string,
    config: ProjectConfigInput
  ): Promise<models.FilePreview[]> {
    return App.GetFilePreviews(projectId, toBackendConfig(config))
  },

  /**
   * Retrieves the outline tree for a specific file.
   */
  async getFileOutline(
    projectId: string,
    path: string
  ): Promise<models.OutlineNode[]> {
    return App.GetFileOutline(projectId, path)
  },

  /**
   * Retrieves update timestamps for all file outlines in a project.
   * Returns a map of relative file paths to Unix timestamps.
   */
  async getOutlineTimestamps(projectId: string): Promise<Record<string, number>> {
    return App.GetOutlineTimestamps(projectId)
  },

  /**
   * Retrieves all semantic chunks for a specific file.
   * @param projectId - Project identifier
   * @param filePath - File path relative to project root
   * @returns Promise resolving to an array of chunks
   */
  async getFileChunks(projectId: string, filePath: string): Promise<models.Chunk[]> {
    return App.GetFileChunks(projectId, filePath)
  },

  /**
   * Reads the content of a file within a project.
   * @param projectId - Project identifier
   * @param relativePath - File path relative to project root
   * @returns Promise resolving to the file content as a string
   */
  async readFileContent(projectId: string, relativePath: string): Promise<string> {
    return App.ReadFileContent(projectId, relativePath)
  },

  /**
   * Reads glob patterns from the project's .gitignore (if present).
   */
  async getGitignorePatterns(projectId: string): Promise<string[]> {
    return App.GetGitignorePatterns(projectId)
  },
  async getEmbeddingCapabilities(): Promise<models.EmbeddingCapabilities> {
    return App.GetEmbeddingCapabilities()
  },
  async getONNXRuntimeSettings(): Promise<ONNXRuntimeSettings> {
    return App.GetONNXRuntimeSettings() as unknown as ONNXRuntimeSettings
  },
  async updateONNXRuntimeSettings(path: string): Promise<ONNXRuntimeSettings> {
    return App.UpdateONNXRuntimeSettings(path) as unknown as ONNXRuntimeSettings
  },
  async testONNXRuntimePath(path: string): Promise<ONNXRuntimeTestResult> {
    return App.TestONNXRuntimePath(path) as unknown as ONNXRuntimeTestResult
  },
  async listEmbeddingModels(): Promise<models.EmbeddingModelInfo[]> {
    return App.ListEmbeddingModels()
  },
  async saveEmbeddingModel(model: models.EmbeddingModelInfo): Promise<models.EmbeddingModelInfo> {
    return App.SaveEmbeddingModel(model)
  },
  async downloadEmbeddingModel(modelId: string): Promise<models.EmbeddingModelInfo> {
    return App.DownloadEmbeddingModel(modelId)
  },

  async search(projectId: string, query: string, k: number): Promise<models.SearchResponse> {
    return App.Search(projectId, query, k)
  },

  /**
   * Opens a directory selection dialog.
   * @param title - Dialog title
   * @param defaultDirectory - Starting directory
   * @returns Promise resolving to the selected directory path
   */
  async selectDirectory(title: string, defaultDirectory: string): Promise<string> {
    return App.SelectDirectory(title, defaultDirectory)
  },

  /**
   * Starts the indexing process for a project.
   * @param projectId - Project identifier
   * @returns Promise that resolves when indexing has started
   */
  async startIndexing(projectId: string): Promise<void> {
    return App.StartIndexing(projectId)
  },

  /**
   * Clears the existing index and runs a fresh indexing pass.
   * @param projectId - Project identifier
   */
  async reindexProject(projectId: string): Promise<void> {
    return App.ReindexProject(projectId)
  },

  /**
   * Deletes all indexed artifacts for a project without starting a new run.
   * @param projectId - Project identifier
   */
  async resetProjectIndex(projectId: string): Promise<void> {
    return App.ResetProjectIndex(projectId)
  },

  /**
   * Stops the indexing process for a project.
   * @param projectId - Project identifier
   * @returns Promise that resolves when indexing has stopped
   */
  async stopIndexing(projectId: string): Promise<void> {
    return App.StopIndexing(projectId)
  },

  /**
   * Gets the current indexing progress for a project.
   * @param projectId - Project identifier
   * @returns Promise resolving to the indexing progress
   */
  async getIndexingProgress(projectId: string): Promise<models.IndexingProgress> {
    return App.GetIndexingProgress(projectId)
  },

  /**
   * Gets statistics for a specific project.
   * @param projectId - Project identifier
   * @returns Promise resolving to project statistics
   */
  async getProjectStats(projectId: string): Promise<models.ProjectStats> {
    return App.GetProjectStats(projectId)
  },

  /**
   * Gets cumulative statistics across all projects.
   * @returns Promise resolving to aggregate project statistics
   */
  async getAllProjectsStats(): Promise<models.ProjectStats> {
    return App.GetAllProjectsStats()
  },
}

// Export types for use in components
export { models }
export type Project = models.Project
export type ProjectConfig = models.ProjectConfig
export type ProjectStats = models.ProjectStats
