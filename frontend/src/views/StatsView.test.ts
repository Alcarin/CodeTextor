import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref } from 'vue';
import StatsView from './StatsView.vue';
import { mockBackend } from '../services/mockBackend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');

describe('StatsView.vue', () => {
  const currentProjectRef = ref<any>(null);

  beforeEach(() => {
    currentProjectRef.value = { id: 'p1', name: 'Test Project', path: '/root' };
    vi.mocked(useCurrentProject).mockReturnValue({
      currentProject: currentProjectRef,
      setCurrentProject: vi.fn(),
      loadCurrentProject: vi.fn(),
      clearCurrentProject: vi.fn(),
    });
    vi.clearAllMocks();
  });

  const mountComponent = () => mount(StatsView);

  it('renders the component and loads initial data', async () => {
    const getStatsSpy = vi.spyOn(mockBackend, 'getProjectStats').mockResolvedValue({ totalFiles: 100, totalChunks: 1000, totalSymbols: 5000, indexSize: 1234567 });

    const wrapper = mountComponent();
    await flushPromises();

    expect(getStatsSpy).toHaveBeenCalled();
    expect(wrapper.text()).toContain('Total Files');
  });

  it('displays project stats correctly', async () => {
    vi.spyOn(mockBackend, 'getProjectStats').mockResolvedValue({ totalFiles: 123, totalChunks: 1234, totalSymbols: 5678, indexSize: 1234567 });

    const wrapper = mountComponent();
    await flushPromises();

    expect(wrapper.text()).toContain('123');
    expect(wrapper.text()).toContain('1,234');
    expect(wrapper.text()).toContain('5,678');
    expect(wrapper.text()).toContain('1.18 MB');
  });

  it('shows a loading indicator while fetching', async () => {
    vi.spyOn(mockBackend, 'getProjectStats').mockReturnValue(new Promise(() => {})); // Never resolves

    const wrapper = mountComponent();
    await flushPromises();

    expect(wrapper.text()).toContain('Loading statistics...');
  });
});
