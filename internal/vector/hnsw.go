package vector

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
)

// hnswIndex implements the Hierarchical Navigable Small World algorithm
// for approximate nearest neighbor search.
type hnswIndex struct {
	mu             sync.RWMutex
	dimension      int
	m              int     // Max connections per layer
	mMax0          int     // Max connections for layer 0
	efConstruction int     // Size of dynamic candidate list during construction
	ml             float64 // Level generation factor
	distFunc       DistanceFunc

	entryPoint *node
	maxLevel   int
	nodes      map[string]*node
}

// node represents a single vector in the HNSW graph.
type node struct {
	id      string
	vector  []float32
	level   int
	friends []map[string]*node // friends[level] = map of connected nodes
}

// neighbor represents a candidate neighbor with distance.
type neighbor struct {
	id       string
	distance float32
	node     *node
}

// priorityQueue implements a min-heap for neighbors.
type priorityQueue []*neighbor

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].distance < pq[j].distance }
func (pq priorityQueue) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*neighbor))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	x := old[n-1]
	*pq = old[0 : n-1]
	return x
}

// maxPriorityQueue implements a max-heap for neighbors.
type maxPriorityQueue []*neighbor

func (pq maxPriorityQueue) Len() int           { return len(pq) }
func (pq maxPriorityQueue) Less(i, j int) bool { return pq[i].distance > pq[j].distance }
func (pq maxPriorityQueue) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }

func (pq *maxPriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*neighbor))
}

func (pq *maxPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	x := old[n-1]
	*pq = old[0 : n-1]
	return x
}

// newHNSWIndex creates a new HNSW index.
func newHNSWIndex(dimension, m, efConstruction int, distFunc DistanceFunc) *hnswIndex {
	return &hnswIndex{
		dimension:      dimension,
		m:              m,
		mMax0:          m * 2,
		efConstruction: efConstruction,
		ml:             1.0 / math.Log(float64(m)),
		distFunc:       distFunc,
		nodes:          make(map[string]*node),
	}
}

// randomLevel generates a random level for a new node.
func (h *hnswIndex) randomLevel() int {
	r := rand.Float64()
	level := int(-math.Log(r) * h.ml)
	return level
}

// insert adds a new vector to the index.
func (h *hnswIndex) insert(id string, vector []float32) {
	h.mu.Lock()
	defer h.mu.Unlock()

	level := h.randomLevel()

	newNode := &node{
		id:      id,
		vector:  vector,
		level:   level,
		friends: make([]map[string]*node, level+1),
	}
	for i := 0; i <= level; i++ {
		newNode.friends[i] = make(map[string]*node)
	}

	h.nodes[id] = newNode

	// First node
	if h.entryPoint == nil {
		h.entryPoint = newNode
		h.maxLevel = level
		return
	}

	ep := h.entryPoint
	currentDist := h.distFunc(vector, ep.vector)

	// Traverse from top level to the level of the new node
	for l := h.maxLevel; l > level; l-- {
		changed := true
		for changed {
			changed = false
			for _, friend := range ep.friends[l] {
				d := h.distFunc(vector, friend.vector)
				if d < currentDist {
					ep = friend
					currentDist = d
					changed = true
				}
			}
		}
	}

	// Insert at each level from level down to 0
	for l := min(level, h.maxLevel); l >= 0; l-- {
		neighbors := h.searchLayer(vector, ep, h.efConstruction, l)

		// Select M best neighbors
		maxConn := h.m
		if l == 0 {
			maxConn = h.mMax0
		}

		selectedNeighbors := h.selectNeighbors(neighbors, maxConn)

		// Connect new node to selected neighbors
		for _, n := range selectedNeighbors {
			newNode.friends[l][n.id] = n.node
			n.node.friends[l][id] = newNode
		}

		// Shrink connections if necessary
		for _, n := range selectedNeighbors {
			if len(n.node.friends[l]) > maxConn {
				h.shrinkConnections(n.node, l, maxConn)
			}
		}

		if len(neighbors) > 0 {
			ep = neighbors[0].node
		}
	}

	// Update entry point if needed
	if level > h.maxLevel {
		h.entryPoint = newNode
		h.maxLevel = level
	}
}

// searchLayer performs a greedy search in a single layer.
func (h *hnswIndex) searchLayer(query []float32, ep *node, ef int, level int) []*neighbor {
	visited := make(map[string]bool)
	visited[ep.id] = true

	candidates := &priorityQueue{}
	heap.Init(candidates)

	results := &maxPriorityQueue{}
	heap.Init(results)

	dist := h.distFunc(query, ep.vector)
	heap.Push(candidates, &neighbor{id: ep.id, distance: dist, node: ep})
	heap.Push(results, &neighbor{id: ep.id, distance: dist, node: ep})

	for candidates.Len() > 0 {
		c := heap.Pop(candidates).(*neighbor)

		// Stop if the closest candidate is farther than the farthest result
		if results.Len() >= ef && c.distance > (*results)[0].distance {
			break
		}

		// Explore neighbors
		if level < len(c.node.friends) {
			for _, friend := range c.node.friends[level] {
				if visited[friend.id] {
					continue
				}
				visited[friend.id] = true

				d := h.distFunc(query, friend.vector)
				if results.Len() < ef || d < (*results)[0].distance {
					heap.Push(candidates, &neighbor{id: friend.id, distance: d, node: friend})
					heap.Push(results, &neighbor{id: friend.id, distance: d, node: friend})
					if results.Len() > ef {
						heap.Pop(results)
					}
				}
			}
		}
	}

	// Convert results to slice
	result := make([]*neighbor, results.Len())
	for i := results.Len() - 1; i >= 0; i-- {
		result[i] = heap.Pop(results).(*neighbor)
	}
	return result
}

// selectNeighbors selects the best neighbors using simple selection.
func (h *hnswIndex) selectNeighbors(candidates []*neighbor, m int) []*neighbor {
	if len(candidates) <= m {
		return candidates
	}
	return candidates[:m]
}

// shrinkConnections reduces the number of connections for a node.
func (h *hnswIndex) shrinkConnections(n *node, level int, maxConn int) {
	if len(n.friends[level]) <= maxConn {
		return
	}

	// Convert to slice and sort by distance
	neighbors := make([]*neighbor, 0, len(n.friends[level]))
	for _, friend := range n.friends[level] {
		d := h.distFunc(n.vector, friend.vector)
		neighbors = append(neighbors, &neighbor{id: friend.id, distance: d, node: friend})
	}

	// Sort by distance
	pq := &priorityQueue{}
	heap.Init(pq)
	for _, nb := range neighbors {
		heap.Push(pq, nb)
	}

	// Keep only the closest maxConn neighbors
	newFriends := make(map[string]*node)
	for i := 0; i < maxConn && pq.Len() > 0; i++ {
		nb := heap.Pop(pq).(*neighbor)
		newFriends[nb.id] = nb.node
	}

	// Remove connections from dropped neighbors
	for id, friend := range n.friends[level] {
		if _, kept := newFriends[id]; !kept {
			delete(friend.friends[level], n.id)
		}
	}

	n.friends[level] = newFriends
}

// search performs a k-NN search in the index.
func (h *hnswIndex) search(query []float32, k int, ef int) []*neighbor {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.entryPoint == nil {
		return nil
	}

	if ef < k {
		ef = k
	}

	ep := h.entryPoint
	currentDist := h.distFunc(query, ep.vector)

	// Traverse from top level down to level 1
	for l := h.maxLevel; l >= 1; l-- {
		changed := true
		for changed {
			changed = false
			if l < len(ep.friends) {
				for _, friend := range ep.friends[l] {
					d := h.distFunc(query, friend.vector)
					if d < currentDist {
						ep = friend
						currentDist = d
						changed = true
					}
				}
			}
		}
	}

	// Search in layer 0
	neighbors := h.searchLayer(query, ep, ef, 0)

	// Return top k
	if len(neighbors) > k {
		neighbors = neighbors[:k]
	}
	return neighbors
}

// delete removes a node from the index.
func (h *hnswIndex) delete(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	n, exists := h.nodes[id]
	if !exists {
		return
	}

	// Remove connections to this node from all friends
	for level := 0; level <= n.level; level++ {
		for _, friend := range n.friends[level] {
			delete(friend.friends[level], id)
		}
	}

	delete(h.nodes, id)

	// Update entry point if necessary
	if h.entryPoint != nil && h.entryPoint.id == id {
		h.entryPoint = nil
		h.maxLevel = 0

		// Find new entry point (node with highest level)
		for _, node := range h.nodes {
			if h.entryPoint == nil || node.level > h.maxLevel {
				h.entryPoint = node
				h.maxLevel = node.level
			}
		}
	}
}

// has checks if a node exists in the index.
func (h *hnswIndex) has(id string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.nodes[id]
	return exists
}

// size returns the number of nodes in the index.
func (h *hnswIndex) size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// getVector returns the vector for a given ID.
func (h *hnswIndex) getVector(id string) ([]float32, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n, exists := h.nodes[id]
	if !exists {
		return nil, false
	}
	return n.vector, true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
