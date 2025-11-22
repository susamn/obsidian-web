package utils

import "time"

// Channel and buffer sizes
const (
	DefaultEventBufferSize = 10000
	DefaultWorkerQueueSize = 1000
	DefaultSSEChannelSize  = 1000
	DefaultBatchSize       = 100
	DefaultCacheSize       = 1000
	NumWorkers             = 10
)

// Database constants
const (
	MaxOpenConnections = 10
	ConnMaxLifetime    = 5 * time.Minute
	BusyTimeout        = 5000 // milliseconds
)

// Indexing constants
const (
	DefaultIndexEventBuffer   = 10000
	DefaultIndexBatchSize     = 50
	DefaultIndexFlushInterval = 500 * time.Millisecond
)

// SSE constants
const (
	SSEPingInterval    = 30 * time.Second
	SSEChannelBuffer   = 10
	SSEBroadcastBuffer = 100
	SSEBatchThreshold  = 5
	SSEFlushInterval   = 1 * time.Second
	SSEMaxBatchSize    = 100
	SSEWriteTimeout    = 100 * time.Millisecond
)

// Cache constants
const (
	ExplorerCacheTTL  = 5 * time.Minute
	ExplorerCacheSize = 1000
)
