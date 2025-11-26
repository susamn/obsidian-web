import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import MarkdownHeading from '../MarkdownHeading.vue'

describe('MarkdownHeading', () => {
  it.each([1, 2, 3, 4, 5, 6])('renders a h%i heading', (level) => {
    const node = {
      level,
      id: `heading-${level}`,
      content: [{ type: 'text', content: `This is a heading ${level}` }],
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

    const wrapper = mount(MarkdownHeading, {
      props: {
        node,
      },
      global: {
        stubs: {
          InlineRenderer: InlineRendererStub,
        },
      },
    })

    const heading = wrapper.find(`h${level}`)
    expect(heading.exists()).toBe(true)
    expect(heading.attributes('id')).toBe(`heading-${level}`)
    expect(heading.text()).toContain(`This is a heading ${level}`)
  })

  it('emits a wikilink-click event when InlineRenderer emits it', async () => {
    const node = {
      level: 1,
      id: 'heading-1',
      content: [{ type: 'wikilink', content: '[[link]]' }],
    }

    const wrapper = mount(MarkdownHeading, {
      props: {
        node,
      },
      global: {
        stubs: {
          InlineRenderer: {
            template: "<span @click=\"$emit('wikilink-click', { href: 'link' })\"></span>",
            props: ['tokens'],
          },
        },
      },
    })

    await wrapper.find('span').trigger('click')
    expect(wrapper.emitted('wikilink-click')).toBeTruthy()
    expect(wrapper.emitted('wikilink-click')[0][0]).toEqual({ href: 'link' })
  })
})
