<!--
  File: views/OutlineView.vue
  Purpose: Displays hierarchical file structure outline.
  Author: CodeTextor project
  Notes: Shows AST-based outline with expandable/collapsible nodes.
-->

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { backend } from '../api/backend';
import type { FilePreview, FileTreeNode as FileTreeNodeType, OutlineNode } from '../types';
import FileTreeNode from '../components/FileTreeNode.vue';
import OutlineContentViewer from '../components/OutlineContentViewer.vue';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { FILE_INDEXED_EVENT } from '../constants/events';

const { currentProject } = useCurrentProject();

const isLoadingTree = ref<boolean>(false);
const treeError = ref<string>('');
const fileTree = ref<FileTreeNodeType[]>([]);
const expandedNodes = ref<Set<string>>(new Set());
const outlineTimestamps = ref<Record<string, number>>({});
const pollingInterval = ref<number | null>(null);
const selectedFilePath = ref<string>('');
const selectedFileVersion = ref<number>(0);
const bumpSelectedFileVersion = (hint?: number) => {
  const next = hint ?? Date.now();
  selectedFileVersion.value = next === selectedFileVersion.value ? next + 1 : next;
};
const selectedNode = ref<OutlineNode | null>(null);
const selectedNodeId = ref<string>('');
const treeColumnWidth = ref<number>(400);
const eventUnsubscribers: Array<() => void> = [];

// Poll interval: check for updates every 5 seconds
const POLL_INTERVAL_MS = 5000;

// Handle outline node selection
const handleNodeClick = (filePath: string, node: OutlineNode) => {
  selectedFilePath.value = filePath;
  selectedNode.value = node;
  selectedNodeId.value = node.id;
  bumpSelectedFileVersion(outlineTimestamps.value[filePath]);
};

const buildFileTree = (previews: FilePreview[]): FileTreeNodeType[] => {
  const roots: FileTreeNodeType[] = [];

  for (const preview of previews) {
    const normalized = preview.relativePath.replace(/\\\\/g, '/');
    const segments = normalized.split('/').filter(Boolean);
    if (segments.length === 0) {
      continue;
    }

    let currentChildren = roots;
    let currentPath = '';

    for (let i = 0; i < segments.length; i++) {
      const segment = segments[i];
      const isFile = i === segments.length - 1;
      const nextPath = currentPath ? `${currentPath}/${segment}` : segment;

      if (isFile) {
        currentChildren.push({
          name: segment,
          path: nextPath,
          isDirectory: false,
          children: [],
          expanded: false,
          outlineStatus: 'idle'
        });
        break;
      }

      let folder = currentChildren.find(node => node.isDirectory && node.name === segment);
      if (!folder) {
        folder = {
          name: segment,
          path: nextPath,
          isDirectory: true,
          children: [],
          expanded: false
        };
        currentChildren.push(folder);
      }

      currentChildren = folder.children;
      currentPath = nextPath;
    }
  }

  const sortTree = (nodes: FileTreeNodeType[]) => {
    nodes.sort((a, b) => {
      if (a.isDirectory !== b.isDirectory) {
        return a.isDirectory ? -1 : 1;
      }
      return a.name.localeCompare(b.name);
    });
    nodes.forEach(child => sortTree(child.children));
  };

  sortTree(roots);
  return roots;
};

const refreshFileTree = async () => {
  if (!currentProject.value) {
    fileTree.value = [];
    return;
  }

  isLoadingTree.value = true;
  treeError.value = '';

  try {
    const previews = await backend.getFilePreviews(
      currentProject.value.id,
      currentProject.value.config
    );
    fileTree.value = buildFileTree(previews);
  } catch (error) {
    treeError.value = error instanceof Error ? error.message : 'Unknown error';
  } finally {
    isLoadingTree.value = false;
  }
};

watch(
  () => currentProject.value?.id,
  async (projectId) => {
    expandedNodes.value.clear();
    fileTree.value = [];
    outlineTimestamps.value = {};
    selectedFilePath.value = '';
    selectedFileVersion.value = 0;
    selectedNode.value = null;
    selectedNodeId.value = '';

    // Stop polling when project changes
    stopPolling();

    if (!projectId) {
      treeError.value = '';
      return;
    }

    // Initial load
    await refreshFileTree();
    await loadInitialTimestamps();

    // Perform immediate check for updates
    await checkForFileTreeChanges();
    await checkForOutlineUpdates();

    // Restart polling for new project
    startPolling();
  },
  { immediate: true }
);

const handleFetchOutline = async (node: FileTreeNodeType) => {
  if (!currentProject.value) {
    node.outlineStatus = 'error';
    node.outlineError = 'Select a project before loading the outline';
    return;
  }

  if (node.outlineStatus === 'ready') {
    return;
  }

  node.outlineStatus = 'loading';
  node.outlineError = '';
  try {
    const result = await backend.getFileOutline(currentProject.value.id, node.path);
    node.outlineNodes = result;
    node.outlineStatus = 'ready';

    // Update timestamp for this file
    if (!outlineTimestamps.value[node.path]) {
      // If we don't have a timestamp yet, fetch it
      try {
        const timestamps = await backend.getOutlineTimestamps(currentProject.value.id);
        if (timestamps[node.path]) {
          outlineTimestamps.value[node.path] = timestamps[node.path];
        }
      } catch (err) {
        console.error('Failed to fetch timestamp:', err);
      }
    }
  } catch (error) {
    node.outlineStatus = 'error';
    node.outlineError = error instanceof Error ? error.message : 'Failed to load outline';
  }
};

const toggleNode = (nodeId: string) => {
  if (expandedNodes.value.has(nodeId)) {
    expandedNodes.value.delete(nodeId);
  } else {
    expandedNodes.value.add(nodeId);
  }
};

const isExpanded = (nodeId: string): boolean => {
  return expandedNodes.value.has(nodeId);
};

// Find a file node by path in the tree (recursively)
const findFileNode = (nodes: FileTreeNodeType[], path: string): FileTreeNodeType | null => {
  for (const node of nodes) {
    if (node.path === path && !node.isDirectory) {
      return node;
    }
    if (node.children.length > 0) {
      const found = findFileNode(node.children, path);
      if (found) return found;
    }
  }
  return null;
};

const findOutlineNodeById = (nodes: OutlineNode[] | undefined, id: string): OutlineNode | null => {
  if (!nodes) {
    return null;
  }
  for (const node of nodes) {
    if (node.id === id) {
      return node;
    }
    const child = findOutlineNodeById(node.children, id);
    if (child) {
      return child;
    }
  }
  return null;
};

// Check for file tree changes (new/deleted files)
const checkForFileTreeChanges = async () => {
  if (!currentProject.value) return;

  try {
    const previews = await backend.getFilePreviews(
      currentProject.value.id,
      currentProject.value.config
    );

    // Build new tree and compare file paths
    const newTree = buildFileTree(previews);

    const getFilePaths = (nodes: FileTreeNodeType[]): Set<string> => {
      const paths = new Set<string>();
      for (const node of nodes) {
        if (!node.isDirectory) {
          paths.add(node.path);
        }
        if (node.children.length > 0) {
          const childPaths = getFilePaths(node.children);
          childPaths.forEach(path => paths.add(path));
        }
      }
      return paths;
    };

    const oldPaths = getFilePaths(fileTree.value);
    const newPaths = getFilePaths(newTree);

    // Check if there are any differences in file paths
    if (oldPaths.size !== newPaths.size) {
      // Different number of files, update tree
      fileTree.value = newTree;
      return;
    }

    // Check if all paths match
    for (const path of oldPaths) {
      if (!newPaths.has(path)) {
        // File was removed or renamed
        fileTree.value = newTree;
        return;
      }
    }

    for (const path of newPaths) {
      if (!oldPaths.has(path)) {
        // New file was added
        fileTree.value = newTree;
        return;
      }
    }
  } catch (error) {
    console.error('Failed to check for file tree changes:', error);
  }
};

// Check for outline updates and refresh if needed
const checkForOutlineUpdates = async () => {
  if (!currentProject.value) return;

  try {
    const newTimestamps = await backend.getOutlineTimestamps(currentProject.value.id);
    let hasChanges = false;
    let selectedFileUpdatedTimestamp: number | null = null;

    // Check each file with a loaded outline
    for (const [path, newTimestamp] of Object.entries(newTimestamps)) {
      const oldTimestamp = outlineTimestamps.value[path];

      // If timestamp changed, refresh the outline for this file
      if (oldTimestamp && newTimestamp > oldTimestamp) {
        hasChanges = true;
        const node = findFileNode(fileTree.value, path);
        if (node && node.outlineStatus === 'ready') {
          // Silently refresh the outline without changing status
          try {
            const result = await backend.getFileOutline(currentProject.value.id, node.path);
            node.outlineNodes = result;
            if (path === selectedFilePath.value) {
              selectedFileUpdatedTimestamp = newTimestamp;
            }
          } catch (error) {
            console.error(`Failed to refresh outline for ${path}:`, error);
          }
        }
      } else if (!oldTimestamp && path === selectedFilePath.value) {
        selectedFileUpdatedTimestamp = newTimestamp;
      }
    }

    // Check for new files with outlines (files we didn't have before)
    for (const path of Object.keys(newTimestamps)) {
      if (!outlineTimestamps.value[path]) {
        hasChanges = true;
        break;
      }
    }

    // If any outline changed, also refresh the file tree to ensure consistency
    if (hasChanges) {
      await checkForFileTreeChanges();
    }

    // Update timestamps
    outlineTimestamps.value = newTimestamps;

    if (selectedFileUpdatedTimestamp !== null) {
      bumpSelectedFileVersion(selectedFileUpdatedTimestamp);
    }
  } catch (error) {
    console.error('Failed to check for outline updates:', error);
  }
};

const refreshOutlineForFile = async (filePath: string, timestamp?: number) => {
  if (!currentProject.value) return;
  const node = findFileNode(fileTree.value, filePath);
  if (!node) {
    if (timestamp) {
      outlineTimestamps.value[filePath] = timestamp;
    }
    return;
  }

  if (node.outlineStatus !== 'ready') {
    if (timestamp) {
      outlineTimestamps.value[filePath] = timestamp;
    }
    node.outlineStatus = 'idle';
    return;
  }

  try {
    const previousSelectedId = selectedFilePath.value === filePath ? selectedNodeId.value : '';
    const result = await backend.getFileOutline(currentProject.value.id, node.path);
    node.outlineNodes = result;
    outlineTimestamps.value[filePath] = timestamp ?? Date.now();

    if (selectedFilePath.value === filePath) {
      if (previousSelectedId) {
        const matching = findOutlineNodeById(result, previousSelectedId);
        if (matching) {
          selectedNode.value = matching;
          selectedNodeId.value = matching.id;
        } else {
          selectedNode.value = null;
          selectedNodeId.value = '';
        }
      }
      bumpSelectedFileVersion(timestamp ?? Date.now());
    }
  } catch (error) {
    console.error(`Failed to refresh outline for ${filePath}:`, error);
  }
};

const handleFileIndexedEvent = (payload: any) => {
  if (!currentProject.value) return;
  const projectId = payload?.projectId;
  const filePath = payload?.filePath;
  if (projectId !== currentProject.value.id || typeof filePath !== 'string') {
    return;
  }
  const timestamp = typeof payload?.timestamp === 'number' ? payload.timestamp : undefined;
  refreshOutlineForFile(filePath, timestamp);
};

// Start polling for outline updates and file tree changes
const startPolling = () => {
  if (pollingInterval.value !== null) return;

  pollingInterval.value = window.setInterval(() => {
    checkForFileTreeChanges();
    checkForOutlineUpdates();
  }, POLL_INTERVAL_MS);
};

// Stop polling
const stopPolling = () => {
  if (pollingInterval.value !== null) {
    window.clearInterval(pollingInterval.value);
    pollingInterval.value = null;
  }
};

// Load initial timestamps
const loadInitialTimestamps = async () => {
  if (!currentProject.value) return;

  try {
    outlineTimestamps.value = await backend.getOutlineTimestamps(currentProject.value.id);
  } catch (error) {
    console.error('Failed to load initial timestamps:', error);
  }
};

// Resize functionality
const initResize = () => {
  let isResizing = false;
  let startX = 0;
  let startWidth = 0;

  const handleMouseMove = (e: MouseEvent) => {
    // Show resize cursor when hovering over resize handle
    const treeColumn = document.querySelector('.tree-column');
    if (treeColumn) {
      const rect = treeColumn.getBoundingClientRect();
      const handleX = rect.right + 8; // Account for gap
      if (Math.abs(e.clientX - handleX) < 8) {
        document.body.style.cursor = 'col-resize';
      } else if (!isResizing) {
        document.body.style.cursor = '';
      }
    }

    // Handle actual resizing
    if (!isResizing) return;

    const delta = e.clientX - startX;
    const newWidth = Math.max(250, Math.min(800, startWidth + delta));
    treeColumnWidth.value = newWidth;
  };

  const handleMouseDown = (e: MouseEvent) => {
    const treeColumn = document.querySelector('.tree-column');
    if (treeColumn) {
      const rect = treeColumn.getBoundingClientRect();
      const handleX = rect.right + 8; // Account for gap

      // Check if click is on the resize handle
      if (Math.abs(e.clientX - handleX) < 8) {
        isResizing = true;
        startX = e.clientX;
        startWidth = treeColumnWidth.value;
        document.body.style.cursor = 'col-resize';
        e.preventDefault();
        e.stopPropagation();
      }
    }
  };

  const handleMouseUp = () => {
    if (isResizing) {
      isResizing = false;
      // Don't reset cursor here, let mousemove handle it
    }
  };

  document.addEventListener('mousemove', handleMouseMove);
  document.addEventListener('mousedown', handleMouseDown);
  document.addEventListener('mouseup', handleMouseUp);

  return () => {
    document.removeEventListener('mousemove', handleMouseMove);
    document.removeEventListener('mousedown', handleMouseDown);
    document.removeEventListener('mouseup', handleMouseUp);
    document.body.style.cursor = '';
  };
};

// Lifecycle hooks
onMounted(() => {
  // Perform initial check immediately when view is opened
  if (currentProject.value) {
    checkForFileTreeChanges();
    checkForOutlineUpdates();
  }
  startPolling();

  // Initialize resize
  const cleanupResize = initResize();
  const offEvent = EventsOn(FILE_INDEXED_EVENT, handleFileIndexedEvent);
  eventUnsubscribers.push(offEvent);

  // Store cleanup in onUnmounted
  onUnmounted(() => {
    stopPolling();
    cleanupResize();
    eventUnsubscribers.forEach(off => off());
    eventUnsubscribers.length = 0;
  });
});
</script>

<template>
  <div class="outline-view">
    <div class="two-column-layout" :style="{ '--tree-width': treeColumnWidth + 'px' }">
      <!-- Left column: File tree -->
      <div class="tree-column section" :style="{ width: treeColumnWidth + 'px' }">
        <div v-if="isLoadingTree" class="status-row">
          Loading file tree...
        </div>
        <div v-else-if="treeError" class="error-row">
          {{ treeError }}
        </div>
        <div v-else-if="fileTree.length === 0" class="empty-state">
          <p>No files found for this project.</p>
        </div>
        <div v-else class="file-tree">
          <FileTreeNode
            v-for="node in fileTree"
            :key="node.path"
            :node="node"
            :level="0"
            :fetchOutline="handleFetchOutline"
            :is-outline-node-expanded="isExpanded"
            :toggle-outline-node="toggleNode"
            :on-node-click="handleNodeClick"
            :selected-node-id="selectedNodeId"
          />
        </div>
      </div>

      <!-- Right column: File content viewer -->
      <div class="content-column section">
        <OutlineContentViewer
          v-if="currentProject"
          :project-id="currentProject.id"
          :file-path="selectedFilePath"
          :file-version="selectedFileVersion"
          :selected-node="selectedNode"
        />
        <div v-else class="empty-state">
          <p>Select a project</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.outline-view {
  width: calc(100% + 4rem);
  height: calc(100% + 4rem);
  margin: -2rem;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.two-column-layout {
  display: flex;
  gap: 1rem;
  flex: 1;
  min-height: 0;
  position: relative;
  padding: 1rem;
}

.section {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.tree-column {
  width: 400px;
  min-width: 250px;
  max-width: 800px;
  padding: 1rem;
  overflow-x: hidden;
  overflow-y: auto;
  flex-shrink: 0;
}

.content-column {
  flex: 1;
  padding: 0;
  overflow: hidden;
  min-width: 0;
}

.file-tree {
  margin-top: 0.5rem;
}

.status-row,
.error-row {
  font-size: 0.95rem;
  color: #9a9da3;
  padding: 1rem;
}

.error-row {
  color: #f48771;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 0.95rem;
  color: #9a9da3;
}

/* Scrollbar styling for tree column */
.tree-column::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.tree-column::-webkit-scrollbar-track {
  background: #1e1e1e;
}

.tree-column::-webkit-scrollbar-thumb {
  background: #424242;
  border-radius: 4px;
}

.tree-column::-webkit-scrollbar-thumb:hover {
  background: #4e4e4e;
}

.tree-column::-webkit-scrollbar-corner {
  background: #1e1e1e;
}
</style>
