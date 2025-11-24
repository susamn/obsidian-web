
import { mount } from '@vue/test-utils';
import { describe, it, expect } from 'vitest';
import MarkdownCallout from '../MarkdownCallout.vue';

describe('MarkdownCallout', () => {
  const baseNode = {
    title: 'Test Title',
    content: [{ type: 'text', content: 'This is a callout.' }],
  };

  const calloutTypes = [
    'note', 'abstract', 'summary', 'tldr', 'info', 'tip', 'hint', 'important',
    'warning', 'caution', 'attention', 'danger', 'error', 'failure', 'bug',
    'example', 'quote'
  ];

  it.each(calloutTypes)('renders correctly for type "%s"', (calloutType) => {
    const node = { ...baseNode, calloutType };
    const InlineRendererStub = {
      props: ['tokens'],
      template: `
        <div class="inline-renderer-stub">
          <template v-for="token in tokens">
            <span v-if="token.type === 'text'">{{ token.content }}</span>
          </template>
        </div>
      `
    };
    const wrapper = mount(MarkdownCallout, {
      props: { node },
      global: {
        stubs: {
          InlineRenderer: InlineRendererStub
        }
      }
    });

    expect(wrapper.find('.md-callout').classes()).toContain(`md-callout-${calloutType}`);
    expect(wrapper.find('.md-callout-title').text()).toBe('Test Title');
    expect(wrapper.find('.md-callout-content').text()).toContain('This is a callout.');
  });

  it('renders with default "note" type for unknown callout types', () => {
    const node = { ...baseNode, calloutType: 'unknown' };
    const wrapper = mount(MarkdownCallout, {
      props: { node },
      global: {
        stubs: {
          InlineRenderer: true
        }
      }
    });
    expect(wrapper.find('.md-callout').classes()).toContain('md-callout-note');
  });

  it('emits a wikilink-click event when InlineRenderer emits it', async () => {
    const node = { ...baseNode, calloutType: 'info' };
    const wrapper = mount(MarkdownCallout, {
      props: { node },
      global: {
        stubs: {
          InlineRenderer: {
            template: '<div @click="$emit(\'wikilink-click\', { href: \'link\' })"></div>',
          }
        }
      }
    });

    await wrapper.find('.md-callout-content div').trigger('click');
    expect(wrapper.emitted('wikilink-click')).toBeTruthy();
    expect(wrapper.emitted('wikilink-click')[0][0]).toEqual({ href: 'link' });
  });
});
