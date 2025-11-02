import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import ProjectsView from './ProjectsView.vue';
import { mockBackend } from '../services/mockBackend';

// Mock the navigation composable
vi.mock('../composables/useNavigation', () => ({
  useNavigation: () => ({
    navigateTo: vi.fn(),
  }),
}));

describe('ProjectsView.vue', () => {
  it('renders the component', () => {
    const wrapper = mount(ProjectsView);
    expect(wrapper.find('button').text()).toContain('Create New Project');
  });

  it('opens the create project form', async () => {
    const wrapper = mount(ProjectsView);
    await wrapper.find('button.btn-primary').trigger('click');
    expect(wrapper.html()).toContain('Create New Project');
  });

  it('slugifies the project name to generate project ID on create mode', async () => {
    const wrapper = mount(ProjectsView);
    await wrapper.find('button.btn-primary').trigger('click');

    const nameInput = wrapper.find('#project-name');
    await nameInput.setValue('My Awesome Project');

    const idInput = wrapper.find('#project-id');
    expect((idInput.element as HTMLInputElement).value).toBe('my-awesome-project');
  });

  it('does not slugify project name when editing', async () => {
    // Mock listProjects to return a project
    const project = { id: 'existing-project', name: 'Existing Project', path: '', createdAt: new Date() };
    vi.spyOn(mockBackend, 'listProjects').mockResolvedValue({ projects: [project] });

    const wrapper = mount(ProjectsView);
    await flushPromises();

    await wrapper.find('[data-testid="edit-project-button"]').trigger('click'); // Click Edit

    const nameInput = wrapper.find('#project-name');
    await nameInput.setValue('A New Name');

    const idInput = wrapper.find('#project-id');
    expect((idInput.element as HTMLInputElement).value).toBe('existing-project');
  });

  it('creates a new project', async () => {
    const createProjectSpy = vi.spyOn(mockBackend, 'createProject');
    const wrapper = mount(ProjectsView);

    await wrapper.find('button.btn-primary').trigger('click');

    await wrapper.find('#project-name').setValue('Test Project');
    await wrapper.find('#project-description').setValue('A test description');

    await wrapper.find('button.btn-success').trigger('click');

    expect(createProjectSpy).toHaveBeenCalledWith({
      id: 'test-project',
      name: 'Test Project',
      path: '/path/to/test-project',
      description: 'A test description',
    });
  });

  it('updates an existing project', async () => {
    const project = { id: 'p1', name: 'Project 1', path: '', createdAt: new Date(), description: '' };
    vi.spyOn(mockBackend, 'listProjects').mockResolvedValue({ projects: [project] });
    const updateProjectSpy = vi.spyOn(mockBackend, 'updateProject');

    const wrapper = mount(ProjectsView);
    await flushPromises();

    await wrapper.find('[data-testid="edit-project-button"]').trigger('click'); // Edit button

    await wrapper.find('#project-name').setValue('Updated Name');
    await wrapper.find('#project-description').setValue('Updated description');

    await wrapper.find('button.btn-success').trigger('click');

    expect(updateProjectSpy).toHaveBeenCalledWith('p1', {
      name: 'Updated Name',
      description: 'Updated description',
    });
  });
});
