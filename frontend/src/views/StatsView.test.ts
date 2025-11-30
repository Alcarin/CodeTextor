import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import StatsView from './StatsView.vue';
import { backend } from '../api/backend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');
vi.mock('../api/backend', () => ({
  backend: {
    getProjectStats: vi.fn()
  }
}));

describe('StatsView.vue', () => {
  const currentProjectRef = ref<any>(null);
  const backendMock = vi.mocked(backend);

  beforeEach(() => {
    currentProjectRef.value = { id: 'p1', name: 'Test Project', path: '/root' };
    vi.mocked(useCurrentProject).mockReturnValue({
      currentProject: currentProjectRef,
      loading: ref(false),
      hasCurrentProject: computed(() => false),
      currentProjectId: computed(() => null),
      setCurrentProject: vi.fn(),
      loadCurrentProject: vi.fn(),
      clearCurrentProject: vi.fn(),
      refreshCurrentProject: vi.fn(),
    });
    vi.clearAllMocks();
    backendMock.getProjectStats.mockReset();
  });

  const mountComponent = () => mount(StatsView);

  it('renders the component and loads initial data', async () => {
    const getStatsSpy = backendMock.getProjectStats.mockResolvedValue({
      totalFiles: 100,
      totalChunks: 1000,
      totalSymbols: 5000,
      databaseSize: 1234567,
      isIndexing: false,
      indexingProgress: 0,
      convertValues: (a: any) => a
    });

    const wrapper = mountComponent();
    await flushPromises();

    expect(getStatsSpy).toHaveBeenCalled();
    expect(wrapper.text()).toContain('Total Files');
  });

  it('displays project stats correctly', async () => {
    backendMock.getProjectStats.mockResolvedValue({
      totalFiles: 123,
      totalChunks: 1234,
      totalSymbols: 5678,
      databaseSize: 1234567,
      isIndexing: false,
      indexingProgress: 0,
      convertValues: (a: any) => a
    });

    const wrapper = mountComponent();
    await flushPromises();

    expect(wrapper.text()).toContain('123');
    expect(wrapper.text()).toContain('1,234');
    expect(wrapper.text()).toContain('5,678');
    expect(wrapper.text()).toContain('1.18 MB');
  });

  it('shows a loading indicator while fetching', async () => {
    backendMock.getProjectStats.mockReturnValue(new Promise(() => {})); // Never resolves

    const wrapper = mountComponent();
    await flushPromises();

    expect(wrapper.text()).toContain('Loading statistics...');
  });
});
