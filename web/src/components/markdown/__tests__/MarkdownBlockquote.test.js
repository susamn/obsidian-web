import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import MarkdownBlockquote from '../MarkdownBlockquote.vue'

describe('MarkdownBlockquote', () => {
  it('renders the blockquote with the correct content', () => {
    const node = {
      content: [{ type: 'text', content: 'This is a blockquote.' }],
    }

    const InlineRendererStub = {
      props: ['tokens'],
      template: `
        <div class="inline-renderer-stub">
          <template v-for="token in tokens">
            <span v-if="token.type === 'text'">{{ token.content }}</span>
          </template>
        </div>
      `,
    }

    const wrapper = mount(MarkdownBlockquote, {
      props: {
        node,
      },
      global: {
        stubs: {
          InlineRenderer: InlineRendererStub,
        },
      },
    })

    expect(wrapper.find('blockquote').exists()).toBe(true)
    expect(wrapper.text()).toContain('This is a blockquote.')
  })

  it('emits a wikilink-click event when InlineRenderer emits it', async () => {
    const node = {
      content: [{ type: 'wikilink', content: '[[link]]' }],
    }

    const wrapper = mount(MarkdownBlockquote, {
      props: {
        node,
      },
      global: {
        stubs: {
          InlineRenderer: {
            template: "<div @click=\"$emit('wikilink-click', { href: 'link' })\"></div>",
            props: ['tokens'],
          },
        },
      },
    })

    await wrapper.find('div').trigger('click')
    expect(wrapper.emitted('wikilink-click')).toBeTruthy()
    expect(wrapper.emitted('wikilink-click')[0][0]).toEqual({ href: 'link' })
  })
})
