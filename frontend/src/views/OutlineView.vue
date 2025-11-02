<!--
  File: views/OutlineView.vue
  Purpose: Displays hierarchical file structure outline.
  Author: CodeTextor project
  Notes: Shows AST-based outline with expandable/collapsible nodes.
-->

<script setup lang="ts">
import { ref } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { mockBackend } from '../services/mockBackend';
import type { OutlineNode } from '../types';
import OutlineTreeNode from '../components/OutlineTreeNode.vue';

// Get current project
const { currentProject } = useCurrentProject();

// State
const filePath = ref<string>('');
const depth = ref<number>(2);
const isLoading = ref<boolean>(false);
const outline = ref<OutlineNode[]>([]);
const expandedNodes = ref<Set<string>>(new Set());

/**
 * Fetches outline for the specified file.
 */
const fetchOutline = async () => {
  if (!currentProject.value) {
    alert('Please select a project first');
    return;
  }

  if (!filePath.value.trim()) {
    alert('Please enter a file path');
    return;
  }

  isLoading.value = true;

  try {
    const result = await mockBackend.getOutline({
      projectId: currentProject.value.id,
      path: filePath.value,
      depth: depth.value
    });

    outline.value = result;
    // Expand all nodes by default
    expandAllNodes(result);
  } catch (error) {
    console.error('Failed to fetch outline:', error);
    alert('Failed to fetch outline: ' + (error instanceof Error ? error.message : 'Unknown error'));
  } finally {
    isLoading.value = false;
  }
};

/**
 * Recursively expands all nodes in the outline.
 * @param nodes - Array of outline nodes to expand
 */
const expandAllNodes = (nodes: OutlineNode[]) => {
  nodes.forEach(node => {
    expandedNodes.value.add(node.id);
    if (node.children) {
      expandAllNodes(node.children);
    }
  });
};

/**
 * Toggles expansion state of a node.
 * @param nodeId - ID of the node to toggle
 */
const toggleNode = (nodeId: string) => {
  if (expandedNodes.value.has(nodeId)) {
    expandedNodes.value.delete(nodeId);
  } else {
    expandedNodes.value.add(nodeId);
  }
};

/**
 * Checks if a node is expanded.
 * @param nodeId - ID of the node to check
 * @returns True if expanded
 */
const isExpanded = (nodeId: string): boolean => {
  return expandedNodes.value.has(nodeId);
};

/**
 * Clears the outline display.
 */
const clearOutline = () => {
  outline.value = [];
  expandedNodes.value.clear();
};
</script>

<template>
  <div class="outline-view">
    <!-- Project context info -->
    <div v-if="currentProject" class="project-context section">
      <div class="info-banner">
        <span class="info-icon">📋</span>
        <div class="info-content">
          <strong>Analyzing files in:</strong> {{ currentProject.name }}
          <span class="db-path">(Database: <code>indexes/{{ currentProject.id }}.db</code>)</span>
        </div>
      </div>
    </div>

    <!-- Input form -->
    <div class="outline-form section">
      <div class="form-group">
        <label for="filePath">File Path</label>
        <input
          id="filePath"
          v-model="filePath"
          type="text"
          placeholder="e.g., /src/main.go"
          class="input-text"
          @keyup.enter="fetchOutline"
          :disabled="isLoading"
        />
      </div>

      <div class="form-row">
        <div class="form-group">
          <label for="depth">Depth Level</label>
          <input
            id="depth"
            v-model.number="depth"
            type="number"
            min="1"
            max="5"
            class="input-number"
            :disabled="isLoading"
          />
        </div>
      </div>

      <div class="form-actions">
        <button
          @click="fetchOutline"
          :disabled="isLoading || !filePath.trim()"
          class="btn btn-primary"
        >
          {{ isLoading ? 'Loading...' : 'Get Outline' }}
        </button>
        <button
          v-if="outline.length > 0"
          @click="clearOutline"
          class="btn btn-secondary"
        >
          Clear
        </button>
      </div>
    </div>

    <!-- Outline tree -->
    <div v-if="outline.length > 0" class="outline-tree section">
      <h3>Structure</h3>
      <div class="tree-container">
        <OutlineTreeNode
          v-for="node in outline"
          :key="node.id"
          :node="node"
          :level="0"
          :expanded="isExpanded(node.id)"
          @toggle="toggleNode"
        />
      </div>
    </div>

    <!-- Empty state -->
    <div v-else-if="!isLoading" class="empty-state section">
      <p>Enter a file path above to view its structural outline</p>
    </div>
  </div>
</template>

<style scoped>
.outline-view {
  max-width: 1200px;
  margin: 0 auto;
}

.section {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.section h3 {
  margin: 0 0 1rem 0;
  color: #d4d4d4;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  color: #d4d4d4;
  font-weight: 500;
}

.input-text {
  width: 100%;
  padding: 0.75rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
  font-size: 0.95rem;
}

.input-text:focus {
  outline: none;
  border-color: #007acc;
}

.form-row {
  display: flex;
  gap: 1rem;
}

.input-number {
  width: 120px;
  padding: 0.75rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  color: #d4d4d4;
}

.form-actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 1rem;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: #007acc;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #005a9e;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background: #5a6268;
}

.tree-container {
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 1rem;
  max-height: 600px;
  overflow-y: auto;
}

.empty-state {
  text-align: center;
  padding: 3rem;
  color: #858585;
}

/* Project context banner */
.project-context {
  margin-bottom: 1.5rem;
}

.info-banner {
  display: flex;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  background: #1a3a5a;
  border: 1px solid #007acc;
  border-radius: 4px;
  align-items: center;
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
  margin-right: 0.5rem;
}

.db-path {
  margin-left: 0.5rem;
  font-size: 0.85rem;
  color: #9ec7e0;
}

.db-path code {
  background: #0d2438;
  padding: 0.2rem 0.5rem;
  border-radius: 3px;
  color: #4ec9b0;
  font-family: 'Courier New', monospace;
  font-size: 0.85em;
  border: 1px solid #1a4a6e;
}
</style>
