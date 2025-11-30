<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, reactive, nextTick } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';
import { backend } from '../api/backend';
import type { IndexingProgress, FilePreview, Project, EmbeddingModelInfo, EmbeddingCapabilities, ProjectStats } from '../types';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { EMBEDDING_DOWNLOAD_PROGRESS_EVENT } from '../constants/events';

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
let downloadEventOff: (() => void) | null = null;

// Embedding model catalog state
const embeddingModels = ref<EmbeddingModelInfo[]>([]);
const selectedEmbeddingModelId = ref('');
const isLoadingEmbeddingModels = ref(false);
const showCustomModelModal = ref(false);
const isSavingEmbeddingModel = ref(false);
const suppressEmbeddingWatcher = ref(false);
const isDownloadingModel = ref(false);
const showDownloadModal = ref(false);
const currentDownloadModelId = ref('');
const downloadStage = ref('');
const downloadPercent = ref(0);
const downloadHasTotal = ref(false);
const embeddingCapabilities = ref<EmbeddingCapabilities | null>(null);
const projectStats = ref<ProjectStats | null>(null);
const statsError = ref('');
const isLoadingStats = ref(false);
const hasPersistedEmbeddings = computed(() => {
  const stats = projectStats.value;
  if (!stats) {
    return false;
  }
  return (stats.totalChunks ?? 0) > 0 || (stats.totalFiles ?? 0) > 0;
});

interface CustomModelFormData {
  id: string;
  displayName: string;
  description: string;
  dimension: number;
  diskSizeMB: number;
  ramMB: number;
  cpuLatencyMs: number;
  isMultilingual: boolean;
  codeQuality: string;
  notes: string;
  sourceType: string;
  sourceUri: string;
  license: string;
  codeFocus: string;
}

interface EmbeddingDownloadProgressPayload {
  modelId: string;
  stage: string;
  downloaded: number;
  total: number;
}

const customModelForm = reactive<CustomModelFormData>({
  id: '',
  displayName: '',
  description: '',
  dimension: 384,
  diskSizeMB: 0,
  ramMB: 0,
  cpuLatencyMs: 0,
  isMultilingual: false,
  codeQuality: 'good',
  notes: '',
  sourceType: 'custom',
  sourceUri: '',
  license: '',
  codeFocus: 'general',
});

const backendGroupOrder = ['fastembed', 'onnx'];
const backendGroupMetadata: Record<string, { label: string; description: string }> = {
  fastembed: {
    label: 'FastEmbed (CPU)',
    description: 'Lightweight CPU models (uses ONNX Runtime).',
  },
  onnx: {
    label: 'ONNX',
    description: 'Larger ONNX models. Requires the onnxruntime library.',
  },
};

const backendRequiresOnnx = () => true;

const backendOrderIndex = (backend: string) => {
  const normalized = backend.toLowerCase();
  const idx = backendGroupOrder.indexOf(normalized);
  if (idx === -1) {
    return backendGroupOrder.length;
  }
  return idx;
};

const capabilitiesKnown = computed(() => embeddingCapabilities.value !== null);
const onnxRuntimeAvailable = computed(() => !capabilitiesKnown.value || !!embeddingCapabilities.value?.onnxRuntimeAvailable);

const getModelBackend = (model?: EmbeddingModelInfo) => {
  if (!model?.backend) {
    return 'onnx';
  }
  return model.backend.toLowerCase();
};

const selectedEmbeddingModel = computed<EmbeddingModelInfo | undefined>(() => {
  return embeddingModels.value.find(model => model.id === selectedEmbeddingModelId.value);
});

const embeddingModelStatusLabel = computed(() => {
  if (!selectedEmbeddingModel.value) {
    return 'N/A';
  }
  const status = selectedEmbeddingModel.value.downloadStatus || 'pending';
  switch (status) {
    case 'ready':
      return 'Ready';
    case 'downloading':
      return 'Downloading…';
    case 'error':
      return 'Error';
    case 'missing':
      return 'Missing';
    default:
      return 'Pending download';
  }
});

const needsModelDownload = computed(() => {
  return !!selectedEmbeddingModel.value && selectedEmbeddingModel.value.downloadStatus !== 'ready';
});

const lastIndexedModelInfo = computed<EmbeddingModelInfo | undefined>(() => {
  if (projectStats.value?.lastEmbeddingModel?.id) {
    return projectStats.value.lastEmbeddingModel;
  }
  return undefined;
});

const embeddingUsageSummaries = computed(() => {
  const usage = projectStats.value?.embeddingModels ?? [];
  if (usage.length === 0) {
    return [];
  }
  const total = usage.reduce((sum, entry) => sum + (entry.chunkCount ?? 0), 0);
  return usage.map(entry => {
    const label = entry.modelInfo?.displayName || entry.modelId || 'Unknown model';
    const details = entry.modelInfo ? describeModelAttributes(entry.modelInfo) : '';
    const percent = total > 0 ? Math.round((entry.chunkCount / total) * 100) : 0;
    return {
      id: entry.modelId || label,
      label,
      details: details || 'Model metadata unavailable.',
      chunkCount: entry.chunkCount,
      percent,
      modelId: entry.modelId || '',
      modelInfo: entry.modelInfo,
    };
  });
});

const primaryEmbeddingUsage = computed(() => embeddingUsageSummaries.value[0]);

const storedEmbeddingModelLabel = computed(() => {
  if (primaryEmbeddingUsage.value) {
    return primaryEmbeddingUsage.value.label;
  }
  if (lastIndexedModelInfo.value?.displayName) {
    return lastIndexedModelInfo.value.displayName;
  }
  if (hasPersistedEmbeddings.value) {
    return 'Unknown model';
  }
  return 'Not indexed yet';
});

const embeddingModelSelectionMismatch = computed(() => {
  if (!selectedEmbeddingModelId.value) {
    return false;
  }
  const referenceId =
    primaryEmbeddingUsage.value?.modelInfo?.id ||
    primaryEmbeddingUsage.value?.modelId ||
    lastIndexedModelInfo.value?.id;
  if (!referenceId) {
    return false;
  }
  return referenceId.trim().toLowerCase() !== selectedEmbeddingModelId.value.trim().toLowerCase();
});

const mismatchReferenceLabel = computed(() => {
  if (primaryEmbeddingUsage.value) {
    return primaryEmbeddingUsage.value.label;
  }
  if (lastIndexedModelInfo.value?.displayName) {
    return lastIndexedModelInfo.value.displayName;
  }
  return 'stored embeddings';
});

const indexingResumeMessage = computed(() => {
  if (isIndexing.value && progress.value.status === 'indexing') {
    if (progress.value.totalFiles > 0) {
      return `Indexing in progress (${progress.value.processedFiles}/${progress.value.totalFiles} files processed)`;
    }
    return 'Indexing started… preparing file list';
  }
  if (progress.value.status === 'error' && progress.value.error) {
    return `Last run failed: ${progress.value.error}`;
  }
  return '';
});

const groupedEmbeddingModels = computed(() => {
  const groups = new Map<string, { backend: string; label: string; description: string; models: EmbeddingModelInfo[]; disabled: boolean }>();

  const ensureGroup = (backend: string) => {
    const normalized = backend.toLowerCase();
    if (!groups.has(normalized)) {
      const meta = backendGroupMetadata[normalized] || {
        label: normalized.toUpperCase(),
        description: 'Modello personalizzato',
      };
      groups.set(normalized, {
        backend: normalized,
        label: meta.label,
        description: meta.description,
        models: [],
        disabled: normalized === 'onnx' && !onnxRuntimeAvailable.value,
      });
    }
    return groups.get(normalized)!;
  };

  embeddingModels.value.forEach(model => {
    const backend = getModelBackend(model) || 'onnx';
    ensureGroup(backend).models.push(model);
  });

  const sorted = Array.from(groups.values());
  sorted.forEach(group => {
    group.models.sort((a, b) => a.displayName.localeCompare(b.displayName));
    group.disabled = backendRequiresOnnx() && !onnxRuntimeAvailable.value;
  });
  sorted.sort((a, b) => {
    const orderDiff = backendOrderIndex(a.backend) - backendOrderIndex(b.backend);
    if (orderDiff !== 0) {
      return orderDiff;
    }
    return a.label.localeCompare(b.label);
  });
  return sorted;
});

const hasModelsRequiringOnnx = computed(() => groupedEmbeddingModels.value.some(group => backendRequiresOnnx() && group.models.length > 0));

const formatBytesToMB = (bytes?: number) => {
  if (!bytes || bytes <= 0) {
    return '0';
  }
  return (bytes / (1024 * 1024)).toFixed(bytes >= 200 * 1024 * 1024 ? 0 : 1);
};

const describeModelAttributes = (model: EmbeddingModelInfo) => {
  const parts: string[] = [];
  if (model.dimension) {
    parts.push(`${model.dimension} dims`);
  }
  if (model.diskSizeBytes) {
    parts.push(`Disk ~${formatBytesToMB(model.diskSizeBytes)} MB`);
  }
  if (model.ramRequirementBytes) {
    parts.push(`RAM ~${formatBytesToMB(model.ramRequirementBytes)} MB`);
  }
  if (model.cpuLatencyMs) {
    parts.push(`~${model.cpuLatencyMs} ms`);
  }
  parts.push(model.isMultilingual ? 'Multilingual' : 'English only');
  if (model.codeQuality) {
    parts.push(model.codeQuality);
  }
  if (model.codeFocus) {
    parts.push(model.codeFocus);
  }
  return parts.join(' · ');
};

const describeModelOption = (model: EmbeddingModelInfo) => {
  const attributes = describeModelAttributes(model);
  if (attributes) {
    return `${model.displayName} · ${attributes}`;
  }
  return model.displayName;
};

const selectedModelLabel = computed(() => {
  if (selectedEmbeddingModel.value) {
    return describeModelOption(selectedEmbeddingModel.value);
  }
  if (embeddingModels.value.length === 0) {
    return isLoadingEmbeddingModels.value ? 'Loading models…' : 'No models available';
  }
  return 'Select an embedding model';
});

const isEmbeddingDropdownOpen = ref(false);
const embeddingSelectRef = ref<HTMLElement | null>(null);

const toggleEmbeddingDropdown = () => {
  if (isLoadingEmbeddingModels.value || embeddingModels.value.length === 0) {
    return;
  }
  isEmbeddingDropdownOpen.value = !isEmbeddingDropdownOpen.value;
};

const closeEmbeddingDropdown = () => {
  isEmbeddingDropdownOpen.value = false;
};

const handleSelectModelFromDropdown = (model: EmbeddingModelInfo, disabled: boolean) => {
  if (disabled) {
    return;
  }
  selectedEmbeddingModelId.value = model.id;
  closeEmbeddingDropdown();
};

const handleWindowClick = (event: MouseEvent) => {
  if (!isEmbeddingDropdownOpen.value) {
    return;
  }
  const target = event.target as Node;
  if (embeddingSelectRef.value && !embeddingSelectRef.value.contains(target)) {
    closeEmbeddingDropdown();
  }
};

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
        embeddingModelInfo: currentProject.value!.config.embeddingModelInfo,
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

// ===== Embedding model helpers =====

const cloneModelInfo = (model?: EmbeddingModelInfo) => {
  if (!model) {
    return undefined;
  }
  return JSON.parse(JSON.stringify(model)) as EmbeddingModelInfo;
};

const updateProjectEmbeddingSelection = async (modelId: string, options?: { skipSave?: boolean }) => {
  if (!currentProject.value) {
    return;
  }
  currentProject.value.config.embeddingModel = modelId;
  const match = embeddingModels.value.find(model => model.id === modelId);
  currentProject.value.config.embeddingModelInfo = cloneModelInfo(match);

  if (!options?.skipSave) {
    await saveProjectConfig({ immediate: true });
  }
};

const syncEmbeddingSelectionFromProject = (project?: Project | null, options?: { skipSave?: boolean }) => {
  if (!project) {
    return;
  }
  let targetId = project.config?.embeddingModel;
  if (!targetId && embeddingModels.value.length > 0) {
    targetId = embeddingModels.value[0].id;
  }
  suppressEmbeddingWatcher.value = true;
  selectedEmbeddingModelId.value = targetId || '';
  nextTick(() => {
    suppressEmbeddingWatcher.value = false;
  });
  if (targetId) {
    void updateProjectEmbeddingSelection(targetId, { skipSave: options?.skipSave ?? true });
  }
};

const loadEmbeddingCapabilities = async () => {
  try {
    const capabilities = await backend.getEmbeddingCapabilities();
    embeddingCapabilities.value = capabilities || null;
  } catch (error) {
    console.error('Failed to load embedding capabilities:', error);
    embeddingCapabilities.value = null;
  }
};

const loadEmbeddingCatalog = async () => {
  try {
    isLoadingEmbeddingModels.value = true;
    const catalog = await backend.listEmbeddingModels();
    embeddingModels.value = catalog || [];
    syncEmbeddingSelectionFromProject(currentProject.value, { skipSave: true });
  } catch (error) {
    console.error('Failed to load embedding models:', error);
    embeddingModels.value = [];
  } finally {
    isLoadingEmbeddingModels.value = false;
  }
};

const loadProjectStats = async () => {
  if (!currentProject.value) {
    projectStats.value = null;
    statsError.value = '';
    return;
  }
  isLoadingStats.value = true;
  try {
    const stats = await backend.getProjectStats(currentProject.value.id);
    projectStats.value = stats || null;
    statsError.value = '';
  } catch (error) {
    console.error('Failed to load project stats:', error);
    statsError.value = error instanceof Error ? error.message : 'Unknown error';
    projectStats.value = null;
  } finally {
    isLoadingStats.value = false;
  }
};

const resetCustomModelForm = () => {
  customModelForm.id = '';
  customModelForm.displayName = '';
  customModelForm.description = '';
  customModelForm.dimension = selectedEmbeddingModel.value?.dimension || 384;
  customModelForm.diskSizeMB = 0;
  customModelForm.ramMB = 0;
  customModelForm.cpuLatencyMs = 0;
  customModelForm.isMultilingual = false;
  customModelForm.codeQuality = 'good';
  customModelForm.notes = '';
  customModelForm.sourceType = 'custom';
  customModelForm.sourceUri = '';
  customModelForm.license = '';
  customModelForm.codeFocus = 'general';
};

const openCustomModelModal = () => {
  resetCustomModelForm();
  showCustomModelModal.value = true;
};

const closeCustomModelModal = () => {
  if (isSavingEmbeddingModel.value) {
    return;
  }
  showCustomModelModal.value = false;
};

const saveCustomModel = async () => {
  if (!customModelForm.displayName.trim()) {
    alert('Please provide a name for the model.');
    return;
  }
  if (!customModelForm.dimension || customModelForm.dimension <= 0) {
    alert('Dimension must be greater than zero.');
    return;
  }

  isSavingEmbeddingModel.value = true;
  try {
    const payload = {
      id: customModelForm.id.trim(),
      displayName: customModelForm.displayName.trim(),
      description: customModelForm.description.trim(),
      dimension: customModelForm.dimension,
      diskSizeBytes: Math.round((customModelForm.diskSizeMB || 0) * 1024 * 1024),
      ramRequirementBytes: Math.round((customModelForm.ramMB || 0) * 1024 * 1024),
      cpuLatencyMs: customModelForm.cpuLatencyMs || undefined,
      isMultilingual: customModelForm.isMultilingual,
      codeQuality: customModelForm.codeQuality,
      notes: customModelForm.notes.trim(),
      sourceType: customModelForm.sourceType || 'custom',
      sourceUri: customModelForm.sourceUri.trim(),
      license: customModelForm.license.trim(),
      downloadStatus: 'pending',
      codeFocus: customModelForm.codeFocus,
    } as EmbeddingModelInfo;

    const saved = await backend.saveEmbeddingModel(payload);
    await loadEmbeddingCatalog();
    suppressEmbeddingWatcher.value = true;
    selectedEmbeddingModelId.value = saved.id;
    nextTick(() => {
      suppressEmbeddingWatcher.value = false;
    });
    await updateProjectEmbeddingSelection(saved.id);
    showCustomModelModal.value = false;
  } catch (error) {
    console.error('Failed to save embedding model:', error);
    alert('Failed to save embedding model. Please check the console for details.');
  } finally {
    isSavingEmbeddingModel.value = false;
  }
};

const handleDownloadEvent = (payload: EmbeddingDownloadProgressPayload) => {
  if (!payload || payload.modelId !== currentDownloadModelId.value || !showDownloadModal.value) {
    return;
  }
  downloadStage.value = payload.stage || 'Downloading…';
  if (typeof payload.total === 'number' && payload.total > 0) {
    downloadHasTotal.value = true;
    const ratio = Math.min(Math.max(payload.downloaded / payload.total, 0), 1);
    downloadPercent.value = Math.round(ratio * 100);
  } else {
    downloadHasTotal.value = false;
    downloadPercent.value = 0;
  }
};

const downloadSelectedModel = async () => {
  if (!selectedEmbeddingModelId.value) {
    return;
  }
  currentDownloadModelId.value = selectedEmbeddingModelId.value;
  downloadStage.value = 'Starting…';
  downloadPercent.value = 0;
  downloadHasTotal.value = false;
  showDownloadModal.value = true;
  isDownloadingModel.value = true;
  try {
    const updated = await backend.downloadEmbeddingModel(selectedEmbeddingModelId.value);
    await loadEmbeddingCatalog();
    // Ensure selected metadata reflects the freshly downloaded model
    if (currentProject.value) {
      currentProject.value.config.embeddingModelInfo = cloneModelInfo(updated);
    }
    syncEmbeddingSelectionFromProject(currentProject.value, { skipSave: true });
  } catch (error) {
    console.error('Failed to download embedding model:', error);
    const message = error instanceof Error ? error.message : 'Unknown error';
    alert(`Failed to download model: ${message}`);
  } finally {
    isDownloadingModel.value = false;
    showDownloadModal.value = false;
    currentDownloadModelId.value = '';
    downloadStage.value = '';
    downloadPercent.value = 0;
    downloadHasTotal.value = false;
  }
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

const triggerIndexingRun = async (): Promise<boolean> => {
  if (!currentProject.value) {
    return false;
  }

  try {
    await backend.startIndexing(currentProject.value.id);
    console.log('Triggering indexing run for', currentProject.value.id);
    await loadProjectStats();
    return true;
  } catch (error) {
    console.error('Failed to start indexing:', error);
    const message = error instanceof Error ? error.message : 'Unknown error';
    alert(`Failed to start indexing: ${message}`);
    return false;
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
    await loadProjectStats();
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
    const previousStatus = progress.value.status;
    progress.value = latest;
    if (latest.status === 'completed' && previousStatus !== 'completed') {
      await loadProjectStats();
    }

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
      const started = await triggerIndexingRun();
      if (!started) {
        indexingEnabled.value = false;
        await backend.setProjectIndexing(currentProject.value.id, false);
        stopProgressPolling();
      }
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

    const started = await triggerIndexingRun();
    if (!started) {
      await backend.setProjectIndexing(currentProject.value.id, false);
      await refreshCurrentProject();
      indexingEnabled.value = false;
      stopProgressPolling();
      return;
    }

    indexingEnabled.value = true;
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
    await loadProjectStats();

    await safeStopIndexing();
    indexingEnabled.value = false;
    stopProgressPolling();
    progress.value = {
      ...progress.value,
      status: 'idle',
      currentFile: '',
      error: ''
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
	syncEmbeddingSelectionFromProject(project, { skipSave: true });
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
  if (typeof window !== 'undefined') {
    window.addEventListener('click', handleWindowClick);
  }

  downloadEventOff = EventsOn(EMBEDDING_DOWNLOAD_PROGRESS_EVENT, (payload: EmbeddingDownloadProgressPayload) => {
    handleDownloadEvent(payload);
  });

  await loadEmbeddingCapabilities();

  if (!currentProject.value) {
    await loadEmbeddingCatalog();
    return;
  }

  isInitializing.value = true; // Prevent watch triggers during initialization

  try {
    // Refresh project to ensure we have the latest state from database
    await refreshCurrentProject();
    await loadProjectStats();
    await loadEmbeddingCatalog();

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
    await loadProjectStats();
  } finally {
    isInitializing.value = false; // Re-enable watchers
  }
});

onBeforeUnmount(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('click', handleWindowClick);
  }
  stopProgressPolling();
  if (downloadEventOff) {
    downloadEventOff();
    downloadEventOff = null;
  }
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
        await loadEmbeddingCatalog();
        await loadProjectStats();

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
    suppressEmbeddingWatcher.value = true;
    selectedEmbeddingModelId.value = '';
    nextTick(() => {
      suppressEmbeddingWatcher.value = false;
    });
    projectStats.value = null;
    statsError.value = '';
  }
});

let saveConfigTimeout: ReturnType<typeof setTimeout> | null = null;

const buildProjectConfigPayload = () => {
  if (!currentProject.value) {
    return null;
  }
  return {
    includePaths: includePaths.value,
    excludePatterns: excludePaths.value,
    rootPath: projectRootPath.value,
    fileExtensions: selectedExtensions.value,
    autoExcludeHidden: autoExcludeHidden.value,
    continuousIndexing: currentProject.value.config.continuousIndexing,
    chunkSizeMin: currentProject.value.config.chunkSizeMin,
    chunkSizeMax: currentProject.value.config.chunkSizeMax,
    embeddingModel: currentProject.value.config.embeddingModel,
    embeddingModelInfo: currentProject.value.config.embeddingModelInfo,
    maxResponseBytes: currentProject.value.config.maxResponseBytes,
  };
};

const persistProjectConfig = async () => {
  if (!currentProject.value) {
    return;
  }
  const payload = buildProjectConfigPayload();
  if (!payload) {
    return;
  }
  await backend.updateProjectConfig(currentProject.value.id, payload);
};

async function saveProjectConfig(options?: { immediate?: boolean }) {
  // Skip if initializing to prevent save during mount
  if (!currentProject.value || isInitializing.value) {
    return;
  }

  // Clear any pending save
  if (saveConfigTimeout) {
    clearTimeout(saveConfigTimeout);
  }

  if (options?.immediate) {
    if (saveConfigTimeout) {
      clearTimeout(saveConfigTimeout);
      saveConfigTimeout = null;
    }
    try {
      await persistProjectConfig();
    } catch (error) {
      console.error('Failed to save project config:', error);
    }
    return;
  }

  // Debounce config saving to avoid database locks
  saveConfigTimeout = setTimeout(async () => {
    if (!currentProject.value || isInitializing.value) {
      return;
    }

    try {
      await persistProjectConfig();
      // DON'T call refreshCurrentProject here - it causes cascading updates
    } catch (error) {
      console.error('Failed to save project config:', error);
    }
  }, 500); // Wait 500ms before saving to batch rapid changes
}

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

watch(selectedEmbeddingModelId, async modelId => {
  if (isInitializing.value || suppressEmbeddingWatcher.value) {
    return;
  }
  if (!modelId) {
    return;
  }
  await updateProjectEmbeddingSelection(modelId);
  if (indexingEnabled.value) {
    try {
      await disableContinuousIndexing();
    } catch (error) {
      console.error('Failed to disable continuous indexing after model change:', error);
    }
  }
});

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
        <p v-if="indexingResumeMessage" class="resume-notice">
          {{ indexingResumeMessage }}
        </p>
        <p v-if="statsError" class="stats-error">
          {{ statsError }}
        </p>
        <div v-if="progress.status === 'error' && progress.error" class="error-banner">
          <strong>Error:</strong> {{ progress.error }}
        </div>
      </div>

      <section class="config-card embedding-model-card">
        <header class="config-card-header">
          <div>
            <h3>Embedding model</h3>
            <p>Select which embedding model generates vectors for this project.</p>
          </div>
          <button type="button" class="btn btn-secondary" @click="openCustomModelModal">
            Add custom model
          </button>
        </header>
        <div class="model-selector-row">
          <label for="embeddingModelSelect">Model</label>
          <div class="selector-column" ref="embeddingSelectRef">
            <button
              id="embeddingModelSelect"
              type="button"
              class="custom-select-trigger"
              :class="{ open: isEmbeddingDropdownOpen, disabled: isLoadingEmbeddingModels || embeddingModels.length === 0 }"
              :disabled="isLoadingEmbeddingModels || embeddingModels.length === 0"
              @click.stop="toggleEmbeddingDropdown"
            >
              <span class="custom-select-label">{{ selectedModelLabel }}</span>
              <span class="custom-select-chevron">▾</span>
            </button>
            <div v-if="isEmbeddingDropdownOpen" class="custom-select-dropdown">
              <template v-if="groupedEmbeddingModels.length > 0">
                <div v-for="group in groupedEmbeddingModels" :key="`group-${group.backend}`" class="custom-select-group">
                  <div class="custom-select-group-header">
                    <strong>{{ group.label }}</strong>
                    <span class="group-description"> — {{ group.description }}</span>
                    <span v-if="group.disabled" class="legend-disabled">(ONNX Runtime required)</span>
                  </div>
                  <button
                    v-for="model in group.models"
                    :key="model.id"
                    type="button"
                    class="custom-select-option"
                    :class="{ disabled: group.disabled, selected: model.id === selectedEmbeddingModelId }"
                    :disabled="group.disabled"
                    @click.stop="handleSelectModelFromDropdown(model, group.disabled)"
                  >
                    {{ describeModelOption(model) }}
                  </button>
                </div>
              </template>
              <div v-else class="custom-select-empty">
                {{ isLoadingEmbeddingModels ? 'Loading models…' : 'No models available' }}
              </div>
            </div>
            <ul v-if="groupedEmbeddingModels.length > 0" class="backend-legend">
              <li v-for="group in groupedEmbeddingModels" :key="`legend-${group.backend}`">
                <strong>{{ group.label }}</strong>
                <span> — {{ group.description }}</span>
                <span v-if="group.disabled" class="legend-disabled">(ONNX Runtime required)</span>
              </li>
            </ul>
            <p v-if="!onnxRuntimeAvailable && hasModelsRequiringOnnx" class="runtime-warning">
              ONNX Runtime not detected: install version 1.22.0, set its shared library path under Projects → “ONNX runtime path”, and restart to enable FastEmbed/ONNX models. GPU builds require CUDA 12.x + cuDNN 9.x.
            </p>
          </div>
          <div class="model-actions">
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
        <div v-if="selectedEmbeddingModel" class="model-details">
          <div class="model-history-row">
            <div class="history-label">Embeddings in database</div>
            <div class="history-content">
              <template v-if="embeddingUsageSummaries.length > 0">
                <div
                  v-for="usage in embeddingUsageSummaries"
                  :key="`usage-${usage.id}`"
                  class="history-usage"
                >
                  <div class="history-usage-header">
                    <strong>{{ usage.label }}</strong>
                    <span class="history-count">
                      {{ usage.chunkCount }} chunks
                      <span v-if="usage.percent">({{ usage.percent }}%)</span>
                    </span>
                  </div>
                </div>
              </template>
              <template v-else>
                <strong>{{ storedEmbeddingModelLabel }}</strong>
              </template>
            </div>
          </div>
          <div
            v-if="embeddingModelSelectionMismatch"
            class="model-mismatch-warning"
          >
            Stored embeddings were generated with {{ mismatchReferenceLabel }}.
            Re-index to apply {{ selectedEmbeddingModel?.displayName || 'the selected model' }}.
          </div>
          <div class="model-status-row">
            <span class="status-chip" :class="selectedEmbeddingModel.downloadStatus">
              {{ embeddingModelStatusLabel }}
            </span>
            <span v-if="selectedEmbeddingModel.localPath" class="model-path">
              Saved at {{ selectedEmbeddingModel.localPath }}
            </span>
            <button
              v-if="needsModelDownload"
              type="button"
              class="btn btn-primary"
              :disabled="isDownloadingModel"
              @click="downloadSelectedModel"
            >
              {{ isDownloadingModel ? 'Downloading…' : 'Download model' }}
            </button>
          </div>
        </div>
        <p v-else class="empty-state-text">Add a model to start indexing this project.</p>
      </section>

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
  <div v-if="showCustomModelModal" class="modal-overlay">
    <div class="modal-card">
      <header>
        <h3>Add embedding model</h3>
        <button
          type="button"
          class="modal-close"
          @click="closeCustomModelModal"
          aria-label="Close modal"
        >
          ×
        </button>
      </header>
      <form @submit.prevent="saveCustomModel">
        <div class="modal-body">
          <div class="modal-grid">
            <label>
              Display name
              <input v-model="customModelForm.displayName" type="text" placeholder="Model name" required />
            </label>
            <label>
              Model ID / slug
              <input v-model="customModelForm.id" type="text" placeholder="hf-org/model" />
            </label>
            <label>
              Dimension
              <input v-model.number="customModelForm.dimension" type="number" min="1" required />
            </label>
            <label>
              Disk size (MB)
              <input v-model.number="customModelForm.diskSizeMB" type="number" min="0" step="1" />
            </label>
            <label>
              RAM (MB)
              <input v-model.number="customModelForm.ramMB" type="number" min="0" step="1" />
            </label>
            <label>
              Latency (ms @512 tok)
              <input v-model.number="customModelForm.cpuLatencyMs" type="number" min="0" step="1" />
            </label>
            <label class="inline-checkbox">
              <input type="checkbox" v-model="customModelForm.isMultilingual" />
              Multilingual
            </label>
            <label>
              Code quality
              <select v-model="customModelForm.codeQuality">
                <option value="excellent">Excellent</option>
                <option value="great">Great</option>
                <option value="good">Good</option>
                <option value="fair">Fair</option>
              </select>
            </label>
            <label>
              Code focus
              <select v-model="customModelForm.codeFocus">
                <option value="general">General</option>
                <option value="code">Code</option>
                <option value="docs">Docs</option>
              </select>
            </label>
            <label>
              Source type
              <select v-model="customModelForm.sourceType">
                <option value="huggingface">Hugging Face</option>
                <option value="onnx">ONNX</option>
                <option value="custom">Custom</option>
              </select>
            </label>
            <label>
              Source URL / path
              <input v-model="customModelForm.sourceUri" type="text" placeholder="https://huggingface.co/..." />
            </label>
            <label>
              License
              <input v-model="customModelForm.license" type="text" placeholder="Apache-2.0" />
            </label>
          </div>
          <label>
            Notes
            <textarea v-model="customModelForm.notes" rows="3" placeholder="Optional notes"></textarea>
          </label>
        </div>
        <div class="modal-actions">
          <button type="button" class="btn btn-secondary" @click="closeCustomModelModal" :disabled="isSavingEmbeddingModel">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" :disabled="isSavingEmbeddingModel">
            {{ isSavingEmbeddingModel ? 'Saving…' : 'Save model' }}
          </button>
        </div>
      </form>
    </div>
  </div>
  <div v-if="showDownloadModal" class="download-modal-backdrop">
    <div class="download-modal">
      <p>
        Downloading
        {{ selectedEmbeddingModel?.displayName || 'model' }}
        <span v-if="downloadStage"> — {{ downloadStage }}</span>
      </p>
      <p v-if="downloadHasTotal" class="download-percent">{{ downloadPercent }}%</p>
      <div class="download-progress" :class="{ indeterminate: !downloadHasTotal }">
        <span :style="downloadHasTotal ? { width: `${downloadPercent}%` } : {}"></span>
      </div>
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

.config-card {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.config-card-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
  margin-bottom: 1rem;
}

.config-card-header h3 {
  margin: 0;
  color: #d4d4d4;
  font-size: 1.2rem;
}

.config-card-header p {
  margin: 0.35rem 0 0 0;
  color: #858585;
  font-size: 0.9rem;
}

.embedding-model-card .model-selector-row {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin-bottom: 1rem;
}

.model-actions {
  margin-top: 0.5rem;
}

.model-actions .btn {
  width: fit-content;
}

.embedding-model-card label {
  color: #d4d4d4;
  font-weight: 500;
  font-size: 0.9rem;
}

.embedding-model-card select {
  background: #1b1b1b;
  border: 1px solid #3e3e42;
  color: #d4d4d4;
  border-radius: 6px;
  padding: 0.5rem 0.75rem;
}

.selector-column {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  width: 100%;
  position: relative;
}

.custom-select-trigger {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #101010;
  border: 1px solid #3e3e42;
  color: #e0e0e0;
  border-radius: 6px;
  padding: 0.5rem 0.75rem;
  font-size: 0.95rem;
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.custom-select-trigger.disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.custom-select-trigger.open {
  border-color: #7c6bff;
  box-shadow: 0 0 0 2px rgba(124, 107, 255, 0.35);
}

.custom-select-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding-right: 0.75rem;
}

.custom-select-chevron {
  font-size: 0.85rem;
}

.custom-select-dropdown {
  position: absolute;
  margin-top: 0.35rem;
  width: 100%;
  background: #151515;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  box-shadow: 0 12px 25px rgba(0, 0, 0, 0.45);
  max-height: 320px;
  overflow-y: auto;
  z-index: 20;
}

.custom-select-group {
  padding: 0.5rem 0.75rem;
}

.custom-select-group-header {
  color: #b8b8b8;
  font-size: 0.85rem;
  margin-bottom: 0.35rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.group-description {
  color: #8c8c8c;
}

.custom-select-option {
  width: 100%;
  text-align: left;
  background: transparent;
  border: none;
  color: #e0e0e0;
  padding: 0.35rem 0;
  font-size: 0.9rem;
  cursor: pointer;
}

.custom-select-option:hover:not(.disabled),
.custom-select-option.selected {
  color: #7c6bff;
}

.custom-select-option.disabled {
  color: #6a6a6a;
  cursor: not-allowed;
}

.custom-select-empty {
  padding: 0.75rem;
  color: #9a9da3;
  font-size: 0.9rem;
}

.backend-legend {
  list-style: none;
  margin: 0;
  padding-left: 0;
  color: #a0a0a0;
  font-size: 0.85rem;
}

.backend-legend li {
  margin-bottom: 0.25rem;
}

.backend-legend li:last-child {
  margin-bottom: 0;
}

.legend-disabled {
  color: #f39c12;
  margin-left: 0.35rem;
  font-weight: 500;
}

.runtime-warning {
  color: #f39c12;
  font-size: 0.85rem;
  margin: 0;
}

.model-details {
  background: #1b1b1b;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1rem;
}

.model-history-row {
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 0.75rem;
  margin-bottom: 0.75rem;
  background: #202023;
}

.history-label {
  font-size: 0.78rem;
  color: #9ea0a6;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  margin-bottom: 0.25rem;
  display: block;
}

.history-content {
  display: flex;
  flex-direction: column;
}

.history-content strong {
  color: #f0f0f0;
  font-size: 1rem;
  display: block;
  margin-bottom: 0.2rem;
}

.history-usage {
  padding: 0.4rem 0;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.history-usage:first-child {
  border-top: none;
  padding-top: 0;
}

.history-usage-header {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
  align-items: baseline;
}

.history-count {
  color: #9ea0a6;
  font-size: 0.85rem;
}

.model-mismatch-warning {
  border: 1px solid rgba(255, 166, 0, 0.4);
  background: rgba(255, 166, 0, 0.08);
  color: #ffca7a;
  border-radius: 6px;
  padding: 0.75rem;
  font-size: 0.9rem;
  margin-bottom: 0.75rem;
}

.model-status-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  margin-bottom: 0.75rem;
}

.status-chip {
  padding: 0.2rem 0.75rem;
  border-radius: 999px;
  font-size: 0.78rem;
  border: 1px solid #3e3e42;
  background: #2d2f34;
  color: #d4d4d4;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.status-chip.ready {
  border-color: #4ec9b0;
  color: #4ec9b0;
}

.status-chip.downloading {
  border-color: #c5a04e;
  color: #c5a04e;
}

.status-chip.error,
.status-chip.missing {
  border-color: #dc3545;
  color: #ff9a9a;
}

.model-path {
  color: #9a9da3;
  font-size: 0.85rem;
}

.empty-state-text {
  margin: 0;
  color: #858585;
  font-size: 0.9rem;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 1rem;
}

.modal-card {
  width: min(700px, 95vw);
  max-height: 90vh;
  overflow-y: auto;
  background: #1f1f1f;
  border: 1px solid #3e3e42;
  border-radius: 10px;
  padding: 1.5rem;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.4);
}

.modal-card header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.modal-card header h3 {
  margin: 0;
  color: #d4d4d4;
}

.modal-close {
  background: none;
  border: none;
  color: #858585;
  font-size: 1.4rem;
  cursor: pointer;
  line-height: 1;
}

.modal-body {
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
}

.modal-grid {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
}

.modal-grid label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  color: #d4d4d4;
  font-size: 0.88rem;
}

.modal-grid input,
.modal-grid select,
.modal-grid textarea {
  background: #101010;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 0.45rem 0.65rem;
  color: #e0e0e0;
  font-size: 0.9rem;
}

.modal-body textarea {
  background: #101010;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 0.45rem 0.65rem;
  color: #e0e0e0;
  font-size: 0.9rem;
}

.inline-checkbox {
  flex-direction: row !important;
  align-items: center;
}

.inline-checkbox input {
  margin-right: 0.4rem;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 1rem;
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

.project-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.75rem;
  margin-top: 1rem;
}

.metric {
  background: #1e1e1e;
  border: 1px solid #313131;
  border-radius: 8px;
  padding: 0.75rem 1rem;
}

.metric-label {
  display: block;
  font-size: 0.75rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #888;
  margin-bottom: 0.25rem;
}

.metric strong {
  display: block;
  color: #f0f0f0;
  font-size: 1rem;
}

.metric small {
  display: block;
  color: #9ea1ff;
  margin-top: 0.15rem;
  font-size: 0.8rem;
}

.resume-notice {
  margin: 0.5rem 0 0;
  color: #a0afff;
  font-size: 0.9rem;
}

.stats-error {
  color: #f28b82;
  margin-top: 0.5rem;
  font-size: 0.85rem;
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

.download-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000;
}

.download-modal {
  background: #202225;
  border: 1px solid #3e3e42;
  border-radius: 10px;
  padding: 1.5rem;
  width: min(90%, 360px);
  box-shadow: 0 20px 45px rgba(0, 0, 0, 0.65);
  text-align: center;
}

.download-modal p {
  margin: 0 0 1rem 0;
  color: #e5e7eb;
  font-size: 1rem;
}

.download-progress {
  width: 100%;
  height: 6px;
  background: #2f3136;
  border-radius: 999px;
  overflow: hidden;
}

.download-progress span {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, #7c6bff, #9f95ff);
  transition: width 0.2s ease;
}

.download-progress.indeterminate span {
  width: 40%;
  animation: download-progress-indeterminate 1.2s linear infinite;
}

.download-percent {
  margin: 0 0 0.5rem 0;
  color: #cfd2f5;
  font-weight: 600;
}

@keyframes download-progress-indeterminate {
  0% {
    margin-left: -40%;
  }
  100% {
    margin-left: 100%;
  }
}
</style>
