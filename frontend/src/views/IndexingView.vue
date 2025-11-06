<!--
  File: views/IndexingView.vue
  Purpose: View for project indexing with progress tracking.
  Author: CodeTextor project
  Notes: Allows users to monitor indexing progress for a selected project.
-->

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';
import { mockBackend } from '../services/mockBackend';
import type { IndexingProgress } from '../types';

type FilePreview = {
  relativePath: string;
  extension: string;
  size: string;
  hidden: boolean;
};

type ResolvedFile = FilePreview & {
  absolutePath: string;
};

// Get current project and navigation
const { currentProject } = useCurrentProject();
const { navigateTo } = useNavigation();

// Indexing state
const progress = ref<IndexingProgress>({
  totalFiles: 0,
  processedFiles: 0,
  currentFile: '',
  status: 'idle'
});
const indexingEnabled = ref(false);

let progressTimer: ReturnType<typeof setInterval> | null = null;
let awaitingRestart = false;

// Computed properties
const isIndexing = computed(() => progress.value.status === 'indexing');
const progressPercentage = computed(() => {
  if (progress.value.totalFiles === 0) return 0;
  return Math.round((progress.value.processedFiles / progress.value.totalFiles) * 100);
});
const hasCurrentProject = computed(() => currentProject.value !== null);

// ===== Indexing scope configuration =====
const defaultExcludePatterns = ['**/.git', '**/.cache', '**/node_modules'];
const includePaths = ref<string[]>([]);
const excludePaths = ref<string[]>([...defaultExcludePatterns]);
const autoExcludeHidden = ref(true);

const mockFiles = ref<FilePreview[]>([
  { relativePath: 'src/main.go', extension: '.go', size: '8 KB', hidden: false },
  { relativePath: 'backend/api/server.go', extension: '.go', size: '21 KB', hidden: false },
  { relativePath: 'frontend/App.vue', extension: '.vue', size: '14 KB', hidden: false },
  { relativePath: 'frontend/components/ProjectSidebar.vue', extension: '.vue', size: '9 KB', hidden: false },
  { relativePath: 'frontend/components/IndexingProgress.vue', extension: '.vue', size: '6 KB', hidden: false },
  { relativePath: 'tests/indexing/indexer_test.go', extension: '.go', size: '11 KB', hidden: false },
  { relativePath: 'tests/frontend/indexing.spec.ts', extension: '.ts', size: '5 KB', hidden: false },
  { relativePath: 'docs/DEV_GUIDE.md', extension: '.md', size: '32 KB', hidden: false },
  { relativePath: '.github/workflows/ci.yml', extension: '.yml', size: '3 KB', hidden: true },
  { relativePath: 'scripts/setup.sh', extension: '.sh', size: '2 KB', hidden: false },
  { relativePath: 'assets/.cache/index.json', extension: '.json', size: '1 KB', hidden: true },
  { relativePath: 'vendor/lib/parser.c', extension: '.c', size: '45 KB', hidden: false },
  { relativePath: 'frontend/styles/index.scss', extension: '.scss', size: '4 KB', hidden: false },
  { relativePath: 'data/index.sqlite', extension: '.sqlite', size: '1200 KB', hidden: false },
  { relativePath: 'scripts/.history/setup.sh', extension: '.sh', size: '2 KB', hidden: true }
]);

const normalizePattern = (pattern: string) => pattern.replace(/\*\*/g, '').replace(/\*/g, '').trim();
const matchesPattern = (filePath: string, pattern: string) => {
  const normalized = normalizePattern(pattern);
  if (!normalized) return false;
  return filePath.includes(normalized);
};

const isHiddenPath = (filePath: string) => filePath.split('/').some(segment => segment.startsWith('.') && segment.length > 1);
const normalizePathValue = (path: string) => {
  const trimmed = path.trim();
  if (trimmed === '/') {
    return trimmed;
  }
  return trimmed.replace(/\/+$/, '');
};

const projectRoot = computed(() => currentProject.value?.path ?? '/projects/demo');
const resolvedFiles = computed<ResolvedFile[]>(() => {
  const root = projectRoot.value.endsWith('/') ? projectRoot.value.slice(0, -1) : projectRoot.value;
  return mockFiles.value.map(file => ({
    ...file,
    absolutePath: `${root}/${file.relativePath}`
  }));
});

const filteredFiles = computed(() => {
  return resolvedFiles.value.filter(file => {
    const pathCandidates = [file.absolutePath, file.relativePath];

    if (autoExcludeHidden.value && (file.hidden || isHiddenPath(file.relativePath))) {
      return false;
    }

    if (excludePaths.value.some(pattern => pathCandidates.some(candidate => matchesPattern(candidate, pattern)))) {
      return false;
    }

    if (includePaths.value.length === 0) {
      return true;
    }

    return includePaths.value.some(pattern =>
      pathCandidates.some(candidate => matchesPattern(candidate, pattern) || candidate.startsWith(pattern))
    );
  });
});

const availableExtensions = computed(() => {
  const extensions = new Set<string>();
  filteredFiles.value.forEach(file => {
    if (file.extension) {
      extensions.add(file.extension);
    }
  });
  return Array.from(extensions).sort();
});

const selectedExtensions = ref<string[]>([]);
const hasExtensionSelection = computed(() => selectedExtensions.value.length > 0);

const isExtensionSelected = (extension: string) => {
  if (selectedExtensions.value.length === 0) {
    return true;
  }
  return selectedExtensions.value.includes(extension);
};

const displayedFiles = computed(() => {
  if (selectedExtensions.value.length === 0 || selectedExtensions.value.length === availableExtensions.value.length) {
    return filteredFiles.value;
  }

  return filteredFiles.value.filter(file => selectedExtensions.value.includes(file.extension));
});

const previewLimit = 12;
const previewFiles = computed(() => displayedFiles.value.slice(0, previewLimit));
const moreFileCount = computed(() => Math.max(displayedFiles.value.length - previewLimit, 0));

/**
 * Adds a new include path to the indexing configuration.
 */
const addIncludePath = (rawPath: string) => {
  const value = normalizePathValue(rawPath);
  if (!value || includePaths.value.includes(value)) {
    return;
  }
  includePaths.value.push(value);
};

/**
 * Adds a new exclude path pattern.
 */
const addExcludePath = (rawPath: string) => {
  const value = normalizePathValue(rawPath);
  if (!value || excludePaths.value.includes(value)) {
    return;
  }
  excludePaths.value.push(value);
};

const removeIncludePath = (index: number) => {
  includePaths.value.splice(index, 1);
};

const removeExcludePath = (index: number) => {
  excludePaths.value.splice(index, 1);
};

/**
 * Opens a folder picker for include paths.
 */
const browseIncludeFolder = async () => {
  const selected = await mockBackend.selectDirectory({
    prompt: 'Select a folder to include in indexing',
    startPath: projectRoot.value
  });

  if (selected) {
    addIncludePath(selected);
  }
};

/**
 * Opens a folder picker for exclude paths.
 */
const browseExcludeFolder = async () => {
  const selected = await mockBackend.selectDirectory({
    prompt: 'Select a folder to exclude from indexing',
    startPath: projectRoot.value
  });

  if (selected) {
    addExcludePath(selected);
  }
};

/**
 * Formats a path for display relative to the current project.
 */
const formatPathForDisplay = (path: string) => {
  if (path.includes('*')) {
    return path;
  }

  const root = projectRoot.value.replace(/\/+$/, '');

  if (!root) {
    return path;
  }

  const normalized = path.replace(/\/+$/, '');

  if (normalized === root) {
    return '(project root)';
  }

  const prefix = `${root}/`;
  if (normalized.startsWith(prefix)) {
    const relative = normalized.slice(prefix.length);
    return relative ? `./${relative}` : '(project root)';
  }

  return normalized;
};

const stopProgressPolling = () => {
  if (progressTimer) {
    clearInterval(progressTimer);
    progressTimer = null;
  }
};

const triggerIndexingRun = () => {
  if (!currentProject.value) {
    return;
  }

  mockBackend.startIndexing(currentProject.value.path);
};

const safeStopIndexing = async () => {
  try {
    await mockBackend.stopIndexing();
  } catch (error) {
    console.error('Failed to stop indexing:', error);
  }
};

const handleProgressTick = async () => {
  const latest = await mockBackend.getIndexingProgress();
  progress.value = latest;

  if (latest.status === 'error') {
    indexingEnabled.value = false;
    awaitingRestart = false;
    stopProgressPolling();
    return;
  }

  if (latest.status === 'indexing') {
    awaitingRestart = false;
  }

  if (latest.status === 'completed' && indexingEnabled.value && !awaitingRestart) {
    if (currentProject.value) {
      await mockBackend.updateProject(currentProject.value.id, {
        lastIndexed: new Date()
      });
    }

    awaitingRestart = true;
    triggerIndexingRun();
  }

  if (!indexingEnabled.value && latest.status === 'idle') {
    stopProgressPolling();
  }
};

const beginProgressPolling = () => {
  if (progressTimer) {
    clearInterval(progressTimer);
  }

  const tick = () => {
    handleProgressTick().catch(error => {
      console.error('Failed to refresh indexing progress:', error);
    });
  };

  tick();
  progressTimer = setInterval(tick, 500);
};

/**
 * Enables continuous indexing for the current project.
 */
const enableContinuousIndexing = async () => {
  if (!currentProject.value) {
    alert('Please select a project first');
    indexingEnabled.value = false;
    return;
  }

  indexingEnabled.value = true;
  awaitingRestart = false;
  triggerIndexingRun();
  beginProgressPolling();
};

/**
 * Disables continuous indexing and resets progress polling.
 */
const disableContinuousIndexing = async () => {
  indexingEnabled.value = false;
  awaitingRestart = false;
  stopProgressPolling();

  await safeStopIndexing();
  progress.value = await mockBackend.getIndexingProgress();
};

const indexingToggleLabel = computed(() =>
  indexingEnabled.value ? 'Disable continuous indexing' : 'Enable continuous indexing'
);

/** 
 * Toggles selection state for a given file extension.
 */
const toggleExtension = (extension: string) => {
  if (selectedExtensions.value.includes(extension)) {
    selectedExtensions.value = selectedExtensions.value.filter(ext => ext !== extension);
  } else {
    selectedExtensions.value = [...selectedExtensions.value, extension];
  }
};

/**
 * Selects or clears all extensions at once.
 */
const toggleAllExtensions = () => {
  if (hasExtensionSelection.value) {
    selectedExtensions.value = [];
  } else {
    selectedExtensions.value = [...availableExtensions.value];
  }
};

const onToggleChanged = async (event: Event) => {
  const target = event.target as HTMLInputElement | null;
  if (!target) {
    return;
  }

  if (target.checked) {
    await enableContinuousIndexing();
  } else {
    await disableContinuousIndexing();
  }

  // Ensure the checkbox reflects the authoritative state after async work
  target.checked = indexingEnabled.value;
};

onMounted(async () => {
  if (!currentProject.value) {
    return;
  }

  const latest = await mockBackend.getIndexingProgress();
  progress.value = latest;

  if (latest.status === 'indexing') {
    indexingEnabled.value = true;
    beginProgressPolling();
  }
});

onBeforeUnmount(() => {
  stopProgressPolling();
});

watch(currentProject, async (project, previous) => {
  if (project) {
    if (!previous || project.id !== previous.id) {
      includePaths.value = [normalizePathValue(project.path)];
      excludePaths.value = [...defaultExcludePatterns];
      selectedExtensions.value = [];
      autoExcludeHidden.value = true;
      indexingEnabled.value = false;
      awaitingRestart = false;
      stopProgressPolling();
      await safeStopIndexing();
      progress.value = {
        totalFiles: 0,
        processedFiles: 0,
        currentFile: '',
        status: 'idle'
      };
    }
  } else {
    includePaths.value = [];
    excludePaths.value = [...defaultExcludePatterns];
    selectedExtensions.value = [];
    autoExcludeHidden.value = true;
    indexingEnabled.value = false;
    awaitingRestart = false;
    stopProgressPolling();
    await safeStopIndexing();
    progress.value = {
      totalFiles: 0,
      processedFiles: 0,
      currentFile: '',
      status: 'idle'
    };
  }
}, { immediate: true });

watch(availableExtensions, extensions => {
  if (extensions.length === 0) {
    selectedExtensions.value = [];
    return;
  }

  const cleaned = selectedExtensions.value.filter(ext => extensions.includes(ext));
  if (cleaned.length !== selectedExtensions.value.length) {
    selectedExtensions.value = cleaned;
  }
}, { immediate: true });
</script>

<template>
  <div class="indexing-view">
    <div v-if="hasCurrentProject && currentProject">
      <div class="global-progress-card">
        <div class="global-progress-header">
          <div class="global-progress-summary">
            <span class="label">Continuous indexing</span>
            <h3>{{ currentProject.name }}</h3>
            <p class="status-line">Status: {{ progress.status.toUpperCase() }}</p>
          </div>
          <label class="indexing-toggle" :class="{ active: indexingEnabled }">
            <input
              type="checkbox"
              :checked="indexingEnabled"
              @change="onToggleChanged"
              aria-label="Toggle continuous indexing"
            />
            <span class="toggle-track">
              <span class="toggle-thumb"></span>
            </span>
            <span class="toggle-text">{{ indexingToggleLabel }}</span>
          </label>
        </div>
        <div class="progress-bar global">
          <div
            class="progress-fill"
            :style="{ width: `${progressPercentage}%` }"
          ></div>
        </div>
        <div class="global-progress-meta">
          <template v-if="progress.totalFiles > 0">
            <span>{{ progressPercentage }}% complete</span>
            <span>{{ progress.processedFiles }} / {{ progress.totalFiles }} files</span>
          </template>
          <span v-else>Awaiting first indexing run</span>
          <span v-if="isIndexing && progress.currentFile">Current: {{ progress.currentFile }}</span>
        </div>
        <div v-if="progress.status === 'error' && progress.error" class="error-banner">
          <strong>Error:</strong> {{ progress.error }}
        </div>
      </div>

      <div class="section scope-section">
        <div class="section-header">
          <h3>Indexing Scope</h3>
          <p>Select which folders and patterns belong to this project's index.</p>
        </div>
        <div class="path-config">
          <div class="path-column">
            <h4>Include folders</h4>
            <p class="helper-text">Files must live under at least one include rule.</p>
            <div class="path-pill-list">
              <div v-if="includePaths.length === 0" class="empty-pill">No include folders yet</div>
              <div
                v-for="(folder, index) in includePaths"
                :key="`include-${folder}-${index}`"
                class="path-pill"
                :title="folder"
              >
                <span>{{ formatPathForDisplay(folder) }}</span>
                <button
                  type="button"
                  class="pill-remove"
                  @click="removeIncludePath(index)"
                  aria-label="Remove include folder"
                >
                  ×
                </button>
              </div>
            </div>
            <div class="path-actions">
              <button type="button" class="btn btn-primary" @click="browseIncludeFolder">
                Choose folder
              </button>
            </div>
          </div>

          <div class="path-column">
            <h4>Exclude folders</h4>
            <p class="helper-text">Excluded paths are skipped even if they match an include rule.</p>
            <label class="toggle">
              <input type="checkbox" v-model="autoExcludeHidden" />
              <span>Exclude hidden directories (.*)</span>
            </label>
            <div class="path-pill-list">
              <div v-if="excludePaths.length === 0" class="empty-pill">No custom exclusions</div>
              <div
                v-for="(folder, index) in excludePaths"
                :key="`exclude-${folder}-${index}`"
                class="path-pill"
                :title="folder"
              >
                <span>{{ formatPathForDisplay(folder) }}</span>
                <button
                  type="button"
                  class="pill-remove"
                  @click="removeExcludePath(index)"
                  aria-label="Remove exclude folder"
                >
                  ×
                </button>
              </div>
            </div>
            <div class="path-actions">
              <button type="button" class="btn btn-secondary" @click="browseExcludeFolder">
                Choose folder
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="section preview-section">
        <div class="section-header">
          <h3>File Preview</h3>
          <p>Inspect which files are currently part of the index scope.</p>
        </div>

        <div class="extensions">
          <div class="extensions-header">
            <h4>File types</h4>
            <button type="button" class="chip chip-action" @click="toggleAllExtensions">
              {{ hasExtensionSelection ? 'Clear filter' : 'Select all' }}
            </button>
          </div>
          <div class="extension-chips">
            <button
              v-for="extension in availableExtensions"
              :key="extension"
              type="button"
              class="chip"
              :class="{ active: isExtensionSelected(extension) }"
              @click="toggleExtension(extension)"
            >
              {{ extension }}
            </button>
            <span v-if="availableExtensions.length === 0" class="helper-text">
              No files match the current configuration yet.
            </span>
          </div>
        </div>

        <div class="file-preview">
          <table v-if="previewFiles.length > 0" class="file-table">
            <thead>
              <tr>
                <th>Relative path</th>
                <th>Extension</th>
                <th>Size</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="file in previewFiles" :key="file.absolutePath">
                <td>
                  <div class="file-name">{{ file.relativePath }}</div>
                  <div class="file-path">{{ file.absolutePath }}</div>
                </td>
                <td>{{ file.extension || '—' }}</td>
                <td>{{ file.size }}</td>
              </tr>
            </tbody>
          </table>

          <div v-else class="empty-state">
            <div class="empty-icon">🗂️</div>
            <p>No files are currently selected for indexing.</p>
          </div>

          <div v-if="moreFileCount > 0" class="more-files">
            +{{ moreFileCount }} additional files match your filters
          </div>
        </div>
      </div>
    </div>

    <div v-else class="section no-project-state">
      <div class="empty-icon">📁</div>
      <h3>No Project Selected</h3>
      <p>Go to the Projects page to create a new project or select an existing one.</p>
      <button @click="navigateTo('projects')" class="btn btn-primary" style="margin-top: 1rem">
        Go to Projects
      </button>
    </div>
  </div>
</template>

<style scoped>
.indexing-view {
  max-width: 900px;
  margin: 0 auto;
}

.global-progress-card {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.global-progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1rem;
}

.global-progress-summary .label {
  display: block;
  text-transform: uppercase;
  color: #858585;
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  margin-bottom: 0.35rem;
}

.global-progress-summary h3 {
  margin: 0;
  color: #d4d4d4;
  font-size: 1.4rem;
}

.global-progress-summary .status-line {
  margin: 0.35rem 0 0 0;
  color: #858585;
  font-size: 0.85rem;
}

.indexing-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.4rem 0.7rem 0.4rem 0.4rem;
  border-radius: 999px;
  background: transparent;
  border: none;
  color: #d4d4d4;
  cursor: pointer;
  transition: color 0.2s ease;
}

.indexing-toggle:hover {
  color: #ffffff;
}

.indexing-toggle input {
  display: none;
}

.toggle-track {
  position: relative;
  width: 46px;
  height: 24px;
  background: #3e3e42;
  border-radius: 12px;
  transition: background 0.2s ease;
}

.toggle-thumb {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #d4d4d4;
  transition: transform 0.2s ease, background 0.2s ease;
}

.indexing-toggle.active .toggle-track {
  background: #007acc;
}

.indexing-toggle.active .toggle-thumb {
  transform: translateX(22px);
  background: #fff;
}

.toggle-text {
  font-size: 0.9rem;
  font-weight: 500;
}

.progress-bar.global {
  height: 12px;
}

.global-progress-meta {
  margin-top: 0.75rem;
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  color: #858585;
  font-size: 0.85rem;
}

.error-banner {
  margin-top: 1rem;
  padding: 0.9rem 1rem;
  background: #5a1a1a;
  border: 1px solid #dc3545;
  border-radius: 6px;
  color: #ff9a9a;
  font-size: 0.9rem;
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
  font-size: 1.1rem;
  color: #d4d4d4;
}

.section-header {
  margin-bottom: 1.25rem;
}

.section-header p {
  margin: 0.25rem 0 0 0;
  color: #858585;
  font-size: 0.9rem;
}

.scope-section .path-config {
  display: flex;
  flex-wrap: wrap;
  gap: 1.5rem;
}

.path-column {
  flex: 1;
  min-width: 260px;
}

.path-column h4 {
  margin: 0;
  color: #d4d4d4;
  font-size: 1rem;
}

.helper-text {
  margin: 0.35rem 0 0.75rem 0;
  color: #858585;
  font-size: 0.85rem;
}

.path-pill-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.path-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 999px;
  padding: 0.4rem 0.9rem;
  font-size: 0.85rem;
  color: #d4d4d4;
}

.pill-remove {
  background: none;
  border: none;
  color: #858585;
  font-size: 1rem;
  cursor: pointer;
  padding: 0;
  line-height: 1;
  transition: color 0.2s ease;
}

.pill-remove:hover {
  color: #ff6b6b;
}

.empty-pill {
  color: #858585;
  font-size: 0.85rem;
  border: 1px dashed #3e3e42;
  border-radius: 999px;
  padding: 0.4rem 0.9rem;
}

.path-actions {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
  align-items: center;
}

.toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
  color: #d4d4d4;
  font-size: 0.9rem;
}

.extensions {
  margin-bottom: 1.5rem;
}

.extensions-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.extensions-header h4 {
  margin: 0;
  color: #d4d4d4;
  font-size: 1rem;
}

.extension-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.chip {
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  color: #d4d4d4;
  padding: 0.35rem 0.85rem;
  border-radius: 999px;
  font-size: 0.85rem;
  cursor: pointer;
  transition: all 0.2s ease;
}

.chip:hover {
  border-color: #4ec9b0;
}

.chip.active {
  background: #007acc;
  border-color: #007acc;
  color: white;
}

.chip-action {
  background: transparent;
  border-style: dashed;
  color: #858585;
}

.file-preview {
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1rem;
}

.file-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}

.file-table th,
.file-table td {
  padding: 0.75rem;
  border-bottom: 1px solid #3e3e42;
  color: #d4d4d4;
  text-align: left;
}

.file-table th {
  text-transform: uppercase;
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  color: #858585;
}

.file-name {
  font-weight: 600;
  color: #d4d4d4;
}

.file-path {
  color: #6f6f6f;
  font-size: 0.75rem;
  margin-top: 0.25rem;
  word-break: break-all;
}

.empty-state {
  text-align: center;
  color: #858585;
  padding: 2rem 1rem;
}

.more-files {
  margin-top: 0.75rem;
  color: #858585;
  font-size: 0.85rem;
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

.progress-bar {
  height: 24px;
  background: #1e1e1e;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid #3e3e42;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #007acc 0%, #00a8ff 100%);
  transition: width 0.3s ease;
}

.no-project-state {
  text-align: center;
  padding: 2rem;
  color: #858585;
}

.empty-icon {
  font-size: 4rem;
  margin-bottom: 1rem;
}
</style>
