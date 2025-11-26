import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import MarkdownCodeBlock from '../MarkdownCodeBlock.vue'

describe('MarkdownCodeBlock', () => {
  it('renders the code block with content and without a language', () => {
    const node = {
      content: 'console.log("Hello, World!");',
    }

    const wrapper = mount(MarkdownCodeBlock, {
      props: {
        node,
      },
    })

    expect(wrapper.find('.code-language').exists()).toBe(false)
    expect(wrapper.find('pre code').text()).toBe('console.log("Hello, World!");')
  })

  it('renders the code block with a language', () => {
    const node = {
      language: 'javascript',
      content: 'console.log("Hello, World!");',
    }

    const wrapper = mount(MarkdownCodeBlock, {
      props: {
        node,
      },
    })

    expect(wrapper.find('.code-language').exists()).toBe(true)
    expect(wrapper.find('.code-language').text()).toBe('javascript')
    expect(wrapper.find('pre code').text()).toBe('console.log("Hello, World!");')
  })
})
