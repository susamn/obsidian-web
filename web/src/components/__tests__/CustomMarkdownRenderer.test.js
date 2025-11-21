import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import CustomMarkdownRenderer from '../CustomMarkdownRenderer.vue';
import { parseMarkdown } from '../../utils/customMarkdownParser';

describe('CustomMarkdownRenderer', () => {
  it('should render heading nodes', () => {
    const nodes = parseMarkdown('# Hello World');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const h1 = wrapper.find('h1');
    expect(h1.exists()).toBe(true);
    expect(h1.text()).toBe('Hello World');
    expect(h1.attributes('id')).toBe('hello-world');
  });

  it('should render paragraph nodes', () => {
    const nodes = parseMarkdown('This is a paragraph.');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const p = wrapper.find('p');
    expect(p.exists()).toBe(true);
    expect(p.text()).toBe('This is a paragraph.');
  });

  it('should render bold text', () => {
    const nodes = parseMarkdown('This is **bold** text');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const strong = wrapper.find('strong');
    expect(strong.exists()).toBe(true);
    expect(strong.text()).toBe('bold');
  });

  it('should render italic text', () => {
    const nodes = parseMarkdown('This is *italic* text');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const em = wrapper.find('em');
    expect(em.exists()).toBe(true);
    expect(em.text()).toBe('italic');
  });

  it('should render inline code', () => {
    const nodes = parseMarkdown('This is `code` text');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const code = wrapper.find('code.md-code-inline');
    expect(code.exists()).toBe(true);
    expect(code.text()).toBe('code');
  });

  it('should render code blocks', () => {
    const nodes = parseMarkdown('```javascript\nconst x = 1;\n```');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const codeBlock = wrapper.find('.md-code-block');
    expect(codeBlock.exists()).toBe(true);
    expect(codeBlock.text()).toContain('const x = 1;');
    expect(codeBlock.text()).toContain('javascript');
  });

  it('should render unordered lists', () => {
    const nodes = parseMarkdown('- Item 1\n- Item 2\n- Item 3');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const ul = wrapper.find('ul');
    expect(ul.exists()).toBe(true);
    const items = wrapper.findAll('li');
    expect(items).toHaveLength(3);
    expect(items[0].text()).toBe('Item 1');
  });

  it('should render ordered lists', () => {
    const nodes = parseMarkdown('1. First\n2. Second\n3. Third');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const ol = wrapper.find('ol');
    expect(ol.exists()).toBe(true);
    const items = wrapper.findAll('li');
    expect(items).toHaveLength(3);
  });

  it('should render blockquotes', () => {
    const nodes = parseMarkdown('> This is a quote');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const blockquote = wrapper.find('blockquote');
    expect(blockquote.exists()).toBe(true);
    expect(blockquote.text()).toBe('This is a quote');
  });

  it('should render callouts', () => {
    const nodes = parseMarkdown('> [!note] Important\n> This is a note');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const callout = wrapper.find('.md-callout');
    expect(callout.exists()).toBe(true);
    expect(callout.find('.md-callout-title').text()).toBe('Important');
    expect(callout.classes()).toContain('md-callout-note');
  });

  it('should render tables', () => {
    const nodes = parseMarkdown('| A | B |\n| --- | --- |\n| 1 | 2 |');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const table = wrapper.find('table');
    expect(table.exists()).toBe(true);
    const headers = wrapper.findAll('th');
    expect(headers).toHaveLength(2);
    expect(headers[0].text()).toBe('A');
  });

  it('should render horizontal rules', () => {
    const nodes = parseMarkdown('---');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const hr = wrapper.find('hr');
    expect(hr.exists()).toBe(true);
  });

  it('should render wikilinks with pill style', () => {
    const wikilinks = [{
      original: '[[Test Page]]',
      target: 'Test Page',
      display: 'Test Page',
      exists: true,
      file_id: 'file-123'
    }];
    const nodes = parseMarkdown('See [[Test Page]]', wikilinks);
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const pill = wrapper.find('.md-wikilink-pill');
    expect(pill.exists()).toBe(true);
    expect(pill.find('.md-wikilink-label').text()).toBe('B');
    expect(pill.find('.md-wikilink-content').text()).toBe('Test Page');
  });

  it('should render broken wikilinks differently', () => {
    const wikilinks = [{
      original: '[[Missing]]',
      target: 'Missing',
      display: 'Missing',
      exists: false
    }];
    const nodes = parseMarkdown('See [[Missing]]', wikilinks);
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const pill = wrapper.find('.md-wikilink-pill-broken');
    expect(pill.exists()).toBe(true);
  });

  it('should emit wikilink-click events', async () => {
    const wikilinks = [{
      original: '[[Test]]',
      target: 'Test',
      display: 'Test',
      exists: true,
      file_id: 'file-123'
    }];
    const nodes = parseMarkdown('[[Test]]', wikilinks);
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    await wrapper.find('.md-wikilink-pill-link').trigger('click');

    expect(wrapper.emitted('wikilink-click')).toBeTruthy();
    expect(wrapper.emitted('wikilink-click')[0][0]).toMatchObject({
      fileId: 'file-123',
      target: 'Test',
      display: 'Test'
    });
  });

  it('should render tags', () => {
    const nodes = parseMarkdown('This has #tag in it');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const tag = wrapper.find('.md-tag');
    expect(tag.exists()).toBe(true);
    expect(tag.text()).toBe('#tag');
  });

  it('should render links', () => {
    const nodes = parseMarkdown('[Click here](https://example.com)');
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    const link = wrapper.find('a.md-link');
    expect(link.exists()).toBe(true);
    expect(link.text()).toBe('Click here');
    expect(link.attributes('href')).toBe('https://example.com');
  });

  it('should render complex documents', () => {
    const markdown = `# Title

This is **bold** and *italic*.

- List item
- Another item

> [!warning] Alert
> Be careful!`;

    const nodes = parseMarkdown(markdown);
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes }
    });

    expect(wrapper.find('h1').exists()).toBe(true);
    expect(wrapper.find('strong').exists()).toBe(true);
    expect(wrapper.find('em').exists()).toBe(true);
    expect(wrapper.find('ul').exists()).toBe(true);
    expect(wrapper.find('.md-callout').exists()).toBe(true);
  });

  it('should handle empty nodes array', () => {
    const wrapper = mount(CustomMarkdownRenderer, {
      props: { nodes: [] }
    });

    expect(wrapper.html()).toContain('custom-markdown-renderer');
  });
});
