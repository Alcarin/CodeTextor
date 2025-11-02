import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import OutlineTreeNode from './OutlineTreeNode.vue';

describe('OutlineTreeNode.vue', () => {
  const node = {
    id: 'n1',
    name: 'MyClass',
    kind: 'class',
    startLine: 1,
    endLine: 10,
    children: [
      {
        id: 'n2',
        name: 'myMethod',
        kind: 'function',
        startLine: 2,
        endLine: 8,
        children: [],
      },
    ],
  };

  it('renders node information', () => {
    const wrapper = mount(OutlineTreeNode, {
      props: { node, level: 0, expanded: true },
    });

    expect(wrapper.text()).toContain('MyClass');
    expect(wrapper.text()).toContain('L1-10');
    expect(wrapper.find('.node-icon').text()).toBe('🔷'); // Icon for class
  });

  it('emits toggle event on click', async () => {
    const wrapper = mount(OutlineTreeNode, {
      props: { node, level: 0, expanded: false },
    });

    await wrapper.find('.node-header').trigger('click');
    expect(wrapper.emitted().toggle).toBeTruthy();
    expect(wrapper.emitted().toggle[0]).toEqual(['n1']);
  });

  it('does not render children when collapsed', () => {
    const wrapper = mount(OutlineTreeNode, {
      props: { node, level: 0, expanded: false },
    });

    expect(wrapper.find('.node-children').exists()).toBe(false);
  });

  it('renders children when expanded', () => {
    const wrapper = mount(OutlineTreeNode, {
      props: { node, level: 0, expanded: true },
    });

    const childrenContainer = wrapper.find('.node-children');
    expect(childrenContainer.exists()).toBe(true);
    expect(childrenContainer.text()).toContain('myMethod');
  });
});
