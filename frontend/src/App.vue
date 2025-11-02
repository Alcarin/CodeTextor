<!--
  File: App.vue
  Purpose: Main application component with navigation and view routing.
  Author: CodeTextor project
  Notes: Root component that manages view switching and layout.
-->

<script setup lang="ts">
import { computed } from 'vue';
import { useNavigation } from './composables/useNavigation';
import IndexingView from './views/IndexingView.vue';
import SearchView from './views/SearchView.vue';
import OutlineView from './views/OutlineView.vue';
import StatsView from './views/StatsView.vue';

// Get navigation composable
const { currentView, navigateTo } = useNavigation();

// Compute which component to display
const currentComponent = computed(() => {
  switch (currentView.value) {
    case 'indexing':
      return IndexingView;
    case 'search':
      return SearchView;
    case 'outline':
      return OutlineView;
    case 'stats':
      return StatsView;
    default:
      return IndexingView;
  }
});
</script>

<template>
  <div class="app-container">
    <!-- Header with navigation -->
    <header class="app-header">
      <h1 class="app-title">🧩 CodeTextor</h1>
      <p class="app-subtitle">Local-first Code Context Provider</p>
    </header>

    <!-- Navigation tabs -->
    <nav class="app-nav">
      <button
        :class="['nav-button', { active: currentView === 'indexing' }]"
        @click="navigateTo('indexing')"
      >
        📂 Indexing
      </button>
      <button
        :class="['nav-button', { active: currentView === 'search' }]"
        @click="navigateTo('search')"
      >
        🔍 Search
      </button>
      <button
        :class="['nav-button', { active: currentView === 'outline' }]"
        @click="navigateTo('outline')"
      >
        📋 Outline
      </button>
      <button
        :class="['nav-button', { active: currentView === 'stats' }]"
        @click="navigateTo('stats')"
      >
        📊 Stats
      </button>
    </nav>

    <!-- Main content area -->
    <main class="app-main">
      <component :is="currentComponent" />
    </main>

    <!-- Footer -->
    <footer class="app-footer">
      <span>CodeTextor v1.0 - Local-first semantic code search</span>
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

.app-header {
  padding: 1.5rem 2rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
}

.app-title {
  margin: 0;
  font-size: 2rem;
  font-weight: 600;
}

.app-subtitle {
  margin: 0.25rem 0 0 0;
  font-size: 0.9rem;
  opacity: 0.9;
}

.app-nav {
  display: flex;
  gap: 0.5rem;
  padding: 1rem 2rem;
  background: #252526;
  border-bottom: 1px solid #3e3e42;
}

.nav-button {
  padding: 0.75rem 1.5rem;
  background: transparent;
  border: 1px solid #3e3e42;
  color: #d4d4d4;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.95rem;
  transition: all 0.2s ease;
}

.nav-button:hover {
  background: #2d2d30;
  border-color: #007acc;
}

.nav-button.active {
  background: #007acc;
  border-color: #007acc;
  color: white;
  font-weight: 500;
}

.app-main {
  flex: 1;
  overflow: auto;
  padding: 2rem;
}

.app-footer {
  padding: 1rem 2rem;
  background: #252526;
  border-top: 1px solid #3e3e42;
  font-size: 0.85rem;
  text-align: center;
  color: #858585;
}
</style>
