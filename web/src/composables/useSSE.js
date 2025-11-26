import { ref, onUnmounted } from 'vue'

/**
 * Composable for managing Server-Sent Events (SSE) connection
 * @param {Object} callbacks - Event handler callbacks
 * @param {Function} callbacks.onBulkUpdate - Handler for bulk_update events (many files changed)
 * @param {Function} callbacks.onConnected - Handler for connection established
 * @param {Function} callbacks.onError - Handler for errors
 */
export function useSSE(callbacks = {}) {
  const connected = ref(false)
  const error = ref(null)
  const reconnectAttempts = ref(0)
  const pendingEvents = ref(0) // Number of pending events in sync channel
  const maxReconnectAttempts = 5
  const reconnectDelay = 3000 // 3 seconds

  let eventSource = null
  let reconnectTimeout = null
  let currentVaultId = null

  /**
   * Connect to SSE endpoint
   * @param {string} vaultId - The vault ID to connect to
   */
  const connect = (vaultId) => {
    if (!vaultId) {
      error.value = 'Vault ID is required'
      return
    }

    currentVaultId = vaultId

    // Close existing connection if any
    disconnect()

    try {
      const sseUrl = `/api/v1/sse/${vaultId}`
      console.log('[SSE] Connecting to:', sseUrl)

      eventSource = new EventSource(sseUrl)

      // Connection opened
      eventSource.onopen = () => {
        console.log('[SSE] Connection opened')
        connected.value = true
        error.value = null
        reconnectAttempts.value = 0
      }

      // Connection established event
      eventSource.addEventListener('connected', (event) => {
        console.log('[SSE] Connected event:', event.data)
        try {
          const data = JSON.parse(event.data)
          console.log('[SSE] Client ID:', data.client_id)
          if (callbacks.onConnected) {
            callbacks.onConnected(data)
          }
        } catch (err) {
          console.error('[SSE] Error parsing connected event:', err)
        }
      })

      // Bulk update event (consolidated file changes)
      eventSource.addEventListener('bulk_update', (event) => {
        console.log('[SSE] Bulk update:', event.data)
        try {
          const data = JSON.parse(event.data)
          console.log(
            `[SSE] Bulk update: ${data.summary.created} created, ${data.summary.modified} modified, ${data.summary.deleted} deleted`
          )
          if (callbacks.onBulkUpdate) {
            callbacks.onBulkUpdate(data)
          }
        } catch (err) {
          console.error('[SSE] Error parsing bulk_update event:', err)
        }
      })

      // Ping event (keep-alive and UI state updates)
      eventSource.addEventListener('ping', (event) => {
        console.debug('[SSE] Ping received')
        try {
          const data = JSON.parse(event.data)
          // Update pending events count from ping
          if (data.pending_events !== undefined) {
            pendingEvents.value = data.pending_events
            console.debug(`[SSE] Pending events: ${data.pending_events}`)
          }
        } catch (err) {
          // Ping might not have data, that's okay
          console.debug('[SSE] Ping event (no data)')
        }
      })

      // Error handler
      eventSource.onerror = (event) => {
        console.error('[SSE] Connection error:', event)
        connected.value = false

        // Only attempt to reconnect if we haven't exceeded max attempts
        if (reconnectAttempts.value < maxReconnectAttempts) {
          reconnectAttempts.value++
          error.value = `Connection lost. Reconnecting... (${reconnectAttempts.value}/${maxReconnectAttempts})`

          console.log(
            `[SSE] Reconnecting in ${reconnectDelay}ms (attempt ${reconnectAttempts.value}/${maxReconnectAttempts})`
          )

          reconnectTimeout = setTimeout(() => {
            connect(currentVaultId)
          }, reconnectDelay)
        } else {
          error.value = 'Connection failed. Maximum reconnection attempts reached.'
          disconnect()

          if (callbacks.onError) {
            callbacks.onError(new Error('Max reconnection attempts reached'))
          }
        }
      }
    } catch (err) {
      console.error('[SSE] Error creating EventSource:', err)
      error.value = err.message
      connected.value = false

      if (callbacks.onError) {
        callbacks.onError(err)
      }
    }
  }

  /**
   * Disconnect from SSE endpoint
   */
  const disconnect = () => {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }

    if (eventSource) {
      console.log('[SSE] Disconnecting')
      eventSource.close()
      eventSource = null
    }

    connected.value = false
  }

  /**
   * Reconnect to SSE endpoint
   */
  const reconnect = () => {
    reconnectAttempts.value = 0
    if (currentVaultId) {
      connect(currentVaultId)
    }
  }

  // Cleanup on component unmount
  onUnmounted(() => {
    disconnect()
  })

  return {
    connected,
    error,
    reconnectAttempts,
    pendingEvents,
    connect,
    disconnect,
    reconnect,
  }
}
