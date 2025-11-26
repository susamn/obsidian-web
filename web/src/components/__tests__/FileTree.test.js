import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import FileTree from '../FileTree.vue'

describe('FileTree', () => {
  const mockNodes = [
    {
      metadata: {
        path: 'file1.md',
        name: 'file1.md',
        is_directory: false,
        is_markdown: true,
      },
    },
    {
      metadata: {
        path: 'folder1',
        name: 'folder1',
        is_directory: true,
        is_markdown: false,
      },
      children: [],
    },
  ]

  it('renders file and folder nodes', () => {
    const wrapper = mount(FileTree, {
      props: {
        nodes: mockNodes,
        vaultId: 'test-vault',
        expandedNodes: {},
      },
      global: {
        plugins: [createPinia()],
      },
    })

    expect(wrapper.text()).toContain('file1.md')
    expect(wrapper.text()).toContain('folder1')
    expect(wrapper.findAll('.fa-file-alt').length).toBe(1) // Markdown file icon
    expect(wrapper.findAll('.fa-folder').length).toBe(1) // Closed folder icon
  })

  it('emits toggle-expand when a folder is clicked', async () => {
    const wrapper = mount(FileTree, {
      props: {
        nodes: mockNodes,
        vaultId: 'test-vault',
        expandedNodes: {},
      },
      global: {
        plugins: [createPinia()],
      },
    })

    await wrapper.findAll('.node-header')[1].trigger('click') // Click on folder1
    expect(wrapper.emitted('toggle-expand')).toBeTruthy()
    expect(wrapper.emitted('toggle-expand')[0][0]).toEqual(mockNodes[1])
  })

  it('renders expanded folder children', async () => {
    const folderNode = {
      metadata: {
        id: 'folder1-id',
        path: 'folder1',
        name: 'folder1',
        is_directory: true,
        is_markdown: false,
      },
      children: [
        {
          metadata: {
            id: 'child-file-id',
            path: 'folder1/child-file.txt',
            name: 'child-file.txt',
            is_directory: false,
            is_markdown: false,
          },
        },
      ],
    }

    const expandedNodes = {
      'folder1-id': true,
    }

    const wrapper = mount(FileTree, {
      props: {
        nodes: [folderNode],
        vaultId: 'test-vault',
        expandedNodes: expandedNodes,
      },
      global: {
        plugins: [createPinia()],
      },
    })

    expect(wrapper.text()).toContain('folder1')
    expect(wrapper.text()).toContain('child-file.txt')
    expect(wrapper.findAll('.fa-folder-open').length).toBe(1) // Open folder icon
    expect(wrapper.findAll('.fa-file-alt').length).toBe(1) // Text file icon for txt
  })
})
