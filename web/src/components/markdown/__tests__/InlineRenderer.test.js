import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import InlineRenderer from '../InlineRenderer.vue'
import katex from 'katex'

// Mock katex
vi.mock('katex', () => ({
  default: {
    renderToString: vi.fn((content, options) => {
      return `<span class="katex-mock mode-${options.displayMode ? 'display' : 'inline'}">${content}</span>`
    }),
  },
}))

describe('InlineRenderer', () => {
  it('renders text tokens', () => {
    const tokens = [{ type: 'text', content: 'Hello World' }]
    const wrapper = mount(InlineRenderer, {
      props: { tokens },
    })
    expect(wrapper.text()).toBe('Hello World')
  })

  it('renders inline latex tokens', () => {
    const tokens = [
      { type: 'text', content: 'Equation: ' },
      { type: 'latex', content: 'x^2', displayMode: false },
    ]

    const wrapper = mount(InlineRenderer, {
      props: { tokens },
    })

    expect(katex.renderToString).toHaveBeenCalledWith('x^2', expect.objectContaining({
      displayMode: false,
    }))
    expect(wrapper.html()).toContain('<span class="katex-mock mode-inline">x^2</span>')
  })

  it('renders display mode inline latex tokens', () => {
    const tokens = [
      { type: 'latex', content: 'sum', displayMode: true },
    ]

    const wrapper = mount(InlineRenderer, {
      props: { tokens },
    })

    expect(katex.renderToString).toHaveBeenCalledWith('sum', expect.objectContaining({
      displayMode: true,
    }))
    expect(wrapper.html()).toContain('<span class="katex-mock mode-display">sum</span>')
  })
})
