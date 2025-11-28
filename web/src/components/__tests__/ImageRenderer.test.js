import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import ImageRenderer from '../ImageRenderer.vue'

describe('ImageRenderer', () => {
  it('renders image with correct url', () => {
    const wrapper = mount(ImageRenderer, {
      props: {
        vaultId: 'test-vault',
        fileId: '12345',
        fileName: 'test-image.png',
      },
    })

    const img = wrapper.find('img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe('/api/v1/assets/test-vault/12345')
    expect(img.attributes('alt')).toBe('test-image.png')
  })

  it('shows loading spinner initially', () => {
    const wrapper = mount(ImageRenderer, {
      props: {
        vaultId: 'test-vault',
        fileId: '12345',
      },
    })

    expect(wrapper.find('.loading-overlay').exists()).toBe(true)
  })

  it('hides loading spinner and shows error on load error', async () => {
    const wrapper = mount(ImageRenderer, {
      props: {
        vaultId: 'test-vault',
        fileId: '12345',
      },
    })

    const img = wrapper.find('img')
    await img.trigger('error')

    expect(wrapper.find('.loading-overlay').exists()).toBe(false)
    expect(wrapper.find('.error-message').exists()).toBe(true)
    expect(wrapper.find('.error-message').text()).toContain('Failed to load image')
  })

  it('hides loading spinner on load success', async () => {
    const wrapper = mount(ImageRenderer, {
      props: {
        vaultId: 'test-vault',
        fileId: '12345',
      },
    })

    const img = wrapper.find('img')
    await img.trigger('load')

    expect(wrapper.find('.loading-overlay').exists()).toBe(false)
    expect(wrapper.find('.error-message').exists()).toBe(false)
  })
})
