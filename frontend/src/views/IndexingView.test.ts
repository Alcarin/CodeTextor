import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import IndexingView from './IndexingView.vue';
import { backend } from '../api/backend';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';

vi.mock('../composables/useCurrentProject');
vi.mock('../composables/useNavigation');
vi.mock('../api/backend', () => ({
  backend: {
    getEmbeddingCapabilities: vi.fn().mockResolvedValue({ onnxRuntimeAvailable: true }),
    listEmbeddingModels: vi.fn().mockResolvedValue([]),
    getProjectStats: vi.fn().mockResolvedValue({
      totalFiles: 0,
      totalChunks: 0,
      totalSymbols: 0,
      databaseSize: 0,
      convertValues: (value: any) => value
    }),
    getIndexingProgress: vi.fn().mockResolvedValue({
      totalFiles: 0,
      processedFiles: 0,
      currentFile: '',
      status: 'idle'
    }),
    getGitignorePatterns: vi.fn().mockResolvedValue([]),
    getFilePreviews: vi.fn().mockResolvedValue([]),
    selectDirectory: vi.fn().mockResolvedValue('/tmp'),
    setProjectIndexing: vi.fn().mockResolvedValue(undefined),
    startIndexing: vi.fn().mockResolvedValue(undefined),
    stopIndexing: vi.fn().mockResolvedValue(undefined),
    reindexProject: vi.fn().mockResolvedValue(undefined),
    saveEmbeddingModel: vi.fn().mockResolvedValue({}),
    downloadEmbeddingModel: vi.fn().mockResolvedValue({}),
    updateProjectConfig: vi.fn().mockResolvedValue({})
  }
}));
vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn().mockReturnValue(() => {}),
  EventsOnMultiple: vi.fn().mockReturnValue(() => {}),
  EventsOnce: vi.fn().mockReturnValue(() => {}),
  EventsOff: vi.fn(),
  EventsOffAll: vi.fn()
}));

describe('IndexingView.vue', () => {
  const setCurrentProjectMock = vi.fn();
  const navigateToMock = vi.fn();
  const currentProjectRef = ref<any>(null);
  const backendMock = vi.mocked(backend);

  beforeEach(() => {
    currentProjectRef.value = null;
    vi.mocked(useCurrentProject).mockReturnValue({
      currentProject: currentProjectRef,
      loading: ref(false),
      hasCurrentProject: computed(() => false),
      currentProjectId: computed(() => null),
      setCurrentProject: setCurrentProjectMock,
      loadCurrentProject: vi.fn(),
      clearCurrentProject: vi.fn(),
      refreshCurrentProject: vi.fn(),
    });
    vi.mocked(useNavigation).mockReturnValue({
      currentView: ref('indexing'),
      navigateTo: navigateToMock,
    });
    setCurrentProjectMock.mockClear();
    navigateToMock.mockClear();
    vi.clearAllMocks();
    backendMock.startIndexing.mockClear();
    backendMock.stopIndexing.mockClear();
    backendMock.getIndexingProgress.mockClear();
  });

  it('renders no project state', () => {
    const wrapper = mount(IndexingView);
    expect(wrapper.text()).toContain('No Project Selected');
    expect(wrapper.find('button').text()).toContain('Go to Projects');
  });

  it('renders project info when a project is selected', async () => {
    currentProjectRef.value = { id: 'p1', name: 'Test Project', path: '/path/to/p1', createdAt: new Date() };
    const wrapper = mount(IndexingView);
    await flushPromises();

    expect(wrapper.text()).toContain('Test Project');
    expect(wrapper.text()).toContain('Enable continuous indexing');
  });

  it.skip('calls startIndexing when toggle is clicked', async () => {
    const project = { id: 'p1', name: 'Test Project', path: '/path/to/p1', createdAt: new Date() };
    currentProjectRef.value = project;
    const startIndexingSpy = backendMock.startIndexing;

    const wrapper = mount(IndexingView);
    await flushPromises();

    await wrapper.find('.indexing-toggle').trigger('click');
    await flushPromises();
    expect(wrapper.find('.indexing-toggle').classes()).toContain('active');
    expect(startIndexingSpy).toHaveBeenCalledWith(project.path);
  });

  it.skip('shows progress during indexing', async () => {
    currentProjectRef.value = { id: 'p1', name: 'Test Project', path: '/path/to/p1', createdAt: new Date() };
    backendMock.getIndexingProgress.mockResolvedValue({
      totalFiles: 100,
      processedFiles: 25,
      currentFile: 'test.js',
      status: 'indexing',
    });

    const wrapper = mount(IndexingView);
    await flushPromises();

    expect(wrapper.find('.global-progress-meta').text()).toContain('25% complete');
    const meta = wrapper.find('.global-progress-meta');
    expect(meta.text()).toContain('25 / 100 files');
    expect(wrapper.find('.status-line').text()).toContain('Status: INDEXING');
    expect(meta.text()).toContain('Current: test.js');
  });

  it.skip('calls stopIndexing when toggle is clicked off', async () => {
    const project = { id: 'p1', name: 'Test Project', path: '/path/to/p1', createdAt: new Date() };
    currentProjectRef.value = project;
    const stopIndexingSpy = backendMock.stopIndexing;

    const wrapper = mount(IndexingView);
    await flushPromises();

    // Turn on indexing
    await wrapper.find('.indexing-toggle').trigger('click');
    
    // Turn off indexing
    await wrapper.find('.indexing-toggle').trigger('click');

    expect(stopIndexingSpy).toHaveBeenCalled();
  });
});
