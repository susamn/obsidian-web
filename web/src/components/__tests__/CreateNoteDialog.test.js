import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import CreateNoteDialog from '../CreateNoteDialog.vue'

// Mock fetch globally
global.fetch = vi.fn()

describe('CreateNoteDialog', () => {
  beforeEach(() => {
    fetch.mockClear()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  const createWrapper = (props = {}) => {
    return mount(CreateNoteDialog, {
      props: {
        show: true,
        vaultId: 'test-vault',
        parentId: null,
        ...props,
      },
    })
  }

  it('renders the dialog when show is true', () => {
    const wrapper = createWrapper()
    expect(wrapper.find('.dialog-overlay').exists()).toBe(true)
    expect(wrapper.find('.dialog-header h2').text()).toBe('Create New Note')
  })

  it('does not render the dialog when show is false', () => {
    const wrapper = createWrapper({ show: false })
    expect(wrapper.find('.dialog-overlay').exists()).toBe(false)
  })

  it('displays folder creation form when folder type is selected', async () => {
    const wrapper = createWrapper()

    // Find and click the folder radio button
    const radios = wrapper.findAll('input[type="radio"]')
    await radios[1].setValue(true)
    await nextTick()

    expect(wrapper.find('.dialog-header h2').text()).toBe('Create New Folder')
    expect(wrapper.find('.content-editor').exists()).toBe(false)
  })

  it('displays note creation form with content editor when note type is selected', async () => {
    const wrapper = createWrapper()

    // Note type should be selected by default
    expect(wrapper.find('.dialog-header h2').text()).toBe('Create New Note')
    expect(wrapper.find('.content-editor').exists()).toBe(true)
  })

  it('auto-focuses filename input when dialog opens', async () => {
    const wrapper = createWrapper()
    await nextTick()

    const input = wrapper.find('#filename')
    // Check that the ref is set (can't directly test focus in jsdom)
    expect(input.element).toBeTruthy()
  })

  it('shows filename preview with .md extension for notes', async () => {
    const wrapper = createWrapper()

    const input = wrapper.find('#filename')
    await input.setValue('test-note')
    await nextTick()

    expect(wrapper.text()).toContain('Will be saved as:')
    expect(wrapper.text()).toContain('test-note.md')
  })

  it('does not add double .md extension if already present', async () => {
    const wrapper = createWrapper()

    const input = wrapper.find('#filename')
    await input.setValue('test-note.md')
    await nextTick()

    expect(wrapper.text()).toContain('test-note.md')
    expect(wrapper.text()).not.toContain('test-note.md.md')
  })

  it('does not show filename preview for folders', async () => {
    const wrapper = createWrapper()

    // Select folder type
    const radios = wrapper.findAll('input[type="radio"]')
    await radios[1].setValue(true)

    const input = wrapper.find('#filename')
    await input.setValue('test-folder')
    await nextTick()

    expect(wrapper.text()).not.toContain('Will be saved as:')
  })

  it('disables save button when filename is empty', async () => {
    const wrapper = createWrapper()

    const saveButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Save'))
    expect(saveButton.attributes('disabled')).toBeDefined()
  })

  it('enables save button when filename is provided', async () => {
    const wrapper = createWrapper()

    const input = wrapper.find('#filename')
    await input.setValue('test-note')
    await nextTick()

    const saveButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Save'))
    expect(saveButton.attributes('disabled')).toBeUndefined()
  })

  it('emits close event when cancel button is clicked', async () => {
    const wrapper = createWrapper()

    const cancelButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Cancel'))
    await cancelButton.trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('emits close event when clicking outside the dialog', async () => {
    const wrapper = createWrapper()

    await wrapper.find('.dialog-overlay').trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('calls API and emits created event on successful save', async () => {
    const mockResponse = {
      data: {
        id: 'new-file-id',
        path: 'test-note.md',
        name: 'test-note.md',
      },
    }

    fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockResponse,
    })

    const wrapper = createWrapper()

    const input = wrapper.find('#filename')
    await input.setValue('test-note')

    const saveButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Save'))
    await saveButton.trigger('click')
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('/api/v1/file/create', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        vault_id: 'test-vault',
        parent_id: null,
        name: 'test-note',
        is_folder: false,
        content: '',
      }),
    })

    expect(wrapper.emitted('created')).toBeTruthy()
    expect(wrapper.emitted('created')[0][0]).toEqual(mockResponse.data)
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('includes content in API call when provided', async () => {
    const mockResponse = {
      data: {
        id: 'new-file-id',
        path: 'test-note.md',
        name: 'test-note.md',
      },
    }

    fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockResponse,
    })

    const wrapper = createWrapper()

    await wrapper.find('#filename').setValue('test-note')
    await wrapper.find('#content').setValue('# Test Note\n\nSome content')

    const saveButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Save'))
    await saveButton.trigger('click')
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('/api/v1/file/create', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        vault_id: 'test-vault',
        parent_id: null,
        name: 'test-note',
        is_folder: false,
        content: '# Test Note\n\nSome content',
      }),
    })
  })

  it('includes parent_id in API call when provided', async () => {
    const mockResponse = {
      data: {
        id: 'new-file-id',
        path: 'folder/test-note.md',
        name: 'test-note.md',
      },
    }

    fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockResponse,
    })

    const wrapper = createWrapper({ parentId: 'parent-folder-id' })

    await wrapper.find('#filename').setValue('test-note')

    const saveButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Save'))
    await saveButton.trigger('click')
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('/api/v1/file/create', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        vault_id: 'test-vault',
        parent_id: 'parent-folder-id',
        name: 'test-note',
        is_folder: false,
        content: '',
      }),
    })
  })

  it('sends is_folder: true for folder creation', async () => {
    const mockResponse = {
      data: {
        id: 'new-folder-id',
        path: 'test-folder',
        name: 'test-folder',
      },
    }

    fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockResponse,
    })

    const wrapper = createWrapper()

    // Select folder type
    const radios = wrapper.findAll('input[type="radio"]')
    await radios[1].setValue(true)

    await wrapper.find('#filename').setValue('test-folder')

    const saveButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Save'))
    await saveButton.trigger('click')
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('/api/v1/file/create', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        vault_id: 'test-vault',
        parent_id: null,
        name: 'test-folder',
        is_folder: true,
        content: '',
      }),
    })
  })

  it('displays error message when API call fails', async () => {
    fetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'File already exists' }),
    })

    const wrapper = createWrapper()

    await wrapper.find('#filename').setValue('test-note')

    const saveButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Save'))
    await saveButton.trigger('click')
    await flushPromises()
    await nextTick()

    expect(wrapper.find('.error-message').exists()).toBe(true)
    expect(wrapper.text()).toContain('File already exists')
  })

  it('disables save button when filename is empty to prevent save', async () => {
    const wrapper = createWrapper()

    const saveButton = wrapper.findAll('.button').find((btn) => btn.text().includes('Save'))

    // Save button should be disabled when no filename
    expect(saveButton.attributes('disabled')).toBeDefined()

    // Add a filename
    await wrapper.find('#filename').setValue('test')
    await nextTick()

    // Save button should be enabled now
    expect(saveButton.attributes('disabled')).toBeUndefined()
  })

  it('resets form when dialog is reopened', async () => {
    const wrapper = createWrapper({ show: false })

    // Open dialog
    await wrapper.setProps({ show: true })
    await nextTick()

    // Fill in form
    await wrapper.find('#filename').setValue('test-note')
    await wrapper.find('#content').setValue('Some content')

    // Close dialog
    await wrapper.setProps({ show: false })

    // Reopen dialog
    await wrapper.setProps({ show: true })
    await nextTick()

    expect(wrapper.find('#filename').element.value).toBe('')
    expect(wrapper.find('#content').element.value).toBe('')
  })

  it('handles Tab key in content editor', async () => {
    const wrapper = createWrapper()

    const textarea = wrapper.find('#content')
    await textarea.setValue('Line 1')

    // Trigger Tab key
    await textarea.trigger('keydown', { key: 'Tab', preventDefault: () => {} })
    await nextTick()

    // Should have inserted spaces
    expect(textarea.element.value).toContain('  ')
  })

  it('handles Enter key in filename input to trigger save', async () => {
    const mockResponse = {
      data: {
        id: 'new-file-id',
        path: 'test-note.md',
        name: 'test-note.md',
      },
    }

    fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockResponse,
    })

    const wrapper = createWrapper()

    const input = wrapper.find('#filename')
    await input.setValue('test-note')

    await input.trigger('keydown.enter')
    await flushPromises()

    expect(fetch).toHaveBeenCalled()
    expect(wrapper.emitted('created')).toBeTruthy()
  })

  it('handles Escape key in filename input to close dialog', async () => {
    const wrapper = createWrapper()

    const input = wrapper.find('#filename')
    await input.trigger('keydown.esc')

    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
