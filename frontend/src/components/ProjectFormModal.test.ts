import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import ProjectFormModal from './ProjectFormModal.vue';
import { backend } from '../api/backend';

// Mock backend
vi.mock('../api/backend', () => ({
  backend: {
    createProject: vi.fn(),
    updateProject: vi.fn(),
    updateProjectConfig: vi.fn(),
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
});

describe('ProjectFormModal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders create mode when no project is provided', () => {
    const wrapper = mount(ProjectFormModal);
    expect(wrapper.text()).toContain('Create New Project');
  });

  it('renders edit mode when project is provided', () => {
    const project = {
      id: 'test-id',
      name: 'Test Project',
      description: 'Test description',
      createdAt: Date.now() / 1000,
      updatedAt: Date.now() / 1000,
      isIndexing: false,
      config: createMockConfig('/test/path'),
      convertValues: () => {}
    };
    const wrapper = mount(ProjectFormModal, {
      props: { project }
    });
    expect(wrapper.text()).toContain('Edit Project');
  });

  it('auto-generates slug from name in create mode', async () => {
    const wrapper = mount(ProjectFormModal);

    const nameInput = wrapper.find('#project-name');
    await nameInput.setValue('My Awesome Project');
    await flushPromises();

    const slugInput = wrapper.find('#project-slug');
    expect((slugInput.element as HTMLInputElement).value).toBe('my-awesome-project');
  });

  it('does not auto-generate slug in edit mode', async () => {
    const project = {
      id: 'existing-id',
      name: 'Existing Project',
      description: '',
      createdAt: Date.now() / 1000,
      updatedAt: Date.now() / 1000,
      isIndexing: false,
      config: createMockConfig('/existing/path'),
      convertValues: () => {}
    };
    const wrapper = mount(ProjectFormModal, {
      props: { project }
    });
    await flushPromises();

    const nameInput = wrapper.find('#project-name');
    await nameInput.setValue('New Name');
    await flushPromises();

    const slugInput = wrapper.find('#project-slug');
    expect((slugInput.element as HTMLInputElement).value).toBe('existing-id');
  });

  it('emits cancel when cancel button is clicked', async () => {
    const wrapper = mount(ProjectFormModal);

    await wrapper.find('.modal-footer .btn-secondary').trigger('click');

    expect(wrapper.emitted('cancel')).toBeTruthy();
  });

  it('emits cancel when overlay is clicked', async () => {
    const wrapper = mount(ProjectFormModal);

    await wrapper.find('.modal-overlay').trigger('click');

    expect(wrapper.emitted('cancel')).toBeTruthy();
  });

  it('creates new project and emits save', async () => {
    const newProject = {
      id: 'new-project',
      name: 'New Project',
      description: 'New description',
      createdAt: Date.now() / 1000,
      updatedAt: Date.now() / 1000,
      isIndexing: false,
      config: createMockConfig('/new/path'),
      convertValues: () => {}
    };
    vi.mocked(backend.createProject).mockResolvedValue(newProject);
    vi.mocked(backend.selectDirectory).mockResolvedValue('/new/path');

    const wrapper = mount(ProjectFormModal);

    await wrapper.find('#project-name').setValue('New Project');
    await wrapper.find('#project-description').setValue('New description');

    // Simulate directory selection
    await wrapper.find('.root-selector .btn-secondary').trigger('click');
    await flushPromises();

    await wrapper.find('.modal-footer .btn-success').trigger('click');
    await flushPromises();

    expect(backend.createProject).toHaveBeenCalledWith(
      'New Project',
      'New description',
      'new-project',
      '/new/path'
    );
    expect(wrapper.emitted('save')).toBeTruthy();
    expect(wrapper.emitted('save')?.[0]).toEqual([newProject]);
  });

  it('updates existing project and emits save', async () => {
    const project = {
      id: 'existing-id',
      name: 'Existing Project',
      description: 'Old description',
      createdAt: Date.now() / 1000,
      updatedAt: Date.now() / 1000,
      isIndexing: false,
      config: createMockConfig('/old/path'),
      convertValues: () => {}
    };
    const updatedMeta = { ...project, name: 'Updated Project', description: 'Updated description' };
    const updatedProject = { ...updatedMeta, config: createMockConfig('/new/path') };

    vi.mocked(backend.updateProject).mockResolvedValue(updatedMeta);
    vi.mocked(backend.updateProjectConfig).mockResolvedValue(updatedProject);
    vi.mocked(backend.selectDirectory).mockResolvedValue('/new/path');

    const wrapper = mount(ProjectFormModal, {
      props: { project }
    });
    await flushPromises();

    await wrapper.find('#project-name').setValue('Updated Project');
    await wrapper.find('#project-description').setValue('Updated description');

    // Simulate directory selection
    await wrapper.find('.root-selector .btn-secondary').trigger('click');
    await flushPromises();

    await wrapper.find('.modal-footer .btn-success').trigger('click');
    await flushPromises();

    expect(backend.updateProject).toHaveBeenCalledWith(
      'existing-id',
      'Updated Project',
      'Updated description'
    );
    expect(backend.updateProjectConfig).toHaveBeenCalledWith(
      'existing-id',
      expect.objectContaining({ rootPath: '/new/path' })
    );
    expect(wrapper.emitted('save')).toBeTruthy();
  });

  it('disables slug input in edit mode', async () => {
    const project = {
      id: 'test-id',
      name: 'Test Project',
      description: '',
      createdAt: Date.now() / 1000,
      updatedAt: Date.now() / 1000,
      isIndexing: false,
      config: createMockConfig('/test/path'),
      convertValues: () => {}
    };
    const wrapper = mount(ProjectFormModal, {
      props: { project }
    });

    const slugInput = wrapper.find('#project-slug');
    expect((slugInput.element as HTMLInputElement).disabled).toBe(true);
  });

  it('disables submit button when name is empty', async () => {
    const wrapper = mount(ProjectFormModal);

    const submitButton = wrapper.find('.modal-footer .btn-success');
    expect((submitButton.element as HTMLButtonElement).disabled).toBe(true);
    expect(backend.createProject).not.toHaveBeenCalled();
  });

  it('shows validation error when root path is empty', async () => {
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});

    const wrapper = mount(ProjectFormModal);

    await wrapper.find('#project-name').setValue('Test Project');
    await wrapper.find('.btn-success').trigger('click');
    await flushPromises();

    expect(alertSpy).toHaveBeenCalledWith('Please select a project root folder');
    expect(backend.createProject).not.toHaveBeenCalled();
  });
});
