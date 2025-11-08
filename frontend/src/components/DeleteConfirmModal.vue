<!--
  File: components/DeleteConfirmModal.vue
  Purpose: Confirmation modal for project deletion.
  Author: CodeTextor project
-->

<script setup lang="ts">
import type { Project } from '../types';

interface Props {
  project: Project;
}

interface Emits {
  (e: 'confirm'): void;
  (e: 'cancel'): void;
}

defineProps<Props>();
defineEmits<Emits>();
</script>

<template>
  <div class="modal-overlay" @click="$emit('cancel')">
    <div class="modal-content modal-sm" @click.stop>
      <div class="modal-header">
        <h3>Delete Project?</h3>
        <button class="modal-close" @click="$emit('cancel')">&times;</button>
      </div>

      <div class="modal-body">
        <div class="warning-message">
          <span class="warning-icon">⚠️</span>
          <div>
            <p><strong>Are you sure you want to delete "{{ project.name }}"?</strong></p>
            <p>This will permanently remove:</p>
            <ul>
              <li>Database file: <code>indexes/{{ project.id }}.db</code></li>
              <li>All indexed chunks and embeddings</li>
              <li>Project configuration</li>
            </ul>
            <p><strong>This action cannot be undone.</strong></p>
          </div>
        </div>
      </div>

      <div class="modal-footer">
        <button @click="$emit('cancel')" class="btn btn-secondary">
          Cancel
        </button>
        <button @click="$emit('confirm')" class="btn btn-danger">
          Delete Project
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

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s ease;
  font-weight: 500;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background: #5a6268;
}

.btn-danger {
  background: #dc3545;
  color: white;
}

.btn-danger:hover {
  background: #c82333;
}
</style>
