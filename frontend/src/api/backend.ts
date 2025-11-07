/**
 * File: backend.ts
 * Purpose: TypeScript wrapper for Wails backend API calls
 * Author: CodeTextor project
 * Notes: Provides type-safe access to Go backend methods
 */

import * as App from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'

/**
 * Backend API wrapper providing type-safe access to Wails bindings.
 * All methods return promises that resolve with backend data or reject with errors.
 */
export const backend = {
  /**
   * Creates a new project with the given name and description.
   * @param name - Project name
   * @param description - Project description
   * @returns Promise resolving to the created project
   */
  async createProject(name: string, description: string): Promise<models.Project> {
    return App.CreateProject(name, description)
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
    config: models.ProjectConfig
  ): Promise<models.Project> {
    return App.UpdateProjectConfig(projectId, config)
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
}

// Export types for use in components
export { models }
export type Project = models.Project
export type ProjectConfig = models.ProjectConfig
export type ProjectStats = models.ProjectStats
