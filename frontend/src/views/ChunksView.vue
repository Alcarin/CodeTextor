<!--
  File: views/ChunksView.vue
  Purpose: Displays semantic chunks extracted from indexed files in a tree structure.
  Author: CodeTextor project
  Notes: Shows chunks as expandable nodes with metadata, similar to OutlineView.
-->

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { backend } from '../api/backend';
import type { FilePreview, FileTreeNode as FileTreeNodeType, Chunk } from '../types';
import FileTreeNode from '../components/FileTreeNode.vue';
import ChunkContentViewer from '../components/ChunkContentViewer.vue';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { FILE_INDEXED_EVENT } from '../constants/events';

const { currentProject } = useCurrentProject();

const isLoadingTree = ref<boolean>(false);
const treeError = ref<string>('');
const fileTree = ref<FileTreeNodeType[]>([]);
const expandedNodes = ref<Set<string>>(new Set());
const selectedFilePath = ref<string>('');
const selectedChunk = ref<Chunk | null>(null);
const selectedChunkId = ref<string>('');
const treeColumnWidth = ref<number>(400);
const eventUnsubscribers: Array<() => void> = [];

// Handle chunk node selection
const handleChunkClick = (filePath: string, chunk: Chunk) => {
  selectedFilePath.value = filePath;
  selectedChunk.value = chunk;
  selectedChunkId.value = chunk.id;
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
    treeError.value = error instanceof Error ? error.message : 'Errore sconosciuto';
  } finally {
    isLoadingTree.value = false;
  }
};

watch(
  () => currentProject.value?.id,
  async (projectId) => {
    expandedNodes.value.clear();
    fileTree.value = [];
    selectedFilePath.value = '';
    selectedChunk.value = null;
    selectedChunkId.value = '';

    if (!projectId) {
      treeError.value = '';
      return;
    }

    await refreshFileTree();
  },
  { immediate: true }
);

const handleFetchChunks = async (node: FileTreeNodeType) => {
  if (!currentProject.value) {
    node.outlineStatus = 'error';
    node.outlineError = 'Select a project before loading chunks';
    return;
  }

  if (node.outlineStatus === 'ready') {
    return;
  }

  node.outlineStatus = 'loading';
  node.outlineError = '';
  try {
    const chunks = await backend.getFileChunks(currentProject.value.id, node.path);
    // Store chunks as a custom property
    node.chunks = chunks;
    node.outlineStatus = 'ready';
  } catch (error) {
    node.outlineStatus = 'error';
    node.outlineError = error instanceof Error ? error.message : 'Failed to load chunks';
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

const pickMatchingChunk = (chunks: Chunk[], previous: Chunk | null | undefined) => {
  if (!previous) {
    return undefined;
  }
  const byId = chunks.find(chunk => chunk.id === previous.id);
  if (byId) {
    return byId;
  }
  return chunks.find(chunk => chunk.lineStart === previous.lineStart);
};

const refreshChunksForFile = async (filePath: string) => {
  if (!currentProject.value) return;
  const node = findFileNode(fileTree.value, filePath);
  if (!node || node.outlineStatus !== 'ready') {
    return;
  }

  try {
    const previousChunk = selectedFilePath.value === filePath ? selectedChunk.value : null;
    const chunks = await backend.getFileChunks(currentProject.value.id, node.path);
    node.chunks = chunks;
    node.outlineStatus = 'ready';

    if (selectedFilePath.value === filePath) {
      const next = pickMatchingChunk(chunks, previousChunk) ?? (chunks.length > 0 ? chunks[0] : null);
      selectedChunk.value = next;
      selectedChunkId.value = next?.id ?? '';
    }
  } catch (error) {
    console.error(`Failed to refresh chunks for ${filePath}:`, error);
  }
};

const handleFileIndexedEvent = (payload: any) => {
  if (!currentProject.value) return;
  const projectId = payload?.projectId;
  const filePath = payload?.filePath;
  if (projectId !== currentProject.value.id || typeof filePath !== 'string') {
    return;
  }
  refreshChunksForFile(filePath);
};

// Resize functionality
const initResize = () => {
  let isResizing = false;
  let startX = 0;
  let startWidth = 0;

  const handleMouseMove = (e: MouseEvent) => {
    const treeColumn = document.querySelector('.tree-column');
    if (treeColumn) {
      const rect = treeColumn.getBoundingClientRect();
      const handleX = rect.right + 8;
      if (Math.abs(e.clientX - handleX) < 8) {
        document.body.style.cursor = 'col-resize';
      } else if (!isResizing) {
        document.body.style.cursor = '';
      }
    }

    if (!isResizing) return;

    const delta = e.clientX - startX;
    const newWidth = Math.max(250, Math.min(800, startWidth + delta));
    treeColumnWidth.value = newWidth;
  };

  const handleMouseDown = (e: MouseEvent) => {
    const treeColumn = document.querySelector('.tree-column');
    if (treeColumn) {
      const rect = treeColumn.getBoundingClientRect();
      const handleX = rect.right + 8;

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

onMounted(() => {
  const cleanupResize = initResize();
  const offEvent = EventsOn(FILE_INDEXED_EVENT, handleFileIndexedEvent);
  eventUnsubscribers.push(offEvent);

  onUnmounted(() => {
    cleanupResize();
    eventUnsubscribers.forEach(off => off());
    eventUnsubscribers.length = 0;
  });
});
</script>

<template>
  <div class="chunks-view">
    <div class="two-column-layout" :style="{ '--tree-width': treeColumnWidth + 'px' }">
      <!-- Left column: File tree with chunks -->
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
            :fetchOutline="handleFetchChunks"
            :is-outline-node-expanded="isExpanded"
            :toggle-outline-node="toggleNode"
            :on-chunk-click="handleChunkClick"
            :selected-node-id="selectedChunkId"
            :chunks-mode="true"
          />
        </div>
      </div>

      <!-- Right column: File content viewer -->
      <div class="content-column section">
        <ChunkContentViewer
          v-if="currentProject"
          :project-id="currentProject.id"
          :file-path="selectedFilePath"
          :selected-chunk="selectedChunk"
        />
        <div v-else class="empty-state">
          <p>Select a project</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chunks-view {
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
  overflow-x: auto;
  overflow-y: auto;
  flex-shrink: 0;
  text-align: left;
}

.content-column {
  flex: 1;
  padding: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
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
