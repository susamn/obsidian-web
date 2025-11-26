<template>
  <div
    v-if="show"
    class="dialog-overlay"
    @click.self="handleCancel"
  >
    <div class="dialog-container">
      <div class="dialog-header">
        <h2>{{ isFolder ? 'Create New Folder' : 'Create New Note' }}</h2>
        <button
          class="close-button"
          title="Close"
          @click="handleCancel"
        >
          <i class="fas fa-times" />
        </button>
      </div>

      <div class="dialog-content">
        <!-- Type Selection -->
        <div class="type-selector">
          <label class="type-option">
            <input
              v-model="isFolder"
              type="radio"
              :value="false"
              name="item-type"
            >
            <i class="fas fa-file-alt" />
            <span>Note</span>
          </label>
          <label class="type-option">
            <input
              v-model="isFolder"
              type="radio"
              :value="true"
              name="item-type"
            >
            <i class="fas fa-folder" />
            <span>Folder</span>
          </label>
        </div>

        <!-- Filename Input -->
        <div class="form-group">
          <label for="filename">{{ isFolder ? 'Folder Name' : 'File Name' }}</label>
          <input
            id="filename"
            ref="filenameInput"
            v-model="filename"
            type="text"
            :placeholder="isFolder ? 'Enter folder name' : 'Enter file name (without .md)'"
            class="filename-input"
            @keydown.enter="handleSave"
            @keydown.esc="handleCancel"
          >
          <div
            v-if="!isFolder && filename"
            class="filename-preview"
          >
            Will be saved as: <strong>{{ filenameWithExtension }}</strong>
          </div>
          <div
            v-if="error"
            class="error-message"
          >
            <i class="fas fa-exclamation-circle" />
            {{ error }}
          </div>
        </div>

        <!-- Markdown Editor (only for files) -->
        <div
          v-if="!isFolder"
          class="form-group"
        >
          <label for="content">Content (optional)</label>
          <textarea
            id="content"
            ref="contentEditor"
            v-model="content"
            placeholder="Start typing your markdown content..."
            class="content-editor"
            @keydown.tab.prevent="handleTab"
          />
          <div class="editor-hint">
            <i class="fas fa-info-circle" />
            Use Markdown syntax for formatting. Press Tab for indentation.
          </div>
        </div>
      </div>

      <div class="dialog-footer">
        <button
          class="button button-secondary"
          @click="handleCancel"
        >
          <i class="fas fa-times" />
          Cancel
        </button>
        <button
          class="button button-primary"
          :disabled="!filename || saving"
          @click="handleSave"
        >
          <i
            v-if="!saving"
            class="fas fa-save"
          />
          <i
            v-else
            class="fas fa-spinner fa-spin"
          />
          {{ saving ? 'Saving...' : 'Save' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps({
  show: {
    type: Boolean,
    required: true,
  },
  vaultId: {
    type: String,
    required: true,
  },
  parentId: {
    type: String,
    default: null,
  },
})

const emit = defineEmits(['close', 'created'])

const filenameInput = ref(null)
const contentEditor = ref(null)
const filename = ref('')
const content = ref('')
const isFolder = ref(false)
const saving = ref(false)
const error = ref('')

const filenameWithExtension = computed(() => {
  if (!filename.value) return ''
  const name = filename.value.trim()
  if (name.toLowerCase().endsWith('.md')) {
    return name
  }
  return `${name}.md`
})

// Focus filename input when dialog opens
watch(
  () => props.show,
  (newVal) => {
    if (newVal) {
      // Reset form
      filename.value = ''
      content.value = ''
      isFolder.value = false
      error.value = ''
      saving.value = false

      // Focus input after DOM update
      nextTick(() => {
        if (filenameInput.value) {
          filenameInput.value.focus()
        }
      })
    }
  }
)

const handleTab = (event) => {
  // Insert tab character in textarea
  const textarea = event.target
  const start = textarea.selectionStart
  const end = textarea.selectionEnd

  // Insert 2 spaces (or tab)
  const spaces = '  '
  content.value = content.value.substring(0, start) + spaces + content.value.substring(end)

  // Move cursor
  nextTick(() => {
    textarea.selectionStart = textarea.selectionEnd = start + spaces.length
  })
}

const handleCancel = () => {
  emit('close')
}

const handleSave = async () => {
  if (!filename.value.trim()) {
    error.value = 'Please enter a name'
    return
  }

  error.value = ''
  saving.value = true

  try {
    const response = await fetch('/api/v1/file/create', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        vault_id: props.vaultId,
        parent_id: props.parentId,
        name: filename.value.trim(),
        is_folder: isFolder.value,
        content: isFolder.value ? '' : content.value,
      }),
    })

    if (!response.ok) {
      const errorData = await response.json()
      throw new Error(errorData.error || `Failed to create ${isFolder.value ? 'folder' : 'note'}`)
    }

    const result = await response.json()

    emit('created', result.data)
    emit('close')
  } catch (err) {
    console.error('Failed to create:', err)
    error.value = err.message || 'Failed to create. Please try again.'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

.dialog-container {
  background-color: var(--background-color);
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
  width: 90%;
  max-width: 600px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  animation: slideIn 0.3s ease;
}

@keyframes slideIn {
  from {
    transform: translateY(-30px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.5rem;
  border-bottom: 1px solid var(--border-color);
}

.dialog-header h2 {
  margin: 0;
  font-size: 1.25rem;
  color: var(--text-color);
}

.close-button {
  background: none;
  border: none;
  color: var(--text-color-secondary);
  cursor: pointer;
  font-size: 1.25rem;
  padding: 0.25rem;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.2s;
}

.close-button:hover {
  color: var(--text-color);
}

.dialog-content {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem;
}

.type-selector {
  display: flex;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.type-option {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  border: 2px solid var(--border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  color: var(--text-color);
}

.type-option:hover {
  border-color: var(--primary-color);
  background-color: rgba(59, 130, 246, 0.05);
}

.type-option input[type='radio'] {
  margin: 0;
}

.type-option input[type='radio']:checked + i {
  color: var(--primary-color);
}

.type-option:has(input[type='radio']:checked) {
  border-color: var(--primary-color);
  background-color: rgba(59, 130, 246, 0.1);
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: var(--text-color);
}

.filename-input {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 1rem;
  background-color: var(--background-color-light);
  color: var(--text-color);
  transition: border-color 0.2s;
}

.filename-input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.filename-preview {
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: var(--text-color-secondary);
}

.filename-preview strong {
  color: var(--primary-color);
}

.error-message {
  margin-top: 0.5rem;
  padding: 0.5rem;
  background-color: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 4px;
  color: #ef4444;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.content-editor {
  width: 100%;
  min-height: 200px;
  padding: 0.75rem;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 0.9375rem;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  background-color: var(--background-color-light);
  color: var(--text-color);
  resize: vertical;
  line-height: 1.6;
  transition: border-color 0.2s;
}

.content-editor:focus {
  outline: none;
  border-color: var(--primary-color);
}

.editor-hint {
  margin-top: 0.5rem;
  font-size: 0.8125rem;
  color: var(--text-color-secondary);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.dialog-footer {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
  padding: 1rem 1.5rem;
  border-top: 1px solid var(--border-color);
}

.button {
  padding: 0.625rem 1.25rem;
  border: none;
  border-radius: 4px;
  font-size: 0.9375rem;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  transition: all 0.2s;
}

.button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.button-secondary {
  background-color: var(--background-color-light);
  color: var(--text-color);
  border: 1px solid var(--border-color);
}

.button-secondary:hover:not(:disabled) {
  background-color: var(--border-color);
}

.button-primary {
  background-color: var(--primary-color, #3b82f6);
  color: white;
}

.button-primary:hover:not(:disabled) {
  background-color: #2563eb;
}
</style>
