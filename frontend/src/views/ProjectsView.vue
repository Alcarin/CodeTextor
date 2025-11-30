<!--
  File: views/ProjectsView.vue
  Purpose: Project management interface for creating, selecting, and deleting projects.
  Author: CodeTextor project
  Notes: Dedicated view for multi-project management.
-->

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';
import { backend } from '../api/backend';
import type { Project, ONNXRuntimeSettings, ONNXRuntimeTestResult, MCPServerConfig } from '../types';
import ProjectCard from '../components/ProjectCard.vue';
import ProjectTable from '../components/ProjectTable.vue';
import DeleteConfirmModal from '../components/DeleteConfirmModal.vue';
import ProjectFormModal from '../components/ProjectFormModal.vue';

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
const viewMode = ref<'grid' | 'table'>('grid'); // Default to grid view
const showSettingsModal = ref<boolean>(false);
const runtimeSettings = ref<ONNXRuntimeSettings | null>(null);
const runtimePathInput = ref<string>('');
const runtimeTestResult = ref<ONNXRuntimeTestResult | null>(null);
const runtimeError = ref<string>('');
const settingsLoading = ref<boolean>(false);
const settingsSaving = ref<boolean>(false);
const settingsTesting = ref<boolean>(false);
const mcpConfig = ref<MCPServerConfig | null>(null);
const mcpConfigForm = ref<{ host: string; port: number; autoStart: boolean }>({
  host: '127.0.0.1',
  port: 3030,
  autoStart: false
});
const mcpSettingsLoading = ref<boolean>(false);
const mcpSettingsSaving = ref<boolean>(false);
const mcpSettingsError = ref<string>('');
const filePickerLoading = ref<boolean>(false);

const normalizeProtocol = (protocol: string): MCPServerConfig['protocol'] => {
  return protocol === 'stdio' ? 'stdio' : 'http';
};

/**
 * Loads all projects from backend.
 */
const loadProjects = async () => {
  isLoading.value = true;
  try {
    projects.value = await backend.listProjects();
  } catch (error) {
    console.error('Failed to load projects:', error);
    alert('Failed to load projects: ' + (error instanceof Error ? error.message : 'Unknown error'));
  } finally {
    isLoading.value = false;
  }
};

/**
 * Opens the create project form.
 */
const openCreateForm = () => {
  projectToEdit.value = null;
  showProjectForm.value = true;
};

/**
 * Opens the edit project form.
 * @param project - The project to edit.
 */
const openEditForm = (project: Project) => {
  projectToEdit.value = project;
  showProjectForm.value = true;
};

/**
 * Cancels project form.
 */
const cancelProjectForm = () => {
  showProjectForm.value = false;
  projectToEdit.value = null;
};

/**
 * Handles project save from modal.
 * @param project - The saved project
 */
const handleProjectSave = async (project: Project) => {
  // Update or add to list
  const index = projects.value.findIndex((p: Project) => p.id === project.id);
  if (index !== -1) {
    // Update existing
    projects.value[index] = project;
  } else {
    // Add new
    projects.value.push(project);
    await setCurrentProject(project);
    navigateTo('indexing');
  }

  showProjectForm.value = false;
  projectToEdit.value = null;
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
    await backend.deleteProject(projectToDelete.value.id);

    // Remove from list
    projects.value = projects.value.filter((p: Project) => p.id !== projectToDelete.value!.id);

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
 * Selects a project and navigates to indexing view.
 * @param project - Project to select
 */
const goToIndexing = async (project: Project) => {
  try {
    await setCurrentProject(project);
    navigateTo('indexing');
  } catch (error) {
    console.error('Failed to select project:', error);
    alert('Failed to select project: ' + (error instanceof Error ? error.message : 'Unknown error'));
  }
};

const refreshRuntimeSettings = async () => {
  settingsLoading.value = true;
  runtimeError.value = '';
  runtimeTestResult.value = null;
  try {
    const settings = await backend.getONNXRuntimeSettings();
    runtimeSettings.value = settings;
    runtimePathInput.value = settings.sharedLibraryPath || '';
  } catch (error: any) {
    runtimeError.value = error?.message || 'Unable to load ONNX runtime settings.';
  } finally {
    settingsLoading.value = false;
  }
};

const browseRuntimePath = async () => {
  filePickerLoading.value = true;
  runtimeError.value = '';
  try {
    const selected = await backend.selectFile(
      'Select ONNX runtime shared library',
      runtimePathInput.value || runtimeSettings.value?.sharedLibraryPath || '',
      '*.so;*.dylib;*.dll'
    );
    if (selected) {
      runtimePathInput.value = selected;
      runtimeTestResult.value = null;
    }
  } catch (error: any) {
    runtimeError.value = error?.message || 'Unable to open file picker.';
  } finally {
    filePickerLoading.value = false;
  }
};

const clearRuntimePath = () => {
  runtimePathInput.value = '';
  runtimeTestResult.value = null;
  runtimeError.value = '';
};

const refreshMCPConfig = async () => {
  mcpSettingsLoading.value = true;
  mcpSettingsError.value = '';
  try {
    const cfg = await backend.getMCPConfig();
    const normalized: MCPServerConfig = {
      host: cfg.host,
      port: cfg.port,
      protocol: normalizeProtocol(cfg.protocol),
      autoStart: cfg.autoStart,
      maxConnections: cfg.maxConnections
    };
    mcpConfig.value = normalized;
    mcpConfigForm.value = {
      host: normalized.host,
      port: normalized.port,
      autoStart: normalized.autoStart
    };
  } catch (error: any) {
    mcpSettingsError.value = error?.message || 'Unable to load MCP server configuration.';
  } finally {
    mcpSettingsLoading.value = false;
  }
};

const openSettings = async () => {
  showSettingsModal.value = true;
  await Promise.all([refreshRuntimeSettings(), refreshMCPConfig()]);
};

const closeSettings = () => {
  showSettingsModal.value = false;
  runtimeTestResult.value = null;
  runtimeError.value = '';
  mcpSettingsError.value = '';
  filePickerLoading.value = false;
};

const saveRuntimeSettings = async () => {
  settingsSaving.value = true;
  runtimeError.value = '';
  runtimeTestResult.value = null;
  try {
    runtimePathInput.value = runtimePathInput.value.trim();
    const updated = await backend.updateONNXRuntimeSettings(runtimePathInput.value);
    runtimeSettings.value = updated;
  } catch (error: any) {
    runtimeError.value = error?.message || 'Failed to save runtime settings.';
  } finally {
    settingsSaving.value = false;
  }
};

const testRuntimePath = async () => {
  settingsTesting.value = true;
  runtimeError.value = '';
  runtimeTestResult.value = null;
  try {
    runtimeTestResult.value = await backend.testONNXRuntimePath(runtimePathInput.value.trim());
  } catch (error: any) {
    runtimeError.value = error?.message || 'Failed to test runtime path.';
  } finally {
    settingsTesting.value = false;
  }
};

const runtimeStatus = computed(() => {
  const settings = runtimeSettings.value;
  if (!settings) return 'Unknown';
  if (settings.runtimeAvailable) {
    return settings.requiresRestart ? 'Runtime ready (restart to apply new path)' : 'Runtime ready';
  }
  return 'Runtime unavailable';
});

const runtimeStatusClass = computed(() => {
  const settings = runtimeSettings.value;
  if (!settings) return 'chip-muted';
  if (settings.runtimeAvailable) {
    return settings.requiresRestart ? 'chip-warning' : 'chip-success';
  }
  return 'chip-error';
});

const saveMCPConfig = async () => {
  mcpSettingsSaving.value = true;
  mcpSettingsError.value = '';
  try {
    const maxConnections = mcpConfig.value?.maxConnections ?? 32;
    const updated = await backend.updateMCPConfig({
      host: mcpConfigForm.value.host,
      port: mcpConfigForm.value.port,
      autoStart: mcpConfigForm.value.autoStart,
      protocol: mcpConfig.value?.protocol ?? 'http',
      maxConnections
    });
    const normalized: MCPServerConfig = {
      host: updated.host,
      port: updated.port,
      protocol: normalizeProtocol(updated.protocol),
      autoStart: updated.autoStart,
      maxConnections: updated.maxConnections
    };
    mcpConfig.value = normalized;
    mcpConfigForm.value = {
      host: normalized.host,
      port: normalized.port,
      autoStart: normalized.autoStart
    };
  } catch (error: any) {
    mcpSettingsError.value = error?.message || 'Failed to save MCP configuration.';
  } finally {
    mcpSettingsSaving.value = false;
  }
};

// Load projects on mount
onMounted(() => {
  loadProjects();
});
</script>

<template>
  <div class="projects-view">
    <!-- Actions -->
    <div class="actions-bar">
      <div class="actions-left">
        <button @click="openCreateForm" class="btn btn-primary">
          + Create New Project
        </button>
        <button @click="loadProjects" :disabled="isLoading" class="btn btn-secondary">
          {{ isLoading ? 'Refreshing...' : '↻ Refresh' }}
        </button>
        <button @click="openSettings" class="btn btn-secondary">
          ⚙ Settings
        </button>
      </div>

      <!-- View mode toggle -->
      <div class="view-toggle">
        <button
          :class="['toggle-btn', { active: viewMode === 'grid' }]"
          @click="viewMode = 'grid'"
          title="Grid view"
        >
          <span class="toggle-icon">▦</span>
        </button>
        <button
          :class="['toggle-btn', { active: viewMode === 'table' }]"
          @click="viewMode = 'table'"
          title="Table view"
        >
          <span class="toggle-icon">☰</span>
        </button>
      </div>
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

    <!-- Grid View -->
    <div v-else-if="viewMode === 'grid'" class="projects-grid">
      <ProjectCard
        v-for="project in projects"
        :key="project.id"
        :project="project"
        :is-active="currentProject?.id === project.id"
        @go-to-indexing="goToIndexing"
        @edit="openEditForm"
        @delete="confirmDelete"
      />
    </div>

    <!-- Table View -->
    <ProjectTable
      v-else
      :projects="projects"
      :current-project-id="currentProject?.id"
      @go-to-indexing="goToIndexing"
      @edit="openEditForm"
      @delete="confirmDelete"
    />

    <!-- Create/Edit Project Modal -->
    <ProjectFormModal
      v-if="showProjectForm"
      :project="projectToEdit"
      @save="handleProjectSave"
      @cancel="cancelProjectForm"
    />

    <!-- Delete Confirmation Modal -->
    <DeleteConfirmModal
      v-if="showDeleteConfirm && projectToDelete"
      :project="projectToDelete"
      @confirm="deleteProject"
      @cancel="cancelDelete"
    />
  </div>

  <!-- Settings modal -->
  <div v-if="showSettingsModal" class="modal-backdrop">
    <div class="modal settings-modal">
      <div class="modal-header">
        <div>
          <h3>Settings</h3>
        </div>
        <button class="close-btn" @click="closeSettings">✕</button>
      </div>

      <div class="modal-body settings-grid">
        <section class="settings-card">
          <div class="card-header">
            <div>
              <p class="modal-eyebrow">Embeddings runtime</p>
              <h4>ONNX Runtime</h4>
            </div>
            <span :class="['chip', runtimeStatusClass]">
              {{ settingsLoading ? 'Checking...' : runtimeStatus }}
            </span>
          </div>
          <p class="card-description">
            Use the ONNX runtime shared library to run FastEmbed models locally.
          </p>

          <div class="field">
            <label class="field-label" for="onnx-path">Shared library path</label>
            <div class="input-row">
              <input
                id="onnx-path"
                v-model="runtimePathInput"
                type="text"
                placeholder="/path/to/libonnxruntime.so"
                class="text-input"
                :disabled="settingsLoading"
              />
              <button
                type="button"
                class="btn btn-ghost"
                @click="browseRuntimePath"
                :disabled="settingsLoading || filePickerLoading"
              >
                {{ filePickerLoading ? 'Opening...' : 'Browse' }}
              </button>
            </div>
            <div class="help-text">
              Leave empty to use system defaults. Changes apply after app restart.
            </div>
          </div>

          <div class="status-grid vertical">
            <div class="status-row">
              <span class="status-label">Active path</span>
              <span class="status-value">{{ runtimeSettings?.activePath || 'Default/unknown' }}</span>
            </div>
            <div class="status-row">
              <span class="status-label">Saved path</span>
              <span class="status-value">{{ runtimeSettings?.sharedLibraryPath || 'Not set' }}</span>
            </div>
          </div>

          <div v-if="runtimeTestResult" :class="['alert', runtimeTestResult.success ? 'alert-success' : 'alert-error']">
            <strong>{{ runtimeTestResult.success ? 'Test OK' : 'Test Failed' }}:</strong>
            <span>{{ runtimeTestResult.message }}</span>
            <span v-if="runtimeTestResult.error"> ({{ runtimeTestResult.error }})</span>
          </div>
          <div v-if="runtimeError" class="alert alert-error">
            {{ runtimeError }}
          </div>

          <div class="card-actions">
            <button
              type="button"
              class="btn btn-ghost"
              @click="clearRuntimePath"
              :disabled="settingsSaving || settingsTesting"
            >
              Clear path
            </button>
            <button @click="testRuntimePath" :disabled="settingsTesting" class="btn btn-secondary">
              {{ settingsTesting ? 'Testing...' : 'Test path' }}
            </button>
            <div class="spacer"></div>
            <button @click="saveRuntimeSettings" :disabled="settingsSaving" class="btn btn-primary">
              {{ settingsSaving ? 'Saving...' : 'Save runtime' }}
            </button>
          </div>
        </section>

        <section class="settings-card">
          <div class="card-header">
            <div>
              <p class="modal-eyebrow">Model Context Protocol</p>
              <h4>Local server</h4>
            </div>
            <span class="chip chip-muted">HTTP · localhost</span>
          </div>
          <p class="card-description">
            Configure the MCP server that IDE clients connect to. 
          </p>

          <div class="grid two-col tight">
            <div>
              <label class="field-label" for="mcp-host">Host</label>
              <input
                id="mcp-host"
                v-model="mcpConfigForm.host"
                type="text"
                placeholder="127.0.0.1"
                class="text-input input-compact"
                :disabled="mcpSettingsLoading"
              />
            </div>
            <div>
              <label class="field-label" for="mcp-port">Port</label>
              <input
                id="mcp-port"
                v-model.number="mcpConfigForm.port"
                type="number"
                min="1"
                max="65535"
                class="text-input input-compact-sm"
                :disabled="mcpSettingsLoading"
              />
            </div>
          </div>

          <label class="checkbox-inline">
            <input
              type="checkbox"
              v-model="mcpConfigForm.autoStart"
              :disabled="mcpSettingsLoading"
            />
            <span>Auto-start MCP server on app launch</span>
          </label>

          <div v-if="mcpSettingsError" class="alert alert-error">
            {{ mcpSettingsError }}
          </div>

          <div class="card-actions">
            <div class="spacer"></div>
            <button @click="saveMCPConfig" :disabled="mcpSettingsSaving" class="btn btn-primary">
              {{ mcpSettingsSaving ? 'Saving...' : 'Save server' }}
            </button>
          </div>
        </section>
      </div>

      <div class="modal-footer modal-footer-simple">
        <div class="help-text">You can close this dialog after saving sections above.</div>
        <div class="spacer"></div>
        <button @click="closeSettings" class="btn btn-secondary">Close</button>
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

/* Actions bar */
.actions-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.actions-left {
  display: flex;
  gap: 1rem;
}

/* View toggle */
.view-toggle {
  display: flex;
  gap: 0.25rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 0.25rem;
}

.toggle-btn {
  padding: 0.5rem 0.75rem;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: #858585;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.toggle-btn:hover {
  background: #3e3e42;
  color: #d4d4d4;
}

.toggle-btn.active {
  background: #007acc;
  color: white;
}

.toggle-icon {
  font-size: 1.2rem;
  line-height: 1;
}

/* Projects grid */
.projects-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 1.5rem;
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

.alert {
  border-radius: 6px;
  padding: 0.75rem 1rem;
  font-size: 0.95rem;
  border: 1px solid transparent;
}

.alert-success {
  background: rgba(40, 167, 69, 0.12);
  color: #b9f6c5;
  border-color: rgba(40, 167, 69, 0.35);
}

.alert-error {
  background: rgba(220, 53, 69, 0.12);
  color: #ffb3b9;
  border-color: rgba(220, 53, 69, 0.4);
}

/* Settings modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000;
  padding: 1rem;
}

.modal {
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 12px;
  width: min(860px, 96vw);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.45);
  display: flex;
  flex-direction: column;
}

.settings-modal {
  max-height: calc(100vh - 2rem);
  overflow: auto;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.1rem 1.25rem;
  border-bottom: 1px solid #2f2f34;
}

.modal-body {
  padding: 1.1rem 1.25rem 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.settings-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1rem;
}

.modal-footer {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  border-top: 1px solid #2f2f34;
}

.close-btn {
  background: transparent;
  border: none;
  color: #b0b0b0;
  font-size: 1.1rem;
  cursor: pointer;
}

.close-btn:hover {
  color: #ffffff;
}

.settings-card {
  border: 1px solid #2f2f34;
  background: #202124;
  border-radius: 10px;
  padding: 1rem 1.1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.modal-eyebrow {
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-size: 0.75rem;
  margin: 0 0 0.15rem 0;
  color: #7f8ea3;
}

.card-description {
  margin: 0;
  color: #9aa0a6;
}

.field-label {
  font-weight: 600;
  color: #d4d4d4;
  display: block;
  margin-bottom: 0.25rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.text-input {
  width: 100%;
  padding: 0.65rem 0.75rem;
  border-radius: 6px;
  border: 1px solid #3e3e42;
  background: #252526;
  color: #e5e5e5;
}

.divider {
  border-top: 1px solid #2f2f34;
  margin: 1rem 0 0.5rem;
}

.grid.two-col {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 0.75rem;
}

.grid.two-col.tight {
  gap: 0.65rem;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  max-width: 520px;
}

.checkbox-inline {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: #d4d4d4;
}

.text-input:focus {
  outline: none;
  border-color: #007acc;
}

.text-input[type='number'] {
  appearance: textfield;
  -moz-appearance: textfield;
}

.text-input[type='number']::-webkit-outer-spin-button,
.text-input[type='number']::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.input-compact {
  max-width: 260px;
}

.input-compact-sm {
  max-width: 140px;
}

.help-text {
  color: #9aa0a6;
  font-size: 0.9rem;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 0.5rem 0.85rem;
  border: 1px solid #2f2f34;
  background: #1a1c20;
  border-radius: 8px;
  padding: 0.85rem 1rem;
}

.status-grid.compact {
  padding: 0.65rem 0.85rem;
}

.status-grid.vertical {
  grid-template-columns: 1fr;
}

.status-row {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.15rem;
  color: #d4d4d4;
  font-size: 0.95rem;
}

.status-label {
  color: #9aa0a6;
}

.status-value {
  font-weight: 600;
  word-break: break-all;
  overflow-wrap: anywhere;
}

.chip {
  border-radius: 999px;
  padding: 0.35rem 0.75rem;
  border: 1px solid #2f2f34;
  font-size: 0.85rem;
  color: #d4d4d4;
  background: #1c1f24;
}

.chip-success {
  background: rgba(40, 167, 69, 0.12);
  color: #b9f6c5;
  border-color: rgba(40, 167, 69, 0.35);
}

.chip-warning {
  background: rgba(255, 193, 7, 0.14);
  color: #ffe082;
  border-color: rgba(255, 193, 7, 0.4);
}

.chip-error {
  background: rgba(220, 53, 69, 0.12);
  color: #ffb3b9;
  border-color: rgba(220, 53, 69, 0.35);
}

.chip-muted {
  color: #9aa0a6;
}

.btn-ghost {
  background: transparent;
  color: #d4d4d4;
  border: 1px solid #3e3e42;
  padding: 0.6rem 0.9rem;
}

.btn-ghost:hover:not(:disabled) {
  background: #2b2b2f;
  border-color: #4b5563;
}

.input-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 0.25rem;
}

.modal-footer-simple {
  border-top: 1px solid #2f2f34;
}

.spacer {
  flex: 1;
}
</style>
