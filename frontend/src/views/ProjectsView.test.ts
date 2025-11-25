import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import ProjectsView from './ProjectsView.vue';
import ProjectFormModal from '../components/ProjectFormModal.vue';
import ProjectCard from '../components/ProjectCard.vue';
import ProjectTable from '../components/ProjectTable.vue';
import { backend } from '../api/backend';

// Mock the composables
vi.mock('../composables/useNavigation', () => ({
  useNavigation: () => ({
    navigateTo: vi.fn(),
  }),
}));

vi.mock('../composables/useCurrentProject', () => ({
  useCurrentProject: () => ({
    currentProject: { value: null },
    setCurrentProject: vi.fn(),
    clearCurrentProject: vi.fn(),
  }),
}));

// Mock backend
vi.mock('../api/backend', () => ({
  backend: {
    listProjects: vi.fn(),
    createProject: vi.fn(),
    updateProject: vi.fn(),
    updateProjectConfig: vi.fn(),
    deleteProject: vi.fn(),
    selectDirectory: vi.fn(),
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

describe('ProjectsView.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(backend.listProjects).mockResolvedValue([]);
  });

  it('renders the component with create button', async () => {
    const wrapper = mount(ProjectsView);
    await flushPromises();
    expect(wrapper.find('button.btn-primary').text()).toContain('Create New Project');
  });

  it('shows ProjectFormModal when create button is clicked', async () => {
    const wrapper = mount(ProjectsView);
    await flushPromises();

    expect(wrapper.findComponent(ProjectFormModal).exists()).toBe(false);

    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    expect(wrapper.findComponent(ProjectFormModal).exists()).toBe(true);
  });

  it('renders projects in grid view by default', async () => {
    const projects = [
      { id: 'p1', name: 'Project 1', description: '', createdAt: Date.now() / 1000, updatedAt: Date.now() / 1000, isIndexing: false, config: createMockConfig('/path1'), convertValues: () => {} },
      { id: 'p2', name: 'Project 2', description: '', createdAt: Date.now() / 1000, updatedAt: Date.now() / 1000, isIndexing: false, config: createMockConfig('/path2'), convertValues: () => {} },
    ];
    vi.mocked(backend.listProjects).mockResolvedValue(projects);

    const wrapper = mount(ProjectsView);
    await flushPromises();

    expect(wrapper.findAllComponents(ProjectCard).length).toBe(2);
    expect(wrapper.findComponent(ProjectTable).exists()).toBe(false);
  });

  it('switches to table view when toggle is clicked', async () => {
    const projects = [
      { id: 'p1', name: 'Project 1', description: '', createdAt: Date.now() / 1000, updatedAt: Date.now() / 1000, isIndexing: false, config: createMockConfig('/path1'), convertValues: () => {} },
    ];
    vi.mocked(backend.listProjects).mockResolvedValue(projects);

    const wrapper = mount(ProjectsView);
    await flushPromises();

    // Find table view toggle button (second toggle button)
    const toggleButtons = wrapper.findAll('.toggle-btn');
    await toggleButtons[1].trigger('click');
    await flushPromises();

    expect(wrapper.findComponent(ProjectTable).exists()).toBe(true);
    expect(wrapper.findAllComponents(ProjectCard).length).toBe(0);
  });

  it('opens edit form when edit is triggered from ProjectCard', async () => {
    const projects = [
      { id: 'p1', name: 'Project 1', description: '', createdAt: Date.now() / 1000, updatedAt: Date.now() / 1000, isIndexing: false, config: createMockConfig('/path1'), convertValues: () => {} },
    ];
    vi.mocked(backend.listProjects).mockResolvedValue(projects);

    const wrapper = mount(ProjectsView);
    await flushPromises();

    const projectCard = wrapper.findComponent(ProjectCard);
    projectCard.vm.$emit('edit', projects[0]);
    await flushPromises();

    expect(wrapper.findComponent(ProjectFormModal).exists()).toBe(true);
    expect(wrapper.findComponent(ProjectFormModal).props('project')).toEqual(projects[0]);
  });

  it('handles project save from modal', async () => {
    const newProject = {
      id: 'new-p',
      name: 'New Project',
      description: '',
      createdAt: Date.now() / 1000,
      updatedAt: Date.now() / 1000,
      isIndexing: false,
      config: createMockConfig('/new-path'),
      convertValues: () => {}
    };

    const wrapper = mount(ProjectsView);
    await flushPromises();

    // Open form
    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    // Simulate save
    const modal = wrapper.findComponent(ProjectFormModal);
    modal.vm.$emit('save', newProject);
    await flushPromises();

    // Modal should be closed
    expect(wrapper.findComponent(ProjectFormModal).exists()).toBe(false);
  });

  it('shows empty state when no projects exist', async () => {
    const wrapper = mount(ProjectsView);
    await flushPromises();

    expect(wrapper.text()).toContain('No Projects Yet');
  });
});
