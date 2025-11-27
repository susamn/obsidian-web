import { ref, onUnmounted } from 'vue'

/**
 * Composable for managing Server-Sent Events (SSE) connection
 * @param {Object} callbacks - Event handler callbacks
 * @param {Function} callbacks.onBulkProcess - Handler for bulk_process events (file changes)
 * @param {Function} callbacks.onPing - Handler for ping events (keep-alive)
 * @param {Function} callbacks.onRefresh - Handler for refresh events (full tree refresh)
 * @param {Function} callbacks.onError - Handler for error events
 * @param {Function} callbacks.onConnected - Handler for connection established
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

      // Generic event handler that processes all SSE events
      const handleSSEEvent = (event) => {
        try {
          const data = JSON.parse(event.data)

          console.log(`[SSE] Received event:`, data)

          // Update pending count from all events
          if (data.pending_count !== undefined) {
            pendingEvents.value = data.pending_count
            console.debug(`[SSE] Pending events: ${data.pending_count}`)
          }

          // Route to specific callbacks based on event type
          switch (data.type) {
            case 'bulk_process':
              console.log(`[SSE] Bulk process: ${data.changes?.length || 0} file changes`)
              if (callbacks.onBulkProcess) {
                callbacks.onBulkProcess(data)
              }
              break

            case 'ping':
              console.debug('[SSE] Ping received')
              if (callbacks.onPing) {
                callbacks.onPing(data)
              }
              break

            case 'refresh':
              console.log('[SSE] Refresh requested')
              if (callbacks.onRefresh) {
                callbacks.onRefresh(data)
              }
              break

            case 'error':
              console.error('[SSE] Server error:', data.error_message)
              error.value = data.error_message
              if (callbacks.onError) {
                callbacks.onError(new Error(data.error_message))
              }
              break

            case 'connected':
              console.log('[SSE] Connected event:', data)
              connected.value = true
              error.value = null
              reconnectAttempts.value = 0
              if (callbacks.onConnected) {
                callbacks.onConnected(data)
              }
              break

            default:
              console.warn('[SSE] Unknown event type:', data.type)
          }
        } catch (err) {
          console.error('[SSE] Error parsing SSE event:', err, event.data)
        }
      }

      // Listen for all SSE event types
      eventSource.addEventListener('bulk_process', handleSSEEvent)
      eventSource.addEventListener('ping', handleSSEEvent)
      eventSource.addEventListener('refresh', handleSSEEvent)
      eventSource.addEventListener('error', handleSSEEvent)
      eventSource.addEventListener('connected', handleSSEEvent)

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
