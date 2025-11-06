import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import ProjectSelector from './ProjectSelector.vue';
import { mockBackend } from '../services/mockBackend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');

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
  });

  it('renders with no project selected', () => {
    const wrapper = mount(ProjectSelector);
    expect(wrapper.text()).toContain('No Project');
  });

  it('opens dropdown on click', async () => {
    const wrapper = mount(ProjectSelector);
    await wrapper.find('.selector-button').trigger('click');
    expect(wrapper.find('.dropdown-menu').exists()).toBe(true);
  });

  it('lists projects in the dropdown', async () => {
    const projects = [
      { id: 'p1', name: 'Project 1', path: '/p1', createdAt: new Date() },
      { id: 'p2', name: 'Project 2', path: '/p2', createdAt: new Date() },
    ];
    vi.spyOn(mockBackend, 'listProjects').mockResolvedValue({ projects, currentProjectId: undefined });

    const wrapper = mount(ProjectSelector);
    await wrapper.find('.selector-button').trigger('click');
    await flushPromises();

    const items = wrapper.findAll('.dropdown-item');
    expect(items.length).toBe(2);
    expect(items[0].text()).toContain('Project 1');
    expect(items[1].text()).toContain('Project 2');
  });

  it('calls setCurrentProject when a project is selected', async () => {
    const projects = [{ id: 'p1', name: 'Project 1', path: '/p1', createdAt: new Date() }];
    vi.spyOn(mockBackend, 'listProjects').mockResolvedValue({ projects, currentProjectId: undefined });

    const wrapper = mount(ProjectSelector);
    await wrapper.find('.selector-button').trigger('click');
    await flushPromises();

    await wrapper.find('.dropdown-item').trigger('click');
    expect(setCurrentProjectMock).toHaveBeenCalledWith(projects[0]);
  });
});
