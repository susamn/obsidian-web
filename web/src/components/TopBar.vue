<template>
  <div class="top-bar">
    <div class="logo">
      Obsidian Web
    </div>
    <div class="actions">
      <!-- Combined status indicator: connection + progress -->
      <div
        class="status-widget"
        :class="statusClass"
        :title="statusTitle"
      >
        <div class="status-icon">
          <i
            v-if="pendingEvents > 0"
            class="fas fa-sync fa-spin"
          />
          <i
            v-else-if="connected && !error"
            class="fas fa-circle"
          />
          <i
            v-else-if="error"
            class="fas fa-exclamation-circle"
          />
          <i
            v-else
            class="fas fa-circle-notch fa-spin"
          />
        </div>
        <div class="status-text">
          <template v-if="connected && !error">
            Live<span
              v-if="pendingEvents > 0"
              class="pending-count"
            >({{ pendingEvents }})</span>
          </template>
          <template v-else-if="error">
            Offline
          </template>
          <template v-else>
            Connecting
          </template>
        </div>
      </div>
      <div
        class="settings-icon"
        @click="goToSettings"
      >
        ⚙️
      </div>
    </div>
  </div>
</template>

<script>
import { ref, watch, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useSSE } from '../composables/useSSE'

export default {
  name: 'TopBar',
  setup() {
    const route = useRoute()
    const pendingEvents = ref(0)
    const connected = ref(false)
    const error = ref(null)

    // Setup SSE connection
    const {
      connected: sseConnected,
      error: sseError,
      pendingEvents: ssePendingEvents,
      connect,
      disconnect,
    } = useSSE({})

    // Watch for route changes to connect/disconnect SSE
    watch(
      () => route.params.id,
      (newVaultId, oldVaultId) => {
        if (oldVaultId) {
          disconnect()
        }

        if (newVaultId) {
          connect(newVaultId)
        }
      },
      { immediate: true }
    )

    // Watch SSE state and update local refs
    watch(ssePendingEvents, (newValue) => {
      pendingEvents.value = newValue
    })

    watch(sseConnected, (newValue) => {
      connected.value = newValue
    })

    watch(sseError, (newValue) => {
      error.value = newValue
    })

    // Computed properties for status
    const statusClass = computed(() => {
      if (error.value) return 'error'
      if (connected.value && pendingEvents.value > 0) return 'live-syncing'
      if (connected.value) return 'connected'
      return 'connecting'
    })

    const statusTitle = computed(() => {
      if (error.value) return `Offline: ${error.value}`
      if (connected.value && pendingEvents.value > 0) {
        return `Live - ${pendingEvents.value} pending events`
      }
      if (connected.value) return 'Live updates enabled'
      return 'Connecting to server...'
    })

    return {
      pendingEvents,
      connected,
      error,
      statusClass,
      statusTitle,
    }
  },
  methods: {
    goToSettings() {
      this.$router.push({ name: 'settings' })
    },
  },
}
</script>

<style scoped>
.top-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 1rem;
  background-color: var(--background-color-light);
  border-bottom: 1px solid var(--border-color);
  color: var(--text-color);
}

.logo {
  font-weight: bold;
}

.actions {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.settings-icon {
  cursor: pointer;
  font-size: 1.5rem;
}

/* Combined status widget */
.status-widget {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  font-size: 0.85rem;
  cursor: default;
  transition: background-color 0.2s;
}

.status-icon {
  display: flex;
  align-items: center;
  font-size: 0.7rem;
}

.status-text {
  font-weight: 500;
  white-space: nowrap;
}

/* Pending count styling */
.pending-count {
  margin-left: 0.2em;
  font-weight: 600;
}

/* Status states */
.status-widget.connected {
  background-color: rgba(152, 195, 121, 0.1);
  color: #98c379; /* Keep distinct green for success */
}

.status-widget.connected .status-icon {
  color: #98c379;
}

.status-widget.live-syncing {
  background-color: color-mix(in srgb, var(--primary-color), transparent 85%);
  color: var(--primary-color);
  animation: pulse-glow 2s ease-in-out infinite;
  border: 1px solid color-mix(in srgb, var(--primary-color), transparent 70%);
}

.status-widget.live-syncing .status-icon {
  color: var(--primary-color);
  animation: rotate-pulse 2s linear infinite;
}

.status-widget.live-syncing .pending-count {
  color: var(--primary-color);
  animation: scale-pulse 1s ease-in-out infinite;
}

.status-widget.connecting {
  background-color: rgba(229, 192, 123, 0.1);
  color: #e5c07b;
}

.status-widget.connecting .status-icon {
  color: #e5c07b;
}

.status-widget.error {
  background-color: rgba(224, 108, 117, 0.1);
  color: #e06c75;
}

.status-widget.error .status-icon {
  color: #e06c75;
}

/* Animations */
@keyframes pulse-glow {
  0%,
  100% {
    box-shadow: 0 0 5px color-mix(in srgb, var(--primary-color), transparent 70%);
  }
  50% {
    box-shadow: 0 0 15px color-mix(in srgb, var(--primary-color), transparent 40%);
  }
}

@keyframes rotate-pulse {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

@keyframes scale-pulse {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.1);
  }
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
