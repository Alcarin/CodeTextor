import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import OutlineView from './OutlineView.vue';
import { mockBackend } from '../services/mockBackend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');

const OutlineTreeNode = {
  name: 'OutlineTreeNode',
  props: ['node'],
  template: '<div class="mock-outline-node">{{ node.name }}</div>'
};

describe('OutlineView.vue', () => {
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

  const mountComponent = () => mount(OutlineView, {
    global: {
      stubs: {
        OutlineTreeNode,
      }
    }
  });

  it('renders initial state with no file selected', () => {
    const wrapper = mountComponent();
    expect(wrapper.text()).toContain('Enter a file path above to view its structural outline');
  });

  it('fetches and displays outline on file path change', async () => {
    const getOutlineSpy = vi.spyOn(mockBackend, 'getOutline').mockResolvedValue([
      { id: 'n1', name: 'Node 1', kind: 'class', startLine: 1, endLine: 10, children: [] },
      { id: 'n2', name: 'Node 2', kind: 'function', startLine: 12, endLine: 20, children: [] },
    ]);

    const wrapper = mountComponent();
    await wrapper.find('input[type="text"]').setValue('src/main.ts');
    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    expect(getOutlineSpy).toHaveBeenCalledWith({ projectId: 'p1', path: 'src/main.ts', depth: 2 });
    const nodes = wrapper.findAll('.mock-outline-node');
    expect(nodes.length).toBe(2);
    expect(nodes[0].text()).toBe('Node 1');
    expect(nodes[1].text()).toBe('Node 2');
  });

  it('shows a loading indicator while fetching', async () => {
    vi.spyOn(mockBackend, 'getOutline').mockReturnValue(new Promise(() => {})); // Promise that never resolves

    const wrapper = mountComponent();
    await wrapper.find('input[type="text"]').setValue('src/main.ts');
    await wrapper.find('button.btn-primary').trigger('click');

    expect(wrapper.find('button.btn-primary').text()).toContain('Loading...');
  });

  it('shows an error message on fetch failure', async () => {
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
    vi.spyOn(mockBackend, 'getOutline').mockRejectedValue(new Error('File not found'));

    const wrapper = mountComponent();
    await wrapper.find('input[type="text"]').setValue('src/main.ts');
    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    expect(alertSpy).toHaveBeenCalledWith('Failed to fetch outline: File not found');
    alertSpy.mockRestore();
  });
});
