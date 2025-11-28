import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createTestingPinia } from '@pinia/testing'
import QuickFind from '../QuickFind.vue'
import { usePersistentTreeStore } from '../../stores/persistentTreeStore'

describe('QuickFind', () => {
  const vaultId = 'test-vault'
  let store

  const mockFiles = [
    {
      metadata: {
        id: '1',
        name: 'Note 1.md',
        path: 'folder/Note 1.md',
        is_directory: false,
        is_markdown: true
      }
    },
    {
      metadata: {
        id: '2',
        name: 'Image.png',
        path: 'assets/Image.png',
        is_directory: false,
        is_markdown: false
      }
    },
    {
      metadata: {
        id: '3',
        name: 'Folder',
        path: 'folder',
        is_directory: true, // Should be filtered out
        is_markdown: false
      }
    },
    {
      metadata: {
        id: '4',
        name: 'Another Note.md',
        path: 'Another Note.md',
        is_directory: false,
        is_markdown: true
      }
    }
  ]

  beforeEach(() => {
    // Setup mock store data
    const pathMap = new Map()
    mockFiles.forEach(node => pathMap.set(node.metadata.path, node))

    const initialState = {
      pathIndex: new Map([[vaultId, pathMap]])
    }

    mount(QuickFind, {
      props: { vaultId },
      global: {
        plugins: [createTestingPinia({
          createSpy: vi.fn,
          initialState: {
            persistentTree: initialState
          }
        })]
      }
    })
    
    store = usePersistentTreeStore()
    store.pathIndex = new Map([[vaultId, pathMap]])
  })

  it('renders properly', () => {
    const wrapper = mount(QuickFind, {
      props: { vaultId },
      global: {
        plugins: [createTestingPinia({ createSpy: vi.fn })]
      }
    })
    expect(wrapper.find('.quick-find-input').exists()).toBe(true)
    expect(wrapper.find('.results-list').exists()).toBe(false)
  })

  it('filters files based on input', async () => {
    const wrapper = mount(QuickFind, {
      props: { vaultId },
      global: {
        plugins: [createTestingPinia({
            createSpy: vi.fn,
            initialState: {
                persistentTree: {
                    pathIndex: store.pathIndex
                }
            }
        })]
      }
    })

    const input = wrapper.find('input')
    await input.setValue('Note')

    expect(wrapper.find('.results-list').exists()).toBe(true)
    const results = wrapper.findAll('.result-item')
    expect(results.length).toBe(2) // Note 1.md and Another Note.md
    expect(results[0].text()).toContain('Note 1.md')
    expect(results[1].text()).toContain('Another Note.md')
  })

  it('shows "No files found" when query matches nothing', async () => {
    const wrapper = mount(QuickFind, {
      props: { vaultId },
      global: {
        plugins: [createTestingPinia({
            createSpy: vi.fn,
            initialState: {
                persistentTree: {
                    pathIndex: store.pathIndex
                }
            }
        })]
      }
    })

    const input = wrapper.find('input')
    await input.setValue('xyznonexistent')

    expect(wrapper.find('.results-list').exists()).toBe(false)
    expect(wrapper.find('.no-results').exists()).toBe(true)
    expect(wrapper.find('.no-results').text()).toBe('No files found')
  })

  it('clears search when clear button is clicked', async () => {
    const wrapper = mount(QuickFind, {
      props: { vaultId },
      global: {
        plugins: [createTestingPinia({
            createSpy: vi.fn,
            initialState: {
                persistentTree: {
                    pathIndex: store.pathIndex
                }
            }
        })]
      }
    })

    const input = wrapper.find('input')
    await input.setValue('Note')
    expect(wrapper.find('.clear-button').exists()).toBe(true)

    await wrapper.find('.clear-button').trigger('click')
    expect(input.element.value).toBe('')
    expect(wrapper.find('.results-list').exists()).toBe(false)
  })

  it('navigates results with arrow keys', async () => {
    const wrapper = mount(QuickFind, {
      props: { vaultId },
      global: {
        plugins: [createTestingPinia({
            createSpy: vi.fn,
            initialState: {
                persistentTree: {
                    pathIndex: store.pathIndex
                }
            }
        })]
      }
    })

    // Mock scrollIntoView
    Element.prototype.scrollIntoView = vi.fn()

    const input = wrapper.find('input')
    await input.setValue('Note')
    
    const results = wrapper.findAll('.result-item')
    expect(results[0].classes()).toContain('active')
    expect(results[1].classes()).not.toContain('active')

    // Down arrow
    await input.trigger('keydown.down')
    expect(results[0].classes()).not.toContain('active')
    expect(results[1].classes()).toContain('active')

    // Up arrow
    await input.trigger('keydown.up')
    expect(results[0].classes()).toContain('active')
    expect(results[1].classes()).not.toContain('active')
  })

  it('selects file on click', async () => {
    const wrapper = mount(QuickFind, {
      props: { vaultId },
      global: {
        plugins: [createTestingPinia({
            createSpy: vi.fn,
            initialState: {
                persistentTree: {
                    pathIndex: store.pathIndex
                }
            }
        })]
      }
    })

    await wrapper.find('input').setValue('Note')
    const results = wrapper.findAll('.result-item')
    
    await results[0].trigger('click')
    
    expect(wrapper.emitted('select')).toBeTruthy()
    expect(wrapper.emitted('select')[0][0].metadata.id).toBe('1')
    expect(wrapper.find('input').element.value).toBe('') // Should clear after selection
  })

  it('selects file on enter key', async () => {
    const wrapper = mount(QuickFind, {
      props: { vaultId },
      global: {
        plugins: [createTestingPinia({
            createSpy: vi.fn,
            initialState: {
                persistentTree: {
                    pathIndex: store.pathIndex
                }
            }
        })]
      }
    })

    const input = wrapper.find('input')
    await input.setValue('Note')
    
    // Select second item
    await input.trigger('keydown.down')
    await input.trigger('keydown.enter')

    expect(wrapper.emitted('select')).toBeTruthy()
    expect(wrapper.emitted('select')[0][0].metadata.id).toBe('4') // 'Another Note.md'
  })
})
