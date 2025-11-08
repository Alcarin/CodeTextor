<!--
  File: components/ProjectFormModal.vue
  Purpose: Modal form for creating and editing projects.
  Author: CodeTextor project
-->

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { backend } from '../api/backend';
import type { Project } from '../types';

interface Props {
  project?: Project | null;
}

interface Emits {
  (e: 'save', project: Project): void;
  (e: 'cancel'): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

// Form state
const projectName = ref<string>('');
const projectSlug = ref<string>('');
const projectDescription = ref<string>('');
const projectRootFolder = ref<string>('');
const isSaving = ref<boolean>(false);

const isEditMode = computed(() => props.project !== null && props.project !== undefined);

/**
 * Generates a URL-safe slug from a string.
 */
const generateSlug = (text: string): string => {
  return text
    .toLowerCase()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9-]+/g, '')
    .replace(/-+/g, '-')
    .replace(/^-+|-+$/g, '');
};

/**
 * Auto-update slug when name changes (only in create mode).
 */
watch(projectName, (newName) => {
  if (!isEditMode.value) {
    projectSlug.value = generateSlug(newName);
  }
});

/**
 * Initialize form with project data if editing.
 */
watch(() => props.project, (newProject) => {
  if (newProject) {
    projectName.value = newProject.name;
    projectSlug.value = newProject.id || '';
    projectDescription.value = newProject.description || '';
    projectRootFolder.value = newProject.config?.rootPath || '';
  } else {
    projectName.value = '';
    projectSlug.value = '';
    projectDescription.value = '';
    projectRootFolder.value = '';
  }
}, { immediate: true });

/**
 * Opens directory picker for project root.
 */
const chooseProjectRoot = async () => {
  const defaultPath = projectRootFolder.value || '/';
  try {
    const selected = await backend.selectDirectory('Select project root folder', defaultPath);
    if (selected) {
      projectRootFolder.value = selected;
    }
  } catch (error) {
    console.error('Failed to select project root folder:', error);
    alert('Failed to select project root folder: ' + (error instanceof Error ? error.message : 'Unknown error'));
  }
};

/**
 * Handles form submission.
 */
const handleSubmit = async () => {
  if (!projectName.value.trim()) {
    alert('Please enter a project name');
    return;
  }

  const rootPath = projectRootFolder.value.trim();
  if (!rootPath) {
    alert('Please select a project root folder');
    return;
  }

  isSaving.value = true;

  try {
    let savedProject: Project;

    if (isEditMode.value && props.project) {
      // Update existing project
      const updatedMeta = await backend.updateProject(
        props.project.id,
        projectName.value,
        projectDescription.value || ''
      );

      savedProject = await backend.updateProjectConfig(props.project.id, {
        ...updatedMeta.config,
        rootPath
      });
    } else {
      // Create new project
      savedProject = await backend.createProject(
        projectName.value,
        projectDescription.value || '',
        projectSlug.value || '',
        rootPath
      );
    }

    emit('save', savedProject);
  } catch (error) {
    console.error('Failed to save project:', error);
    alert('Failed to save project: ' + (error instanceof Error ? error.message : 'Unknown error'));
  } finally {
    isSaving.value = false;
  }
};

/**
 * Handles cancel action.
 */
const handleCancel = () => {
  emit('cancel');
};
</script>

<template>
  <div class="modal-overlay" @click="handleCancel">
    <div class="modal-content" @click.stop>
      <div class="modal-header">
        <h3>{{ isEditMode ? 'Edit Project' : 'Create New Project' }}</h3>
        <button class="modal-close" @click="handleCancel">&times;</button>
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
            :disabled="isSaving"
          />
        </div>

        <div class="form-group">
          <label for="project-slug">{{ isEditMode ? 'ID (immutable)' : 'ID / Slug' }}</label>
          <input
            id="project-slug"
            v-model="projectSlug"
            type="text"
            :placeholder="isEditMode ? '' : 'Auto-generated from project name'"
            class="form-input"
            :disabled="isSaving || isEditMode"
            :title="isEditMode ? 'The ID is immutable and cannot be changed after creation' : 'URL-safe identifier for database filename. Auto-generated from project name, but you can customize it.'"
          />
          <small class="form-help">
            {{ isEditMode
              ? `Used for database filename: project-${project?.id}.db`
              : 'Used for database filename: project-{id}.db. Auto-generated from project name, but you can edit it before saving.'
            }}
          </small>
        </div>

        <div class="form-group">
          <label for="project-root">Project Root Folder *</label>
          <div class="root-selector">
            <input
              id="project-root"
              v-model="projectRootFolder"
              type="text"
              class="form-input"
              placeholder="/path/to/project"
              readonly
            />
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click.stop="chooseProjectRoot"
            >
              Browse
            </button>
          </div>
          <small class="form-help">
            This directory becomes the project's root. Include paths are stored relative to it.
          </small>
        </div>

        <div class="form-group">
          <label for="project-description">Description (optional)</label>
          <textarea
            id="project-description"
            v-model="projectDescription"
            placeholder="A brief description of this project..."
            class="form-textarea"
            rows="3"
            :disabled="isSaving"
          ></textarea>
        </div>
      </div>

      <div class="modal-footer">
        <button
          @click="handleCancel"
          :disabled="isSaving"
          class="btn btn-secondary"
        >
          Cancel
        </button>
        <button
          @click="handleSubmit"
          :disabled="!projectName || isSaving"
          class="btn btn-success"
        >
          {{ isSaving ? 'Saving...' : (isEditMode ? 'Save Changes' : 'Create Project') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
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

.form-help {
  display: block;
  margin-top: 0.5rem;
  color: #858585;
  font-size: 0.85rem;
  font-style: italic;
}

.root-selector {
  display: flex;
  gap: 0.5rem;
}

.root-selector .form-input {
  flex: 1;
}

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
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover:not(:disabled) {
  background: #5a6268;
}

.btn-success {
  background: #28a745;
  color: white;
}

.btn-success:hover:not(:disabled) {
  background: #218838;
}
</style>
