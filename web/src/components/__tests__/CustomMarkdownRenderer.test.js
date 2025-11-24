import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import CustomMarkdownRenderer from '../CustomMarkdownRenderer.vue';
import MarkdownParagraph from '../markdown/MarkdownParagraph.vue';

describe('CustomMarkdownRenderer', () => {
  it('should render a paragraph for an unknown node type', () => {
    const nodes = [{ type: 'unknown_node_type', content: 'some content' }];
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes },
      global: {
        stubs: {
          MarkdownParagraph: true,
        }
      }
    });

    // We expect it to fall back to MarkdownParagraph
    const paragraph = wrapper.findComponent(MarkdownParagraph);
    expect(paragraph.exists()).toBe(true);
    expect(paragraph.props('node')).toEqual(nodes[0]);
  });

  it('emits wikilink-click event when a child component emits it', async () => {
    const nodes = [{ type: 'paragraph', content: [{ type: 'wikilink', text: '[[link]]' }] }];
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    // Simulate the event from a child component
    // Here we find a component that can emit the event (e.g., MarkdownParagraph)
    const paragraph = wrapper.findComponent(MarkdownParagraph);
    const eventPayload = { fileId: 'file-123', target: 'link' };
    paragraph.vm.$emit('wikilink-click', eventPayload);
    await wrapper.vm.$nextTick();

    expect(wrapper.emitted('wikilink-click')).toBeTruthy();
    expect(wrapper.emitted('wikilink-click')[0][0]).toEqual(eventPayload);
  });

  it('renders nothing when nodes array is empty', () => {
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes: [] }
    });
    const renderer = wrapper.find('.custom-markdown-renderer');
    expect(renderer.exists()).toBe(true);
    expect(renderer.element.children).toHaveLength(0);
  });

  it('renders multiple components for multiple nodes', () => {
    const nodes = [
      { type: 'heading', level: 1, content: [{type: 'text', text: 'Title'}] },
      { type: 'paragraph', content: [{type: 'text', text: 'Some text.'}] }
    ];
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    expect(wrapper.findAll('.custom-markdown-renderer > *')).toHaveLength(2);
    expect(wrapper.find('h1').exists()).toBe(true);
    expect(wrapper.find('p').exists()).toBe(true);
  });
});