import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import ProjectSelector from './ProjectSelector.vue';
import { backend } from '../api/backend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');
vi.mock('../api/backend', () => ({
  backend: {
    listProjects: vi.fn(),
  },
}));

// Helper to create mock project config
const createMockConfig = (rootPath: string = '/test/path') => ({
  rootPath,
  includePaths: [],
  excludePatterns: [],
  fileExtensions: ['.ts', '.js', '.vue'],
  autoExcludeHidden: true,
  continuousIndexing: false,
  chunkSizeMin: 100,
  chunkSizeMax: 500,
  embeddingModel: 'default',
  maxResponseBytes: 1000000,
  convertValues: () => {},
});

describe('ProjectSelector.vue', () => {
  const setCurrentProjectMock = vi.fn();

  beforeEach(() => {
    vi.mocked(useCurrentProject).mockReturnValue({
      currentProject: ref(null),
      loading: ref(false),
      hasCurrentProject: computed(() => false),
      currentProjectId: computed(() => null),
      setCurrentProject: setCurrentProjectMock,
      loadCurrentProject: vi.fn(),
      clearCurrentProject: vi.fn(),
      refreshCurrentProject: vi.fn(),
    });
    setCurrentProjectMock.mockClear();
    vi.mocked(backend.listProjects).mockResolvedValue([]);
  });

  it('renders with no project selected', () => {
    const wrapper = mount(ProjectSelector);
    expect(wrapper.text()).toContain('No Project');
  });

  it('opens dropdown on click', async () => {
    const wrapper = mount(ProjectSelector);
    await wrapper.find('.selector-title').trigger('click');
    expect(wrapper.find('.dropdown-menu').exists()).toBe(true);
  });

  it('lists projects in the dropdown', async () => {
    const projects = [
      { id: 'p1', name: 'Project 1', description: '', createdAt: Date.now() / 1000, updatedAt: Date.now() / 1000, isIndexing: false, config: createMockConfig('/p1'), convertValues: () => {} },
      { id: 'p2', name: 'Project 2', description: '', createdAt: Date.now() / 1000, updatedAt: Date.now() / 1000, isIndexing: false, config: createMockConfig('/p2'), convertValues: () => {} },
    ];
    vi.mocked(backend.listProjects).mockResolvedValue(projects);

    const wrapper = mount(ProjectSelector);
    await flushPromises();
    await wrapper.find('.selector-title').trigger('click');
    await flushPromises();

    const items = wrapper.findAll('.dropdown-item');
    expect(items.length).toBeGreaterThanOrEqual(2);
    expect(wrapper.text()).toContain('Project 1');
    expect(wrapper.text()).toContain('Project 2');
  });

  it('calls setCurrentProject when a project is selected', async () => {
    const projects = [{ id: 'p1', name: 'Project 1', description: '', createdAt: Date.now() / 1000, updatedAt: Date.now() / 1000, isIndexing: false, config: createMockConfig('/p1'), convertValues: () => {} }];
    vi.mocked(backend.listProjects).mockResolvedValue(projects);

    const wrapper = mount(ProjectSelector);
    await flushPromises();
    await wrapper.find('.selector-title').trigger('click');
    await flushPromises();

    // Click the second dropdown-item (first is "View All", second is the project)
    const items = wrapper.findAll('.dropdown-item');
    await items[1].trigger('click');
    expect(setCurrentProjectMock).toHaveBeenCalledWith(projects[0]);
  });
});
