import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, computed } from 'vue';
import SearchView from './SearchView.vue';
import { backend, models } from '../api/backend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');
vi.mock('../api/backend', async () => {
  const actual = await vi.importActual<typeof import('../api/backend')>('../api/backend');
  return {
    ...actual,
    backend: {
      ...actual.backend,
      search: vi.fn()
    }
  };
});

describe('SearchView.vue', () => {
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
    backendMock.search.mockReset();
  });

  const mountComponent = () => mount(SearchView);

  it('renders the search input and button', () => {
    const wrapper = mountComponent();
    expect(wrapper.find('#query').exists()).toBe(true);
    expect(wrapper.find('button').text()).toContain('Search');
  });

  it('calls semanticSearch on search button click', async () => {
    const semanticSearchSpy = backendMock.search.mockResolvedValue(
      models.SearchResponse.createFrom({ chunks: [], totalResults: 0, queryTime: 0 })
    );

    const wrapper = mountComponent();
    await wrapper.find('#query').setValue('test query');
    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    expect(semanticSearchSpy).toHaveBeenCalledWith('p1', 'test query', 10);
  });

  it('displays search results', async () => {
    const chunks = [
      { id: 'c1', projectId: 'p1', symbolName: 'Chunk 1', content: 'content 1', filePath: 'file1.ts', embedding: [], lineStart: 1, lineEnd: 10, charStart: 0, charEnd: 100, createdAt: 0, updatedAt: 0, similarity: 0.9, symbolKind: 'function' },
      { id: 'c2', projectId: 'p1', symbolName: 'Chunk 2', content: 'content 2', filePath: 'file2.ts', embedding: [], lineStart: 5, lineEnd: 15, charStart: 50, charEnd: 150, createdAt: 0, updatedAt: 0, similarity: 0.8, symbolKind: 'class' },
    ];
    backendMock.search.mockResolvedValue(models.SearchResponse.createFrom({ chunks, totalResults: 2, queryTime: 123 }));

    const wrapper = mountComponent();
    await wrapper.find('#query').setValue('test query');
    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    expect(wrapper.text()).toContain('Found 2 results in 123ms');
    const results = wrapper.findAll('.result-item');
    expect(results.length).toBe(2);
    expect(results[0].text()).toContain('Chunk 1');
    expect(results[1].text()).toContain('Chunk 2');
  });

  it('shows a loading indicator while searching', async () => {
    backendMock.search.mockReturnValue(new Promise(() => {})); // Never resolves

    const wrapper = mountComponent();
    await wrapper.find('#query').setValue('test query');
    await wrapper.find('button.btn-primary').trigger('click');

    expect(wrapper.find('button.btn-primary').attributes('disabled')).toBeDefined();
    expect(wrapper.find('button.btn-primary').text()).toContain('Searching...');
  });

  it('shows an error message on search failure', async () => {
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
    backendMock.search.mockRejectedValue(new Error('Search failed'));

    const wrapper = mountComponent();
    await wrapper.find('#query').setValue('test query');
    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    expect(alertSpy).toHaveBeenCalledWith('Search failed: Search failed');
    alertSpy.mockRestore();
  });
});
