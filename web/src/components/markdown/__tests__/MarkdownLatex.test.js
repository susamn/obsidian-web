import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import MarkdownLatex from '../MarkdownLatex.vue'
import katex from 'katex'

// Mock katex
vi.mock('katex', () => ({
  default: {
    renderToString: vi.fn((content, options) => {
      if (content === 'error') {
        throw new Error('KaTeX error')
      }
      return `<span class="katex-mock">${content}</span>`
    }),
  },
}))

describe('MarkdownLatex', () => {
  it('renders latex content using KaTeX', () => {
    const node = {
      content: 'E = mc^2',
    }

    const wrapper = mount(MarkdownLatex, {
      props: {
        node,
      },
    })

    expect(katex.renderToString).toHaveBeenCalledWith('E = mc^2', {
      throwOnError: false,
      displayMode: true,
    })
    expect(wrapper.find('.md-latex').exists()).toBe(true)
    expect(wrapper.html()).toContain('<span class="katex-mock">E = mc^2</span>')
  })

  it('handles KaTeX errors gracefully', () => {
    const node = {
      content: 'error',
    }

    const wrapper = mount(MarkdownLatex, {
      props: {
        node,
      },
    })

    expect(wrapper.find('.latex-error').exists()).toBe(true)
    expect(wrapper.text()).toContain('Error parsing LaTeX')
  })
})
