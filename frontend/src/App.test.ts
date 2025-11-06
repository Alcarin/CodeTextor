import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import App from './App.vue';
import { useCurrentProject } from './composables/useCurrentProject';
import { useNavigation } from './composables/useNavigation';

// Mock composables
vi.mock('./composables/useCurrentProject');
vi.mock('./composables/useNavigation');

// Mock child components to avoid rendering them
const ProjectSelector = {
  template: '<div data-testid="project-selector-mock"></div>'
};
const ProjectsView = { template: '<div data-testid="projects-view-mock"></div>' };
const IndexingView = { template: '<div data-testid="indexing-view-mock"></div>' };

describe('App.vue', () => {
  const currentProjectRef = ref<any>(null);
  const currentViewRef = ref<any>('projects');
  const navigateToMock = vi.fn((view) => { currentViewRef.value = view; });

  beforeEach(() => {
    currentProjectRef.value = null;
    currentViewRef.value = 'projects';
    vi.mocked(useCurrentProject).mockReturnValue({
      currentProject: currentProjectRef,
      loading: ref(false),
      hasCurrentProject: computed(() => false),
      currentProjectId: computed(() => null),
      loadCurrentProject: vi.fn(),
      setCurrentProject: vi.fn(),
      clearCurrentProject: vi.fn(),
      refreshCurrentProject: vi.fn(),
    });
    vi.mocked(useNavigation).mockReturnValue({
      currentView: currentViewRef,
      navigateTo: navigateToMock,
    });
    navigateToMock.mockClear();
  });

  const mountComponent = () => mount(App, {
    global: {
      stubs: {
        ProjectSelector,
        ProjectsView,
        IndexingView,
        // Stub other views as well to keep tests clean
        SearchView: { template: '<div>Search</div>' },
        OutlineView: { template: '<div>Outline</div>' },
        StatsView: { template: '<div>Stats</div>' },
        MCPView: { template: '<div>MCP</div>' },
      }
    }
  });

  it('renders the main layout', () => {
    const wrapper = mountComponent();
    expect(wrapper.find('.app-nav').exists()).toBe(true);
    expect(wrapper.find('.app-main').exists()).toBe(true);
    expect(wrapper.find('.app-footer').exists()).toBe(true);
    expect(wrapper.text()).toContain('CodeTextor');
  });

  it('renders ProjectsView by default', () => {
    const wrapper = mountComponent();
    expect(wrapper.find('[data-testid="projects-view-mock"]').exists()).toBe(true);
  });

  it('disables navigation when no project is selected', async () => {
    const wrapper = mountComponent();
    const searchButton = wrapper.findAll('button').find(b => b.text().includes('Search'));
    expect(searchButton).toBeDefined();
    expect(searchButton!.attributes('disabled')).toBeDefined();

    const indexingButton = wrapper.findAll('button').find(b => b.text().includes('Indexing'));
    expect(indexingButton).toBeDefined();
    expect(indexingButton!.attributes('disabled')).toBeDefined();
  });

  it('enables navigation when a project is selected', async () => {
    currentProjectRef.value = { id: 'p1', name: 'Test Project' };
    const wrapper = mountComponent();
    await flushPromises();

    const searchButton = wrapper.findAll('button').find(b => b.text().includes('Search'));
    expect(searchButton).toBeDefined();
    expect(searchButton!.attributes('disabled')).toBeUndefined();

    const indexingButton = wrapper.findAll('button').find(b => b.text().includes('Indexing'));
    expect(indexingButton).toBeDefined();
    expect(indexingButton!.attributes('disabled')).toBeUndefined();
  });

  it('navigates to a different view on button click', async () => {
    currentProjectRef.value = { id: 'p1', name: 'Test Project' };
    const wrapper = mountComponent();
    await flushPromises();

    currentViewRef.value = 'indexing';
    await flushPromises();

    expect(wrapper.find('[data-testid="indexing-view-mock"]').exists()).toBe(true);
  });
});
