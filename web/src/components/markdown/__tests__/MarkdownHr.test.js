import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import MarkdownHr from '../MarkdownHr.vue'

describe('MarkdownHr', () => {
  it('renders an hr element', () => {
    const node = {} // node prop is not used by the component, but required
    const wrapper = mount(MarkdownHr, {
      props: {
        node,
      },
    })

    expect(wrapper.find('hr').exists()).toBe(true)
    expect(wrapper.find('hr').classes()).toContain('md-hr')
  })
})
