<!--
  File: views/ProjectsView.vue
  Purpose: Project management interface for creating, selecting, and deleting projects.
  Author: CodeTextor project
  Notes: Dedicated view for multi-project management.
-->

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';
import { mockBackend } from '../services/mockBackend';
import type { Project, CreateProjectRequest } from '../types';

// Get composables
const { currentProject, setCurrentProject, clearCurrentProject } = useCurrentProject();
const { navigateTo } = useNavigation();

// State
const projects = ref<Project[]>([]);
const isLoading = ref<boolean>(false);
const showProjectForm = ref<boolean>(false);
const showDeleteConfirm = ref<boolean>(false);
const projectToDelete = ref<Project | null>(null);
const projectToEdit = ref<Project | null>(null);

// Form state
const projectName = ref<string>('');
const projectId = ref<string>('');
const projectDescription = ref<string>('');
const isSavingProject = ref<boolean>(false);

const isEditMode = computed(() => projectToEdit.value !== null);

/**
 * Slugify a string
 * @param text The text to slugify
 * @returns Slugified text
 */
const slugify = (text: string): string => {
  return text
    .toString()
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-') // Replace spaces with -
    .replace(/[^\w\-]+/g, '') // Remove all non-word chars
    .replace(/--+/g, '-'); // Replace multiple - with single -
};

watch(projectName, (newName) => {
  if (!isEditMode.value) {
    projectId.value = slugify(newName);
  }
});

/**
 * Loads all projects from backend.
 */
const loadProjects = async () => {
  isLoading.value = true;
  try {
    const response = await mockBackend.listProjects();
    projects.value = response.projects;
  } catch (error) {
    console.error('Failed to load projects:', error);
    alert('Failed to load projects: ' + (error instanceof Error ? error.message : 'Unknown error'));
  } finally {
    isLoading.value = false;
  }
};

/**
 * Resets form state.
 */
const resetForm = () => {
  projectName.value = '';
  projectDescription.value = '';
  projectId.value = '';
  projectToEdit.value = null;
};

/**
 * Opens the create project form.
 */
const openCreateForm = () => {
  resetForm();
  showProjectForm.value = true;
};

/**
 * Opens the edit project form.
 * @param project - The project to edit.
 */
const openEditForm = (project: Project) => {
  resetForm();
  projectToEdit.value = project;
  projectName.value = project.name;
  projectDescription.value = project.description || '';
  projectId.value = project.id;
  showProjectForm.value = true;
};

/**
 * Cancels project creation/editing.
 */
const cancelSave = () => {
  showProjectForm.value = false;
  resetForm();
};

/**
 * Saves a project (creates or updates).
 */
const saveProject = async () => {
  if (!projectName.value.trim()) {
    alert('Please enter a project name');
    return;
  }
  if (!projectId.value.trim()) {
    alert('Please enter a project ID');
    return;
  }

  isSavingProject.value = true;

  try {
    if (isEditMode.value && projectToEdit.value) {
      // Update existing project
      const updatedProject = await mockBackend.updateProject(projectToEdit.value.id, {
        name: projectName.value,
        description: projectDescription.value || undefined,
      });

      // Update project in the list
      const index = projects.value.findIndex(p => p.id === updatedProject.id);
      if (index !== -1) {
        projects.value[index] = updatedProject;
      }
    } else {
      // Create new project
      const request: CreateProjectRequest = {
        id: projectId.value,
        name: projectName.value,
        // The user does not want to specify the path, so we use the project id.
        path: `/path/to/${projectId.value}`,
        description: projectDescription.value || undefined
      };

      const newProject = await mockBackend.createProject(request);
      projects.value.push(newProject);
      await setCurrentProject(newProject);
      navigateTo('indexing');
    }

    showProjectForm.value = false;
    resetForm();
  } catch (error) {
    console.error('Failed to save project:', error);
    alert('Failed to save project: ' + (error instanceof Error ? error.message : 'Unknown error'));
  } finally {
    isSavingProject.value = false;
  }
};


/**
 * Selects a project as current.
 * @param project - Project to select
 */
const selectProject = async (project: Project) => {
  try {
    await setCurrentProject(project);
  } catch (error) {
    console.error('Failed to select project:', error);
    alert('Failed to select project: ' + (error instanceof Error ? error.message : 'Unknown error'));
  }
};

/**
 * Opens delete confirmation dialog.
 * @param project - Project to delete
 */
const confirmDelete = (project: Project) => {
  projectToDelete.value = project;
  showDeleteConfirm.value = true;
};

/**
 * Cancels project deletion.
 */
const cancelDelete = () => {
  projectToDelete.value = null;
  showDeleteConfirm.value = false;
};

/**
 * Deletes the selected project.
 */
const deleteProject = async () => {
  if (!projectToDelete.value) return;

  try {
    await mockBackend.deleteProject(projectToDelete.value.id);

    // Remove from list
    projects.value = projects.value.filter(p => p.id !== projectToDelete.value!.id);

    // Clear current project if it was deleted
    if (currentProject.value?.id === projectToDelete.value.id) {
      clearCurrentProject();
    }

    // Close dialog
    showDeleteConfirm.value = false;
    projectToDelete.value = null;
  } catch (error) {
    console.error('Failed to delete project:', error);
    alert('Failed to delete project: ' + (error instanceof Error ? error.message : 'Unknown error'));
  }
};

/**
 * Formats date to relative time.
 * @param date - Date to format
 * @returns Formatted string
 */
const formatDate = (date?: Date): string => {
  if (!date) return 'Never';

  const now = new Date();
  const target = new Date(date);
  const diffMs = now.getTime() - target.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return target.toLocaleDateString();
};

// Load projects on mount
onMounted(() => {
  loadProjects();
});
</script>

<template>
  <div class="projects-view">
    <!-- Info banner -->
    <div class="info-section section">
      <div class="info-banner">
        <span class="info-icon">ℹ️</span>
        <div class="info-content">
          <strong>Multi-Project Architecture:</strong> Each project has its own isolated SQLite-vec database
          stored at <code>indexes/&lt;projectId&gt;.db</code>. Projects are completely independent with
          no data mixing or cross-contamination.
        </div>
      </div>
    </div>

    <!-- Actions -->
    <div class="actions-bar">
      <button @click="openCreateForm" class="btn btn-primary">
        + Create New Project
      </button>
      <button @click="loadProjects" :disabled="isLoading" class="btn btn-secondary">
        {{ isLoading ? 'Refreshing...' : '↻ Refresh' }}
      </button>
    </div>

    <!-- Projects list -->
    <div v-if="isLoading" class="loading-state section">
      <div class="spinner"></div>
      <p>Loading projects...</p>
    </div>

    <div v-else-if="projects.length === 0" class="empty-state section">
      <div class="empty-icon">📁</div>
      <h3>No Projects Yet</h3>
      <p>Create your first project to get started with code indexing and semantic search.</p>
      <button @click="openCreateForm" class="btn btn-primary" style="margin-top: 1rem">
        Create Your First Project
      </button>
    </div>

    <div v-else class="projects-grid">
      <div
        v-for="project in projects"
        :key="project.id"
        :class="['project-card', { active: currentProject?.id === project.id }]"
      >
        <!-- Active badge -->
        <div v-if="currentProject?.id === project.id" class="active-badge">
          ● Active
        </div>

        <!-- Project header -->
        <div class="project-header">
          <span class="project-icon">📁</span>
          <div class="project-info">
            <h3>{{ project.name }}</h3>
            <p v-if="project.description" class="project-description">
              {{ project.description }}
            </p>
          </div>
        </div>

        <!-- Project details -->
        <div class="project-details">
          <div class="detail-row">
            <span class="detail-label">Project ID:</span>
            <code class="detail-value">{{ project.id }}</code>
          </div>
          <div class="detail-row">
            <span class="detail-label">Database:</span>
            <code class="detail-value db-path">indexes/{{ project.id }}.db</code>
          </div>
          <div class="detail-row">
            <span class="detail-label">Created:</span>
            <span class="detail-value">{{ formatDate(project.createdAt) }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Last Indexed:</span>
            <span class="detail-value">{{ formatDate(project.lastIndexed) }}</span>
          </div>
        </div>

        <!-- Actions -->
        <div class="project-actions">
          <button
            v-if="currentProject?.id !== project.id"
            @click="selectProject(project)"
            class="btn btn-primary btn-sm"
          >
            Select Project
          </button>
          <button
            v-else
            @click="navigateTo('indexing')"
            class="btn btn-success btn-sm"
          >
            Go to Indexing
          </button>
          <button @click="openEditForm(project)" class="btn btn-secondary btn-sm" data-testid="edit-project-button">
            Edit
          </button>
          <button
            @click="confirmDelete(project)"
            class="btn btn-danger btn-sm"
          >
            Delete
          </button>
        </div>
      </div>
    </div>

    <!-- Create/Edit Project Modal -->
    <div v-if="showProjectForm" class="modal-overlay" @click="cancelSave">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3>{{ isEditMode ? 'Edit Project' : 'Create New Project' }}</h3>
          <button class="modal-close" @click="cancelSave">&times;</button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label for="project-name">Project Name *</label>
            <input
              id="project-name"
              v-model="projectName"
              type="text"
              placeholder="My Awesome Project"
              class="form-input"
              :disabled="isSavingProject"
            />
          </div>

          <div class="form-group">
            <label for="project-id">Project ID *</label>
            <input
              id="project-id"
              v-model="projectId"
              type="text"
              placeholder="my-awesome-project"
              class="form-input"
              :disabled="isSavingProject || isEditMode"
            />
          </div>

          <div class="form-group">
            <label for="project-description">Description (optional)</label>
            <textarea
              id="project-description"
              v-model="projectDescription"
              placeholder="A brief description of this project..."
              class="form-textarea"
              rows="3"
              :disabled="isSavingProject"
            ></textarea>
          </div>
        </div>

        <div class="modal-footer">
          <button
            @click="cancelSave"
            :disabled="isSavingProject"
            class="btn btn-secondary"
          >
            Cancel
          </button>
          <button
            @click="saveProject"
            :disabled="!projectName || !projectId || isSavingProject"
            class="btn btn-success"
          >
            {{ isSavingProject ? 'Saving...' : (isEditMode ? 'Save Changes' : 'Create Project') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div v-if="showDeleteConfirm && projectToDelete" class="modal-overlay" @click="cancelDelete">
      <div class="modal-content modal-sm" @click.stop>
        <div class="modal-header">
          <h3>Delete Project?</h3>
          <button class="modal-close" @click="cancelDelete">&times;</button>
        </div>

        <div class="modal-body">
          <div class="warning-message">
            <span class="warning-icon">⚠️</span>
            <div>
              <p><strong>Are you sure you want to delete "{{ projectToDelete.name }}"?</strong></p>
              <p>This will permanently remove:</p>
              <ul>
                <li>Database file: <code>indexes/{{ projectToDelete.id }}.db</code></li>
                <li>All indexed chunks and embeddings</li>
                <li>Project configuration</li>
              </ul>
              <p><strong>This action cannot be undone.</strong></p>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button @click="cancelDelete" class="btn btn-secondary">
            Cancel
          </button>
          <button @click="deleteProject" class="btn btn-danger">
            Delete Project
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.projects-view {
  max-width: 1400px;
  margin: 0 auto;
}

.section {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

/* Info banner */
.info-section {
  margin-bottom: 1.5rem;
}

.info-banner {
  display: flex;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  background: #1a3a5a;
  border: 1px solid #007acc;
  border-radius: 4px;
  align-items: flex-start;
}

.info-icon {
  font-size: 1.2rem;
  flex-shrink: 0;
}

.info-content {
  flex: 1;
  color: #7fc7ff;
  font-size: 0.9rem;
  line-height: 1.5;
}

.info-content strong {
  color: #a8d8ff;
}

.info-content code {
  background: #0d2438;
  padding: 0.2rem 0.5rem;
  border-radius: 3px;
  color: #4ec9b0;
  font-family: 'Courier New', monospace;
  font-size: 0.9em;
  border: 1px solid #1a4a6e;
}

/* Actions bar */
.actions-bar {
  display: flex;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

/* Projects grid */
.projects-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 1.5rem;
}

.project-card {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  transition: all 0.2s ease;
  position: relative;
}

.project-card:hover {
  border-color: #007acc;
  box-shadow: 0 4px 12px rgba(0, 122, 204, 0.2);
}

.project-card.active {
  border-color: #28a745;
  background: #1a2e1a;
  box-shadow: 0 4px 12px rgba(40, 167, 69, 0.2);
}

.active-badge {
  position: absolute;
  top: 1rem;
  right: 1rem;
  background: #28a745;
  color: white;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
}

.project-header {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
  margin-bottom: 1rem;
}

.project-icon {
  font-size: 2rem;
}

.project-info {
  flex: 1;
}

.project-info h3 {
  margin: 0 0 0.5rem 0;
  color: #d4d4d4;
  font-size: 1.2rem;
}

.project-description {
  margin: 0;
  color: #858585;
  font-size: 0.9rem;
  line-height: 1.4;
}

.project-details {
  background: #1e1e1e;
  border-radius: 4px;
  padding: 1rem;
  margin-bottom: 1rem;
}

.detail-row {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
  font-size: 0.85rem;
}

.detail-row:last-child {
  margin-bottom: 0;
}

.detail-label {
  color: #858585;
  font-weight: 600;
  min-width: 100px;
}

.detail-value {
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
  word-break: break-all;
}

.detail-value.db-path {
  color: #4ec9b0;
  background: #1a1a1a;
  padding: 0.2rem 0.5rem;
  border-radius: 3px;
}

.project-actions {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
}

/* Buttons */
.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s ease;
  font-weight: 500;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-sm {
  padding: 0.5rem 1rem;
  font-size: 0.85rem;
  flex: 1;
}

.btn-primary {
  background: #007acc;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #005a9e;
}

.btn-success {
  background: #28a745;
  color: white;
}

.btn-success:hover:not(:disabled) {
  background: #218838;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover:not(:disabled) {
  background: #5a6268;
}

.btn-danger {
  background: #dc3545;
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background: #c82333;
}

/* States */
.loading-state,
.empty-state {
  text-align: center;
  padding: 3rem;
  color: #858585;
}

.spinner {
  border: 4px solid #3e3e42;
  border-top-color: #007acc;
  border-radius: 50%;
  width: 40px;
  height: 40px;
  animation: spin 1s linear infinite;
  margin: 0 auto 1rem;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.empty-icon {
  font-size: 4rem;
  margin-bottom: 1rem;
}

.empty-state h3 {
  margin: 0 0 0.5rem 0;
  color: #d4d4d4;
}

/* Modal styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.modal-content {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  width: 90%;
  max-width: 600px;
  max-height: 90vh;
  overflow-y: auto;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
}

.modal-content.modal-sm {
  max-width: 500px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem;
  border-bottom: 1px solid #3e3e42;
}

.modal-header h3 {
  margin: 0;
  color: #d4d4d4;
  font-size: 1.3rem;
}

.modal-close {
  background: none;
  border: none;
  color: #858585;
  font-size: 2rem;
  cursor: pointer;
  line-height: 1;
  padding: 0;
  width: 32px;
  height: 32px;
  transition: color 0.2s ease;
}

.modal-close:hover {
  color: #d4d4d4;
}

.modal-body {
  padding: 1.5rem;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1.5rem;
  border-top: 1px solid #3e3e42;
}

/* Form styles */
.form-group {
  margin-bottom: 1.5rem;
}

.form-group:last-child {
  margin-bottom: 0;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  color: #d4d4d4;
  font-size: 0.9rem;
  font-weight: 500;
}

.form-input,
.form-textarea {
  width: 100%;
  padding: 0.75rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  color: #d4d4d4;
  font-size: 0.95rem;
  font-family: inherit;
  transition: border-color 0.2s ease;
  box-sizing: border-box;
}

.form-input:focus,
.form-textarea:focus {
  outline: none;
  border-color: #007acc;
}

.form-input:disabled,
.form-textarea:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.form-textarea {
  resize: vertical;
  min-height: 80px;
}

/* Warning message */
.warning-message {
  display: flex;
  gap: 1rem;
  padding: 1rem;
  background: #5a3a1a;
  border: 1px solid #ffc107;
  border-radius: 4px;
  color: #ffd966;
}

.warning-icon {
  font-size: 2rem;
  flex-shrink: 0;
}

.warning-message p {
  margin: 0 0 0.75rem 0;
}

.warning-message p:last-child {
  margin-bottom: 0;
}

.warning-message ul {
  margin: 0.5rem 0;
  padding-left: 1.5rem;
}

.warning-message code {
  background: #3a2a1a;
  padding: 0.2rem 0.5rem;
  border-radius: 3px;
  color: #4ec9b0;
  font-family: 'Courier New', monospace;
}
</style>
