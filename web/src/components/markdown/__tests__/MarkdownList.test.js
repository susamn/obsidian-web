import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import MarkdownList from '../MarkdownList.vue'

describe('MarkdownList', () => {
  const baseItems = [
    { content: [{ type: 'text', content: 'Item 1' }] },
    { content: [{ type: 'text', content: 'Item 2' }] },
  ]

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

  it('renders an unordered list (ul)', () => {
    const node = {
      type: 'ul',
      items: baseItems,
    }

    const wrapper = mount(MarkdownList, {
      props: { node },
      global: {
        stubs: {
          InlineRenderer: InlineRendererStub,
        },
      },
    })

    expect(wrapper.find('ul').exists()).toBe(true)
    const listItems = wrapper.findAll('li')
    expect(listItems).toHaveLength(2)
    expect(listItems[0].text()).toBe('Item 1')
    expect(listItems[1].text()).toBe('Item 2')
  })

  it('renders an ordered list (ol)', () => {
    const node = {
      type: 'ol',
      items: baseItems,
    }

    const wrapper = mount(MarkdownList, {
      props: { node },
      global: {
        stubs: {
          InlineRenderer: InlineRendererStub,
        },
      },
    })

    expect(wrapper.find('ol').exists()).toBe(true)
    const listItems = wrapper.findAll('li')
    expect(listItems).toHaveLength(2)
    expect(listItems[0].text()).toBe('Item 1')
    expect(listItems[1].text()).toBe('Item 2')
  })

  it('emits a wikilink-click event from an item', async () => {
    const node = {
      type: 'ul',
      items: [{ content: [{ type: 'wikilink', content: '[[link]]' }] }],
    }

    const wrapper = mount(MarkdownList, {
      props: { node },
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
