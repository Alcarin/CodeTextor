import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import IndexingView from './IndexingView.vue';
import { mockBackend } from '../services/mockBackend';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';

vi.mock('../composables/useCurrentProject');
vi.mock('../composables/useNavigation');

describe('IndexingView.vue', () => {
  const setCurrentProjectMock = vi.fn();
  const navigateToMock = vi.fn();
  const currentProjectRef = ref<any>(null);

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
    const startIndexingSpy = vi.spyOn(mockBackend, 'startIndexing');

    const wrapper = mount(IndexingView);
    await flushPromises();

    await wrapper.find('.indexing-toggle').trigger('click');
    await flushPromises();
    expect(wrapper.find('.indexing-toggle').classes()).toContain('active');
    expect(startIndexingSpy).toHaveBeenCalledWith(project.path);
  });

  it('shows progress during indexing', async () => {
    currentProjectRef.value = { id: 'p1', name: 'Test Project', path: '/path/to/p1', createdAt: new Date() };
    vi.spyOn(mockBackend, 'getIndexingProgress').mockResolvedValue({
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

  it('calls stopIndexing when toggle is clicked off', async () => {
    const project = { id: 'p1', name: 'Test Project', path: '/path/to/p1', createdAt: new Date() };
    currentProjectRef.value = project;
    const stopIndexingSpy = vi.spyOn(mockBackend, 'stopIndexing');

    const wrapper = mount(IndexingView);
    await flushPromises();

    // Turn on indexing
    await wrapper.find('.indexing-toggle').trigger('click');
    
    // Turn off indexing
    await wrapper.find('.indexing-toggle').trigger('click');

    expect(stopIndexingSpy).toHaveBeenCalled();
  });
});
