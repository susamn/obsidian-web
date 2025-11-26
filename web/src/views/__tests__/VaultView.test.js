import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { ref } from 'vue'
import VaultView from '../VaultView.vue'
import { useFileStore } from '../../stores/fileStore'
import { createRouter, createWebHistory } from 'vue-router'

// Mock the useSSE composable
vi.mock('../../composables/useSSE', () => ({
  useSSE: vi.fn(() => ({
    connected: ref(false),
    error: ref(null),
    connect: vi.fn(),
    disconnect: vi.fn(),
    reconnect: vi.fn(),
  })),
}))

const router = createRouter({
  history: createWebHistory(),
  routes: [{ path: '/vault/:id', component: VaultView }],
})

describe('VaultView', () => {
  let fileStore

  beforeEach(async () => {
    router.push('/vault/test-vault')
    await router.isReady()
  })

  it('renders properly and fetches tree data', async () => {
    const wrapper = mount(VaultView, {
      global: {
        plugins: [
          router,
          createTestingPinia({
            createSpy: vi.fn,
            initialState: {
              file: {
                vaultId: 'test-vault',
                treeData: [],
                loading: false,
                error: null,
              },
            },
          }),
        ],
      },
    })

    fileStore = useFileStore()

    expect(wrapper.text()).toContain('Vault test-vault')
    expect(fileStore.setVaultId).toHaveBeenCalledWith('test-vault')
    expect(fileStore.fetchTree).toHaveBeenCalledWith('test-vault')
    expect(wrapper.findComponent({ name: 'FileTree' }).exists()).toBe(true)
  })

  it('displays loading state', () => {
    const wrapper = mount(VaultView, {
      global: {
        plugins: [
          router,
          createTestingPinia({
            createSpy: vi.fn,
            initialState: {
              file: {
                vaultId: 'test-vault',
                treeData: [],
                loading: true,
                error: null,
              },
            },
          }),
        ],
      },
    })

    expect(wrapper.text()).toContain('Loading file tree...')
  })

  it('displays error state', () => {
    const wrapper = mount(VaultView, {
      global: {
        plugins: [
          router,
          createTestingPinia({
            createSpy: vi.fn,
            initialState: {
              file: {
                vaultId: 'test-vault',
                treeData: [],
                loading: false,
                error: 'Failed to load',
              },
            },
          }),
        ],
      },
    })

    expect(wrapper.text()).toContain('Error: Failed to load')
  })
})
