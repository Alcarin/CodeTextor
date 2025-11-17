<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';
import { backend } from '../api/backend';
import type { IndexingProgress, FilePreview, Project } from '../types';

// Get current project and navigation
const { currentProject, refreshCurrentProject } = useCurrentProject();
const { navigateTo } = useNavigation();

// Indexing state
const progress = ref<IndexingProgress>({
  totalFiles: 0,
  processedFiles: 0,
  currentFile: '',
  status: 'idle'
});
const indexingEnabled = ref(false);
const manualReindexing = ref(false);

let progressTimer: ReturnType<typeof setInterval> | null = null;

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
const projectRootPath = ref<string>('/');
const gitignorePatterns = ref<string[]>([]);

const files = ref<FilePreview[]>([]);
let fetchPreviewsTimeout: ReturnType<typeof setTimeout> | null = null;
let isInitializing = ref(false); // Flag to prevent cascading updates during mount

const fetchFilePreviews = async () => {
  if (!currentProject.value) {
    files.value = [];
    return;
  }

  // Clear any pending fetch
  if (fetchPreviewsTimeout) {
    clearTimeout(fetchPreviewsTimeout);
  }

  // Debounce file preview fetching to avoid database locks
  fetchPreviewsTimeout = setTimeout(async () => {
    try {
      const previewConfig = {
        includePaths: includePaths.value,
        excludePatterns: excludePaths.value,
        rootPath: projectRootPath.value,
        autoExcludeHidden: autoExcludeHidden.value,
        // We intentionally do not pass selected extensions so we can still show all available chips.
        fileExtensions: [],
        continuousIndexing: currentProject.value!.config.continuousIndexing,
        chunkSizeMin: currentProject.value!.config.chunkSizeMin,
        chunkSizeMax: currentProject.value!.config.chunkSizeMax,
        embeddingModel: currentProject.value!.config.embeddingModel,
        maxResponseBytes: currentProject.value!.config.maxResponseBytes,
      };
      const result = await backend.getFilePreviews(currentProject.value!.id, previewConfig);
      // Ensure we always have an array, never null
      files.value = result || [];
    } catch (error) {
      console.error('Failed to fetch file previews:', error);
      files.value = [];
    }
  }, 300); // Wait 300ms before fetching to batch rapid changes
};

const normalizePathValue = (path: string) => {
  const trimmed = path.trim();
  if (trimmed === '/') {
    return trimmed;
  }
  return trimmed.replace(/\/+$/, '');
};

const normalizedProjectRoot = () => {
  const raw = projectRootPath.value || '/';
  const trimmed = raw.replace(/\/+$/, '');
  return trimmed === '' ? '/' : trimmed;
};

const formatFullPath = (relative: string) => {
  const root = normalizedProjectRoot();
  if (!relative || relative === '.') {
    return root;
  }
  const normalizedRelative = relative.replace(/^\/+/, '');
  if (root === '/') {
    return `/${normalizedRelative}`;
  }
  return `${root}/${normalizedRelative}`;
};

const convertAbsoluteToRelative = (absolute: string) => {
  const root = normalizedProjectRoot();
  const normalizedAbsolute = absolute.replace(/\/+$/, '');
  if (root === '/') {
    const trimmed = normalizedAbsolute.replace(/^\/+/, '');
    return trimmed === '' ? '.' : trimmed;
  }
  if (normalizedAbsolute === root) {
    return '.';
  }
  if (normalizedAbsolute.startsWith(root + '/')) {
    return normalizedAbsolute.slice(root.length + 1);
  }
  return null;
};

const projectRoot = computed(() => {
  return projectRootPath.value || '/';
});

const availableExtensions = computed(() => {
  const extensions = new Set<string>();
  files.value.forEach((file: FilePreview) => {
    if (file.extension) {
      extensions.add(file.extension);
    }
  });
  return Array.from(extensions).sort();
});

const selectedExtensions = ref<string[]>([]);

const allExtensionsSelected = computed(() => {
  const extensions = availableExtensions.value;
  if (extensions.length === 0) {
    return false;
  }
  if (selectedExtensions.value.length !== extensions.length) {
    return false;
  }
  const selectedSet = new Set(selectedExtensions.value);
  return extensions.every(ext => selectedSet.has(ext));
});

const isExtensionSelected = (extension: string) => {
  return selectedExtensions.value.includes(extension);
};

const displayedFiles = computed(() => {
  if (selectedExtensions.value.length === 0) {
    return files.value;
  }
  return files.value.filter((file: FilePreview) => selectedExtensions.value.includes(file.extension));
});

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
  const selected = await backend.selectDirectory(
    'Select a folder to include in indexing',
    projectRoot.value
  );

  if (selected) {
    const relative = convertAbsoluteToRelative(selected);
    if (!relative) {
      alert('The selected folder must live inside the project root.');
      return;
    }
    addIncludePath(relative);
  }
};

/**
 * Opens a folder picker for exclude paths.
 */
const browseExcludeFolder = async () => {
  const selected = await backend.selectDirectory(
    'Select a folder to exclude from indexing',
    projectRoot.value
  );

  if (selected) {
    addExcludePath(selected);
  }
};

const stopProgressPolling = () => {
  if (progressTimer) {
    clearInterval(progressTimer);
    progressTimer = null;
  }
};

const triggerIndexingRun = async () => {
  if (!currentProject.value) {
    return;
  }

  try {
    await backend.startIndexing(currentProject.value.id);
    console.log('Triggering indexing run for', currentProject.value.id);
  } catch (error) {
    console.error('Failed to start indexing:', error);
  }
};

const reindexNow = async () => {
  if (!currentProject.value) {
    alert('Please select a project first');
    return;
  }
  if (isIndexing.value) {
    alert('Indexing is already running. Please wait for it to finish.');
    return;
  }
  manualReindexing.value = true;
  try {
    await backend.reindexProject(currentProject.value.id);
    beginProgressPolling();
  } catch (error) {
    console.error('Failed to re-index project:', error);
    alert('Failed to re-index project: ' + (error instanceof Error ? error.message : 'Unknown error'));
    manualReindexing.value = false;
  }
};

const safeStopIndexing = async () => {
  if (!currentProject.value) {
    return;
  }

  try {
    await backend.stopIndexing(currentProject.value.id);
    console.log('Stopping indexing for', currentProject.value.id);
  } catch (error) {
    console.error('Failed to stop indexing:', error);
  }
};

const handleProgressTick = async () => {
  if (!currentProject.value) {
    return;
  }

  try {
    const latest = await backend.getIndexingProgress(currentProject.value.id);
progress.value = latest;

    if (latest.status === 'error') {
      indexingEnabled.value = false;
      stopProgressPolling();
      return;
    }

    if (manualReindexing.value && latest.status !== 'indexing') {
      manualReindexing.value = false;
    }

    if (latest.status === 'completed' && indexingEnabled.value) {
      // TODO: This should be handled by the backend
      triggerIndexingRun();
    }

    if (!indexingEnabled.value && latest.status === 'idle') {
      stopProgressPolling();
    }
  } catch (error) {
    console.error('Failed to get indexing progress:', error);
    indexingEnabled.value = false;
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

  try {
    // Update database state
    await backend.setProjectIndexing(currentProject.value.id, true);

    // Refresh current project to get updated isIndexing state
    await refreshCurrentProject();

    indexingEnabled.value = true;
    triggerIndexingRun();
    beginProgressPolling();
  } catch (error) {
    console.error('Failed to enable indexing:', error);
    alert('Failed to enable indexing: ' + (error instanceof Error ? error.message : 'Unknown error'));
    indexingEnabled.value = false;
  }
};

/**
 * Disables continuous indexing and resets progress polling.
 */
const disableContinuousIndexing = async () => {
  if (!currentProject.value) {
    return;
  }

  try {
    // Update database state
    await backend.setProjectIndexing(currentProject.value.id, false);

    // Refresh current project to get updated isIndexing state
    await refreshCurrentProject();

    indexingEnabled.value = false;
    stopProgressPolling();

    await safeStopIndexing();
    // TODO: Get progress from backend
    progress.value = {
      totalFiles: 0,
      processedFiles: 0,
      currentFile: '',
      status: 'idle'
    };
  } catch (error) {
    console.error('Failed to disable indexing:', error);
    alert('Failed to disable indexing: ' + (error instanceof Error ? error.message : 'Unknown error'));
  }
};

const indexingToggleLabel = computed(() =>
  indexingEnabled.value ? 'Disable continuous indexing' : 'Enable continuous indexing'
);

const getDefaultExcludePatterns = () => {
	return gitignorePatterns.value.length > 0
		? [...gitignorePatterns.value]
		: [...defaultExcludePatterns];
};

const isLegacyDefaultExclude = (patterns: string[]) => {
	if (!patterns || patterns.length !== defaultExcludePatterns.length) {
		return false;
	}
	const a = [...patterns].sort();
	const b = [...defaultExcludePatterns].sort();
	return a.every((value, index) => value === b[index]);
};

const loadGitignorePatterns = async (project: Project) => {
	try {
		const patterns = await backend.getGitignorePatterns(project.id);
		gitignorePatterns.value = patterns?.length ? patterns : [];
	} catch (error) {
		console.warn('Failed to load .gitignore patterns:', error);
		gitignorePatterns.value = [];
	}
};

const applyProjectConfigValues = async (project: Project) => {
	projectRootPath.value = project.config?.rootPath ?? '/';
	await loadGitignorePatterns(project);
	includePaths.value = project.config?.includePaths?.length > 0
		? project.config.includePaths
		: ['.'];
	const hasCustomExclude = project.config?.excludePatterns?.length > 0
		&& !isLegacyDefaultExclude(project.config.excludePatterns);
	if (hasCustomExclude) {
		excludePaths.value = project.config!.excludePatterns;
	} else {
		excludePaths.value = getDefaultExcludePatterns();
	}
	selectedExtensions.value = project.config?.fileExtensions ?? [];
	autoExcludeHidden.value = project.config?.autoExcludeHidden ?? true;
};

const getFileName = (relativePath: string) => {
	const normalized = relativePath || '';
	const parts = normalized.split('/');
	return parts[parts.length - 1] || normalized;
};

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
  if (availableExtensions.value.length === 0) {
    selectedExtensions.value = [];
    return;
  }

  if (allExtensionsSelected.value) {
    selectedExtensions.value = [];
    return;
  }

  selectedExtensions.value = [...availableExtensions.value];
};

onMounted(async () => {
  if (!currentProject.value) {
    return;
  }

  isInitializing.value = true; // Prevent watch triggers during initialization

  try {
    // Refresh project to ensure we have the latest state from database
    await refreshCurrentProject();

    if (!currentProject.value) {
      return;
    }

    // Apply saved configuration
    await applyProjectConfigValues(currentProject.value);
    // Initialize indexing state from project
    indexingEnabled.value = currentProject.value.isIndexing;

    const latest = await backend.getIndexingProgress(currentProject.value.id);
    progress.value = latest;

    if (indexingEnabled.value || latest.status === 'indexing') {
      beginProgressPolling();
    }

    await fetchFilePreviews();
  } finally {
    isInitializing.value = false; // Re-enable watchers
  }
});

onBeforeUnmount(() => {
  stopProgressPolling();
});

watch(currentProject, async (project, previous) => {
  // Skip if this is initial mount (handled by onMounted)
  if (isInitializing.value) {
    return;
  }

  if (project) {
    // Only process if project actually changed
    if (!previous || project.id !== previous.id) {
      isInitializing.value = true; // Prevent recursive triggers

      try {
        // Refresh project to get latest state from database
        await refreshCurrentProject();

        // Use the refreshed project state
        const refreshedProject = currentProject.value;
        if (!refreshedProject) return;

        await applyProjectConfigValues(refreshedProject);
        indexingEnabled.value = refreshedProject.isIndexing;
        stopProgressPolling();
        await safeStopIndexing();
        progress.value = {
          totalFiles: 0,
          processedFiles: 0,
          currentFile: '',
          status: 'idle'
        };

        // If indexing is enabled, start polling
        if (indexingEnabled.value) {
          beginProgressPolling();
        }
        await fetchFilePreviews();
      } finally {
        isInitializing.value = false;
      }
    }
  } else {
    includePaths.value = [];
    excludePaths.value = getDefaultExcludePatterns();
    selectedExtensions.value = [];
    autoExcludeHidden.value = true;
    projectRootPath.value = '/';
    gitignorePatterns.value = [];
    indexingEnabled.value = false;
    stopProgressPolling();
    await safeStopIndexing();
    progress.value = {
      totalFiles: 0,
      processedFiles: 0,
      currentFile: '',
      status: 'idle'
    };
    files.value = []; // Don't call fetchFilePreviews when no project
  }
});

let saveConfigTimeout: ReturnType<typeof setTimeout> | null = null;

const saveProjectConfig = async () => {
  // Skip if initializing to prevent save during mount
  if (!currentProject.value || isInitializing.value) {
    return;
  }

  // Clear any pending save
  if (saveConfigTimeout) {
    clearTimeout(saveConfigTimeout);
  }

  // Debounce config saving to avoid database locks
  saveConfigTimeout = setTimeout(async () => {
    if (!currentProject.value || isInitializing.value) {
      return;
    }

      try {
        const config = {
          includePaths: includePaths.value,
          excludePatterns: excludePaths.value,
          rootPath: projectRootPath.value,
          fileExtensions: selectedExtensions.value,
        autoExcludeHidden: autoExcludeHidden.value,
        // Other config properties will be loaded from the project and not changed here
        continuousIndexing: currentProject.value.config.continuousIndexing,
        chunkSizeMin: currentProject.value.config.chunkSizeMin,
        chunkSizeMax: currentProject.value.config.chunkSizeMax,
        embeddingModel: currentProject.value.config.embeddingModel,
        maxResponseBytes: currentProject.value.config.maxResponseBytes,
      };
      await backend.updateProjectConfig(currentProject.value.id, config);
      // DON'T call refreshCurrentProject here - it causes cascading updates
    } catch (error) {
      console.error('Failed to save project config:', error);
    }
  }, 500); // Wait 500ms before saving to batch rapid changes
};

  watch([includePaths, excludePaths, autoExcludeHidden], () => {
    // Skip during initialization
    if (isInitializing.value) {
      return;
    }
    saveProjectConfig();
    fetchFilePreviews();
  }, { deep: true });

  watch(selectedExtensions, () => {
    if (isInitializing.value) {
      return;
    }
    saveProjectConfig();
  }, { deep: true });

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
          <div class="global-progress-actions">
            <label class="indexing-toggle" :class="{ active: indexingEnabled }">
              <input
                type="checkbox"
                :checked="indexingEnabled"
                @click="indexingEnabled ? disableContinuousIndexing() : enableContinuousIndexing()"
                aria-label="Toggle continuous indexing"
              />
              <span class="toggle-track">
                <span class="toggle-thumb"></span>
              </span>
              <span class="toggle-text">{{ indexingToggleLabel }}</span>
            </label>
            <button
              type="button"
              class="btn btn-secondary reindex-button"
              @click="reindexNow"
              :disabled="manualReindexing || !hasCurrentProject || isIndexing"
            >
              {{ manualReindexing ? 'Re-indexing…' : 'Re-index now' }}
            </button>
          </div>
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
            <p class="helper-text">Project root: {{ projectRoot }}</p>
            <div class="path-pill-list">
              <div v-if="includePaths.length === 0" class="empty-pill">No include folders yet</div>
              <div
                v-for="(folder, index) in includePaths"
                :key="`include-${folder}-${index}`"
                class="path-pill"
                :title="folder"
              >
                <span>{{ formatFullPath(folder) }}</span>
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
                <span>{{ folder }}</span>
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
          <h3>File Type Filter</h3>
          <p>Configure which file extensions should be included in the index scope; in the future we may add single-file exclusion controls.</p>
        </div>

        <div class="extensions">
          <div class="extensions-header">
            <h4>File types</h4>
            <button type="button" class="chip chip-action" @click="toggleAllExtensions">
              {{ allExtensionsSelected ? 'Unselect all' : 'Select all' }}
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
          <table v-if="displayedFiles.length > 0" class="file-table">
            <thead>
              <tr>
                <th>File</th>
                <th>Extension</th>
                <th>Size</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="file in displayedFiles" :key="file.absolutePath">
                <td>
                  <div class="file-name">{{ getFileName(file.relativePath) }}</div>
                  <div class="file-path">{{ file.relativePath }}</div>
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
  width: 100%;
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

.global-progress-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.reindex-button {
  white-space: nowrap;
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
