
import { mount } from '@vue/test-utils';
import { describe, it, expect } from 'vitest';
import MarkdownParagraph from '../MarkdownParagraph.vue';

describe('MarkdownParagraph', () => {
  it('renders the paragraph with content', () => {
    const node = {
      content: [{ type: 'text', content: 'This is a paragraph.' }]
    };

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

    const wrapper = mount(MarkdownParagraph, {
      props: {
        node
      },
      global: {
        stubs: {
          InlineRenderer: InlineRendererStub
        }
      }
    });

    expect(wrapper.find('p').exists()).toBe(true);
    expect(wrapper.text()).toContain('This is a paragraph.');
  });

  it('emits a wikilink-click event when InlineRenderer emits it', async () => {
    const node = {
      content: [{ type: 'wikilink', content: '[[link]]' }]
    };

    const wrapper = mount(MarkdownParagraph, {
      props: {
        node
      },
      global: {
        stubs: {
          InlineRenderer: {
            template: '<span @click="$emit(\'wikilink-click\', { href: \'link\' })"></span>',
            props: ['tokens']
          }
        }
      }
    });

    await wrapper.find('span').trigger('click');
    expect(wrapper.emitted('wikilink-click')).toBeTruthy();
    expect(wrapper.emitted('wikilink-click')[0][0]).toEqual({ href: 'link' });
  });
});
