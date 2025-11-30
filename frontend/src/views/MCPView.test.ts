import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import MCPView from './MCPView.vue';
import { backend } from '../api/backend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');
vi.mock('../api/backend', () => ({
  backend: {
    getMCPConfig: vi.fn(),
    updateMCPConfig: vi.fn(),
    startMCPServer: vi.fn(),
    stopMCPServer: vi.fn(),
    getMCPStatus: vi.fn(),
    getMCPTools: vi.fn(),
    toggleMCPTool: vi.fn()
  }
}));
vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn().mockReturnValue(() => {})
}));

describe('MCPView.vue', () => {
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
    backendMock.getMCPConfig.mockReset();
    backendMock.getMCPStatus.mockReset();
    backendMock.getMCPTools.mockReset();
    backendMock.startMCPServer.mockReset();
    backendMock.stopMCPServer.mockReset();
  });

  const mountComponent = () => mount(MCPView);

  it('renders the component and loads initial data', async () => {
    const getConfigSpy = backendMock.getMCPConfig.mockResolvedValue({
      host: 'localhost',
      port: 3000,
      protocol: 'http',
      autoStart: false,
      maxConnections: 10
    });
    const getStatusSpy = backendMock.getMCPStatus.mockResolvedValue({
      isRunning: false,
      uptime: 0,
      activeConnections: 0,
      totalRequests: 0,
      averageResponseTime: 0
    });
    const getToolsSpy = backendMock.getMCPTools.mockResolvedValue([]);

    const wrapper = mountComponent();
    await flushPromises();

    expect(getConfigSpy).toHaveBeenCalled();
    expect(getStatusSpy).toHaveBeenCalled();
    expect(getToolsSpy).toHaveBeenCalled();
    expect(wrapper.text()).toContain('Server Status');
  });

  it('starts the server when the start button is clicked', async () => {
    backendMock.getMCPConfig.mockResolvedValue({
      host: 'localhost',
      port: 3000,
      protocol: 'http',
      autoStart: false,
      maxConnections: 10
    });
    backendMock.getMCPStatus.mockResolvedValue({
      isRunning: false,
      uptime: 0,
      activeConnections: 0,
      totalRequests: 0,
      averageResponseTime: 0
    });
    backendMock.getMCPTools.mockResolvedValue([]);
    const startServerSpy = backendMock.startMCPServer.mockResolvedValue();

    const wrapper = mountComponent();
    await flushPromises();

    await wrapper.find('button.btn-success').trigger('click');
    await flushPromises();

    expect(startServerSpy).toHaveBeenCalled();
  });

  it('stops the server when the stop button is clicked', async () => {
    backendMock.getMCPConfig.mockResolvedValue({
      host: 'localhost',
      port: 3000,
      protocol: 'http',
      autoStart: false,
      maxConnections: 10
    });
    backendMock.getMCPStatus
      .mockResolvedValue({
        isRunning: true,
        uptime: 120,
        activeConnections: 1,
        totalRequests: 10,
        averageResponseTime: 50
      });
    backendMock.getMCPTools.mockResolvedValue([]);
    const stopServerSpy = backendMock.stopMCPServer.mockResolvedValue();

    const wrapper = mountComponent();
    await flushPromises();

    await wrapper.find('button.btn-danger').trigger('click');
    await flushPromises();

    expect(stopServerSpy).toHaveBeenCalled();
  });

  it('displays server status correctly', async () => {
    backendMock.getMCPConfig.mockResolvedValue({
      host: 'localhost',
      port: 3000,
      protocol: 'http',
      autoStart: false,
      maxConnections: 10
    });
    backendMock.getMCPStatus.mockResolvedValue({
      isRunning: true,
      uptime: 123,
      activeConnections: 2,
      totalRequests: 42,
      averageResponseTime: 55
    });
    backendMock.getMCPTools.mockResolvedValue([]);

    const wrapper = mountComponent();
    await flushPromises();

    expect(wrapper.text()).toContain('Running');
    expect(wrapper.text()).toContain('2m 3s');
    expect(wrapper.find('.status-grid').text()).toContain('2');
    expect(wrapper.find('.status-grid').text()).toContain('42');
    expect(wrapper.find('.status-grid').text()).toContain('55.0ms');
  });
});
