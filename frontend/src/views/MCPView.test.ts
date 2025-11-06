import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import MCPView from './MCPView.vue';
import { mockBackend } from '../services/mockBackend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');

describe('MCPView.vue', () => {
  const currentProjectRef = ref<any>(null);

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
  });

  const mountComponent = () => mount(MCPView);

  it('renders the component and loads initial data', async () => {
    const getConfigSpy = vi.spyOn(mockBackend, 'getMCPConfig').mockResolvedValue({ host: 'localhost', port: 3000, protocol: 'http', autoStart: false, maxConnections: 10 });
    const getStatusSpy = vi.spyOn(mockBackend, 'getMCPStatus').mockResolvedValue({ isRunning: false, uptime: 0, activeConnections: 0, totalRequests: 0, averageResponseTime: 0 });
    const getToolsSpy = vi.spyOn(mockBackend, 'getMCPTools').mockResolvedValue([]);

    const wrapper = mountComponent();
    await flushPromises();

    expect(getConfigSpy).toHaveBeenCalled();
    expect(getStatusSpy).toHaveBeenCalled();
    expect(getToolsSpy).toHaveBeenCalled();
    expect(wrapper.text()).toContain('Server Status');
  });

  it('starts the server when the start button is clicked', async () => {
    vi.spyOn(mockBackend, 'getMCPConfig').mockResolvedValue({ host: 'localhost', port: 3000, protocol: 'http', autoStart: false, maxConnections: 10 });
    vi.spyOn(mockBackend, 'getMCPStatus').mockResolvedValue({ isRunning: false, uptime: 0, activeConnections: 0, totalRequests: 0, averageResponseTime: 0 });
    vi.spyOn(mockBackend, 'getMCPTools').mockResolvedValue([]);
    const startServerSpy = vi.spyOn(mockBackend, 'startMCPServer').mockResolvedValue();

    const wrapper = mountComponent();
    await flushPromises();

    await wrapper.find('button.btn-success').trigger('click');
    await flushPromises();

    expect(startServerSpy).toHaveBeenCalled();
  });

  it('stops the server when the stop button is clicked', async () => {
    vi.spyOn(mockBackend, 'getMCPConfig').mockResolvedValue({ host: 'localhost', port: 3000, protocol: 'http', autoStart: false, maxConnections: 10 });
    vi.spyOn(mockBackend, 'getMCPStatus').mockResolvedValue({ isRunning: true, uptime: 120, activeConnections: 1, totalRequests: 10, averageResponseTime: 50 });
    vi.spyOn(mockBackend, 'getMCPTools').mockResolvedValue([]);
    const stopServerSpy = vi.spyOn(mockBackend, 'stopMCPServer').mockResolvedValue();

    const wrapper = mountComponent();
    await flushPromises();

    await wrapper.find('button.btn-danger').trigger('click');
    await flushPromises();

    expect(stopServerSpy).toHaveBeenCalled();
  });

  it('displays server status correctly', async () => {
    vi.spyOn(mockBackend, 'getMCPConfig').mockResolvedValue({ host: 'localhost', port: 3000, protocol: 'http', autoStart: false, maxConnections: 10 });
    vi.spyOn(mockBackend, 'getMCPStatus').mockResolvedValue({ isRunning: true, uptime: 123, activeConnections: 2, totalRequests: 42, averageResponseTime: 55 });
    vi.spyOn(mockBackend, 'getMCPTools').mockResolvedValue([]);

    const wrapper = mountComponent();
    await flushPromises();

    expect(wrapper.text()).toContain('Running');
    expect(wrapper.text()).toContain('2m 3s');
    expect(wrapper.find('.status-grid').text()).toContain('2');
    expect(wrapper.find('.status-grid').text()).toContain('42');
    expect(wrapper.find('.status-grid').text()).toContain('55.0ms');
  });
});
