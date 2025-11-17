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
import { backend } from './api/backend';
import ProjectsView from './views/ProjectsView.vue';
import IndexingView from './views/IndexingView.vue';
import SearchView from './views/SearchView.vue';
import OutlineView from './views/OutlineView.vue';
import ChunksView from './views/ChunksView.vue';
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

// Mobile menu state
const showMobileMenu = ref(false);

/**
 * Toggles mobile menu visibility.
 */
const toggleMobileMenu = () => {
  showMobileMenu.value = !showMobileMenu.value;
};

/**
 * Closes mobile menu.
 */
const closeMobileMenu = () => {
  showMobileMenu.value = false;
};

/**
 * Handles navigation and closes mobile menu.
 */
const handleMobileNavigate = (view: ViewName) => {
  handleNavigate(view);
  closeMobileMenu();
};

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
    case 'chunks':
      return ChunksView;
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
    // Get cumulative stats for all projects
    projectStats.value = await backend.getAllProjectsStats();
    // TODO: Implement MCP status retrieval when available
    mcpStatus.value = {
      isRunning: false,
      uptime: 0,
      activeConnections: 0,
      totalRequests: 0,
      averageResponseTime: 0
    };
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
    <!-- Navigation bar with tabs and project selector -->
    <nav class="app-nav">
      <!-- Left side: Project selector as H1 -->
      <div class="nav-left">
        <ProjectSelector />
      </div>

      <!-- Right side: Navigation tabs (only shown when a project is selected) -->
      <div v-if="currentProject" class="nav-tabs">
        <button
          :class="['nav-tab', { active: currentView === 'indexing' }]"
          @click="handleNavigate('indexing')"
        >
          ⚡ Indexing
        </button>
        <button
          :class="['nav-tab', { active: currentView === 'search' }]"
          @click="handleNavigate('search')"
        >
          🔍 Search
        </button>
        <button
          :class="['nav-tab', { active: currentView === 'outline' }]"
          @click="handleNavigate('outline')"
        >
          📋 Outline
        </button>
        <button
          :class="['nav-tab', { active: currentView === 'chunks' }]"
          @click="handleNavigate('chunks')"
        >
          🧩 Chunks
        </button>
        <button
          :class="['nav-tab', { active: currentView === 'stats' }]"
          @click="handleNavigate('stats')"
        >
          📊 Stats
        </button>
        <button
          :class="['nav-tab', { active: currentView === 'mcp' }]"
          @click="handleNavigate('mcp')"
        >
          🔌 MCP
        </button>
      </div>

      <!-- Hamburger menu button (mobile only) -->
      <button v-if="currentProject" class="hamburger-button" @click="toggleMobileMenu">
        <span class="hamburger-icon">☰</span>
      </button>

      <!-- Mobile menu overlay -->
      <div v-if="showMobileMenu" class="mobile-menu-overlay" @click="closeMobileMenu">
        <div class="mobile-menu" @click.stop>
          <div class="mobile-menu-header">
            <h3>Navigation</h3>
            <button class="close-button" @click="closeMobileMenu">✕</button>
          </div>
          <div class="mobile-menu-items">
            <button
              :class="['mobile-menu-item', { active: currentView === 'indexing' }]"
              @click="handleMobileNavigate('indexing')"
            >
              <span class="menu-icon">⚡</span>
              <span>Indexing</span>
            </button>
            <button
              :class="['mobile-menu-item', { active: currentView === 'search' }]"
              @click="handleMobileNavigate('search')"
            >
              <span class="menu-icon">🔍</span>
              <span>Search</span>
            </button>
            <button
              :class="['mobile-menu-item', { active: currentView === 'outline' }]"
              @click="handleMobileNavigate('outline')"
            >
              <span class="menu-icon">📋</span>
              <span>Outline</span>
            </button>
            <button
              :class="['mobile-menu-item', { active: currentView === 'chunks' }]"
              @click="handleMobileNavigate('chunks')"
            >
              <span class="menu-icon">🧩</span>
              <span>Chunks</span>
            </button>
            <button
              :class="['mobile-menu-item', { active: currentView === 'stats' }]"
              @click="handleMobileNavigate('stats')"
            >
              <span class="menu-icon">📊</span>
              <span>Stats</span>
            </button>
            <button
              :class="['mobile-menu-item', { active: currentView === 'mcp' }]"
              @click="handleMobileNavigate('mcp')"
            >
              <span class="menu-icon">🔌</span>
              <span>MCP</span>
            </button>
          </div>
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
      <div v-if="projectStats" class="footer-stats">
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
  align-items: flex-end;
  justify-content: space-between;
  padding: 0;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
  min-height: 70px;
}

.nav-tabs {
  display: flex;
  gap: 0.25rem;
  align-items: flex-end;
  margin-bottom: -1px;
}

.nav-tab {
  padding: 0.75rem 1.5rem;
  background: rgba(255, 255, 255, 0.15);
  border: none;
  border-top-left-radius: 8px;
  border-top-right-radius: 8px;
  color: rgba(255, 255, 255, 0.95);
  cursor: pointer;
  font-size: 0.9rem;
  transition: all 0.2s ease;
  font-weight: 500;
  white-space: nowrap;
  border-bottom: 2px solid transparent;
  position: relative;
}

.nav-tab:hover {
  background: rgba(255, 255, 255, 0.25);
  color: white;
}

.nav-tab.active {
  background: #1e1e1e;
  color: #d4d4d4;
  font-weight: 600;
  border-bottom: 2px solid #1e1e1e;
}

.nav-left {
  display: flex;
  align-items: center;
  padding-bottom: 0.5rem;
}

/* Hamburger menu button - hidden by default, shown on mobile */
.hamburger-button {
  display: none;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  background: rgba(255, 255, 255, 0.15);
  border: none;
  border-radius: 6px;
  color: white;
  cursor: pointer;
  font-size: 1.5rem;
  margin-bottom: 0.5rem;
  margin-left: 0.5rem;
  transition: all 0.2s ease;
}

.hamburger-button:hover {
  background: rgba(255, 255, 255, 0.25);
}

.hamburger-icon {
  display: block;
  line-height: 1;
}

/* Mobile menu overlay */
.mobile-menu-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 9999;
  display: flex;
  align-items: flex-start;
  padding-top: 70px;
}

.mobile-menu {
  width: 280px;
  max-width: 80vw;
  background: #2d2d30;
  border-radius: 8px;
  margin: 1rem;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  overflow: hidden;
}

.mobile-menu-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
}

.mobile-menu-header h3 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
}

.close-button {
  background: none;
  border: none;
  color: white;
  font-size: 1.5rem;
  cursor: pointer;
  padding: 0;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: background 0.2s ease;
}

.close-button:hover {
  background: rgba(255, 255, 255, 0.2);
}

.mobile-menu-items {
  padding: 0.5rem;
}

.mobile-menu-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem 1.5rem;
  background: transparent;
  border: none;
  border-radius: 6px;
  color: #d4d4d4;
  cursor: pointer;
  font-size: 1rem;
  font-weight: 500;
  text-align: left;
  transition: all 0.2s ease;
  margin-bottom: 0.25rem;
}

.mobile-menu-item:hover {
  background: #3e3e42;
}

.mobile-menu-item.active {
  background: #667eea;
  color: white;
}

.menu-icon {
  font-size: 1.2rem;
}

/* Responsive breakpoint */
@media (max-width: 1023px) {
  .hamburger-button {
    display: flex;
  }

  .nav-tabs {
    display: none;
  }
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
