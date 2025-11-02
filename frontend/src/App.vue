<!--
  File: App.vue
  Purpose: Main application component with navigation and view routing.
  Author: CodeTextor project
  Notes: Root component that manages view switching and layout.
-->

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, watch } from 'vue';
import { useNavigation } from './composables/useNavigation';
import { useCurrentProject } from './composables/useCurrentProject';
import { mockBackend } from './services/mockBackend';
import ProjectsView from './views/ProjectsView.vue';
import IndexingView from './views/IndexingView.vue';
import SearchView from './views/SearchView.vue';
import OutlineView from './views/OutlineView.vue';
import StatsView from './views/StatsView.vue';
import MCPView from './views/MCPView.vue';
import ProjectSelector from './components/ProjectSelector.vue';
import type { ProjectStats, MCPServerStatus } from './types';
import type { ViewName } from './composables/useNavigation';

// Get navigation composable
const { currentView, navigateTo } = useNavigation();

// Get current project composable
const { currentProject, loadCurrentProject } = useCurrentProject();

// Footer stats
const projectStats = ref<ProjectStats | null>(null);
const mcpStatus = ref<MCPServerStatus | null>(null);
let statsInterval: number | null = null;

// Set initial view
navigateTo('projects');

// Check if navigation is allowed (requires a project for most views)
const isNavigationAllowed = (view: ViewName): boolean => {
  // Projects view is always allowed
  if (view === 'projects') return true;
  // Other views require a project
  return currentProject.value !== null;
};

// Compute which component to display
const currentComponent = computed(() => {
  switch (currentView.value) {
    case 'projects':
      return ProjectsView;
    case 'indexing':
      return IndexingView;
    case 'search':
      return SearchView;
    case 'outline':
      return OutlineView;
    case 'stats':
      return StatsView;
    case 'mcp':
      return MCPView;
    default:
      return ProjectsView;
  }
});

/**
 * Updates footer statistics.
 */
const updateFooterStats = async () => {
  try {
    [projectStats.value, mcpStatus.value] = await Promise.all([
      mockBackend.getProjectStats(),
      mockBackend.getMCPStatus()
    ]);
  } catch (error) {
    console.error('Failed to update footer stats:', error);
  }
};

/**
 * Formats number with K/M suffix.
 * @param num - Number to format
 * @returns Formatted string
 */
const formatNumber = (num: number): string => {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M';
  } else if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'K';
  }
  return num.toString();
};

/**
 * Handles navigation with project requirement check.
 * @param view - Target view name
 */
const handleNavigate = (view: ViewName) => {
  if (isNavigationAllowed(view)) {
    navigateTo(view);
  }
};

// Watch for project changes and redirect if needed
watch(currentProject, (newProject) => {
  // If no project and not on projects view, redirect to projects
  if (!newProject && currentView.value !== 'projects') {
    navigateTo('projects');
  }
});

// Lifecycle
onMounted(async () => {
  // Load current project first
  await loadCurrentProject();

  // If no project, ensure we're on projects view
  if (!currentProject.value && currentView.value !== 'projects') {
    navigateTo('projects');
  }

  // Then start stats updates
  updateFooterStats();
  statsInterval = window.setInterval(updateFooterStats, 5000);
});

onUnmounted(() => {
  if (statsInterval) {
    clearInterval(statsInterval);
  }
});
</script>

<template>
  <div class="app-container">
    <!-- Navigation bar with logo and tabs -->
    <nav class="app-nav">
      <div class="nav-brand">
        <h1 class="app-title">🧩 CodeTextor</h1>
        <span class="app-subtitle">Local-first Code Context Provider</span>
      </div>

      <div class="nav-right">
        <ProjectSelector />

        <div class="nav-tabs">
        <button
          :class="['nav-button', { active: currentView === 'projects' }]"
          @click="handleNavigate('projects')"
        >
          📂 Projects
        </button>
        <button
          :class="['nav-button', { active: currentView === 'indexing', disabled: !currentProject }]"
          :disabled="!currentProject"
          @click="handleNavigate('indexing')"
        >
          ⚡ Indexing
        </button>
        <button
          :class="['nav-button', { active: currentView === 'search', disabled: !currentProject }]"
          :disabled="!currentProject"
          @click="handleNavigate('search')"
        >
          🔍 Search
        </button>
        <button
          :class="['nav-button', { active: currentView === 'outline', disabled: !currentProject }]"
          :disabled="!currentProject"
          @click="handleNavigate('outline')"
        >
          📋 Outline
        </button>
        <button
          :class="['nav-button', { active: currentView === 'stats', disabled: !currentProject }]"
          :disabled="!currentProject"
          @click="handleNavigate('stats')"
        >
          📊 Stats
        </button>
        <button
          :class="['nav-button', { active: currentView === 'mcp', disabled: !currentProject }]"
          :disabled="!currentProject"
          @click="handleNavigate('mcp')"
        >
          🔌 MCP
        </button>
        </div>
      </div>
    </nav>

    <!-- Main content area -->
    <main class="app-main">
      <component :is="currentComponent" />
    </main>

    <!-- Footer -->
    <footer class="app-footer">
      <div class="footer-brand">
        <span class="footer-title">CodeTextor v1.0</span>
        <span v-if="currentProject" class="footer-subtitle">
          Project: {{ currentProject.name }}
        </span>
        <span v-else class="footer-subtitle">No project selected</span>
      </div>
      <div v-if="currentProject && projectStats" class="footer-stats">
        <div class="footer-stat">
          <span class="stat-icon">📁</span>
          <span class="stat-value">{{ formatNumber(projectStats.totalFiles) }}</span>
          <span class="stat-label">Files</span>
        </div>
        <div class="footer-stat">
          <span class="stat-icon">🧩</span>
          <span class="stat-value">{{ formatNumber(projectStats.totalChunks) }}</span>
          <span class="stat-label">Chunks</span>
        </div>
        <div class="footer-stat">
          <span class="stat-icon">🔤</span>
          <span class="stat-value">{{ formatNumber(projectStats.totalSymbols) }}</span>
          <span class="stat-label">Symbols</span>
        </div>
        <div v-if="mcpStatus" class="footer-stat">
          <span :class="['stat-icon', mcpStatus.isRunning ? 'status-active' : 'status-inactive']">●</span>
          <span class="stat-value">MCP</span>
          <span class="stat-label">{{ mcpStatus.isRunning ? 'Running' : 'Stopped' }}</span>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
}

.app-nav {
  display: flex;
  align-items: center;
  gap: 2rem;
  padding: 1rem 2rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-bottom: 2px solid #3e3e42;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
}

.nav-brand {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.app-title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 600;
  color: white;
}

.app-subtitle {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.8);
}

.nav-right {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  margin-left: auto;
}

.nav-tabs {
  display: flex;
  gap: 0.5rem;
}

.nav-button {
  padding: 0.6rem 1.2rem;
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: white;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: all 0.2s ease;
  backdrop-filter: blur(10px);
}

.nav-button:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.4);
  transform: translateY(-1px);
}

.nav-button.active {
  background: white;
  border-color: white;
  color: #667eea;
  font-weight: 600;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.nav-button:disabled,
.nav-button.disabled {
  opacity: 0.4;
  cursor: not-allowed;
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.1);
}

.app-main {
  flex: 1;
  overflow: auto;
  padding: 2rem;
}

.app-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 2rem;
  background: #252526;
  border-top: 1px solid #3e3e42;
  font-size: 0.85rem;
  color: #858585;
}

.footer-brand {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}

.footer-title {
  font-weight: 600;
  color: #d4d4d4;
}

.footer-subtitle {
  font-size: 0.75rem;
  color: #858585;
}

.footer-stats {
  display: flex;
  gap: 2rem;
  align-items: center;
}

.footer-stat {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.stat-icon {
  font-size: 1rem;
}

.stat-icon.status-active {
  color: #28a745;
  animation: pulse-footer 2s ease-in-out infinite;
}

.stat-icon.status-inactive {
  color: #6c757d;
}

@keyframes pulse-footer {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}

.stat-value {
  font-weight: 600;
  color: #d4d4d4;
  font-size: 0.9rem;
}

.stat-label {
  color: #858585;
  font-size: 0.75rem;
}
</style>
