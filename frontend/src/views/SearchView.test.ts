import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref } from 'vue';
import SearchView from './SearchView.vue';
import { mockBackend } from '../services/mockBackend';
import { useCurrentProject } from '../composables/useCurrentProject';

vi.mock('../composables/useCurrentProject');

describe('SearchView.vue', () => {
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

  const mountComponent = () => mount(SearchView);

  it('renders the search input and button', () => {
    const wrapper = mountComponent();
    expect(wrapper.find('#query').exists()).toBe(true);
    expect(wrapper.find('button').text()).toContain('Search');
  });

  it('calls semanticSearch on search button click', async () => {
    const semanticSearchSpy = vi.spyOn(mockBackend, 'semanticSearch').mockResolvedValue({ chunks: [], totalResults: 0, queryTime: 0 });

    const wrapper = mountComponent();
    await wrapper.find('#query').setValue('test query');
    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    expect(semanticSearchSpy).toHaveBeenCalledWith({ projectId: 'p1', query: 'test query', k: 10 });
  });

  it('displays search results', async () => {
    const chunks = [
      { id: 'c1', name: 'Chunk 1', content: 'content 1', filePath: 'file1.ts', startLine: 1, endLine: 10, similarity: 0.9, kind: 'function' },
      { id: 'c2', name: 'Chunk 2', content: 'content 2', filePath: 'file2.ts', startLine: 5, endLine: 15, similarity: 0.8, kind: 'class' },
    ];
    vi.spyOn(mockBackend, 'semanticSearch').mockResolvedValue({ chunks, totalResults: 2, queryTime: 123 });

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
    vi.spyOn(mockBackend, 'semanticSearch').mockReturnValue(new Promise(() => {})); // Never resolves

    const wrapper = mountComponent();
    await wrapper.find('#query').setValue('test query');
    await wrapper.find('button.btn-primary').trigger('click');

    expect(wrapper.find('button.btn-primary').attributes('disabled')).toBeDefined();
    expect(wrapper.find('button.btn-primary').text()).toContain('Searching...');
  });

  it('shows an error message on search failure', async () => {
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
    vi.spyOn(mockBackend, 'semanticSearch').mockRejectedValue(new Error('Search failed'));

    const wrapper = mountComponent();
    await wrapper.find('#query').setValue('test query');
    await wrapper.find('button.btn-primary').trigger('click');
    await flushPromises();

    expect(alertSpy).toHaveBeenCalledWith('Search failed: Search failed');
    alertSpy.mockRestore();
  });
});