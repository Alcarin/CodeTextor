import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import OutlineView from './OutlineView.vue';
import { backend } from '../api/backend';
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
    vi.clearAllMocks();
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
    vi.spyOn(backend, 'getFilePreviews').mockResolvedValue([
      { absolutePath: '/root/main.ts', relativePath: 'main.ts', extension: '.ts', size: '1 KB', hidden: false, lastModified: Date.now() / 1000 },
    ]);
  });

  const mountComponent = () => mount(OutlineView, {
    global: {
      stubs: {
        OutlineTreeNode,
      }
    }
  });

  it('renders the directory tree once previews load', async () => {
    const wrapper = mountComponent();
    await flushPromises();
    expect(wrapper.text()).toContain('main.ts');
  });

  it('fetches and displays outline when a file is expanded', async () => {
    const getOutlineSpy = vi.spyOn(backend, 'getFileOutline').mockResolvedValue([
      { id: 'n1', name: 'Node 1', kind: 'class', filePath: 'main.ts', startLine: 1, endLine: 10, children: [], convertValues: () => {} } as any,
      { id: 'n2', name: 'Node 2', kind: 'function', filePath: 'main.ts', startLine: 12, endLine: 20, children: [], convertValues: () => {} } as any,
    ]);

    const wrapper = mountComponent();
    await flushPromises();
    await wrapper.find('[data-testid="file-tree-toggle"]').trigger('click');
    await flushPromises();

    expect(getOutlineSpy).toHaveBeenCalledWith('p1', 'main.ts', 2);
    const nodes = wrapper.findAll('.mock-outline-node');
    expect(nodes.length).toBe(2);
    expect(nodes[0].text()).toBe('Node 1');
    expect(nodes[1].text()).toBe('Node 2');
  });

  it('shows a loading indicator while fetching', async () => {
    vi.spyOn(backend, 'getFileOutline').mockReturnValue(new Promise(() => {})); // Promise that never resolves

    const wrapper = mountComponent();
    await flushPromises();
    await wrapper.find('[data-testid="file-tree-toggle"]').trigger('click');

    expect(wrapper.text()).toContain('Loading...');
  });

  it('shows an error message on fetch failure', async () => {
    vi.spyOn(backend, 'getFileOutline').mockRejectedValue(new Error('File not found'));

    const wrapper = mountComponent();
    await flushPromises();
    await wrapper.find('[data-testid="file-tree-toggle"]').trigger('click');
    await flushPromises();

    expect(wrapper.text()).toContain('File not found');
  });
});
