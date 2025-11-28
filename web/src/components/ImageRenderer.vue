<template>
  <div class="image-renderer">
    <div class="image-container">
      <img
        :src="assetUrl"
        :alt="fileName"
        class="rendered-image"
        @load="handleLoad"
        @error="handleError"
      >
      <div
        v-if="loading"
        class="loading-overlay"
      >
        <i class="fas fa-spinner fa-spin" />
      </div>
      <div
        v-if="error"
        class="error-message"
      >
        <i class="fas fa-exclamation-triangle" />
        <span>Failed to load image</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  vaultId: {
    type: String,
    required: true,
  },
  fileId: {
    type: String,
    required: true,
  },
  fileName: {
    type: String,
    default: 'Image',
  },
})

const loading = ref(true)
const error = ref(false)

const assetUrl = computed(() => {
  return `/api/v1/assets/${props.vaultId}/${props.fileId}`
})

const handleLoad = () => {
  loading.value = false
}

const handleError = () => {
  loading.value = false
  error.value = true
}
</script>

<style scoped>
.image-renderer {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: auto;
  padding: 2rem;
  background-color: var(--background-color-alt, rgba(0, 0, 0, 0.02));
  height: 100%;
}

.image-container {
  position: relative;
  max-width: 100%;
  max-height: 100%;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  border-radius: 8px;
  overflow: hidden;
}

.rendered-image {
  display: block;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.loading-overlay,
.error-message {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background-color: rgba(255, 255, 255, 0.8);
  color: var(--text-color-secondary);
}

.error-message {
  color: #ef4444;
  gap: 0.5rem;
}
</style>
