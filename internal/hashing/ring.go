package hashing

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

type Ring struct {
	nodes    []string
	vNodes   map[uint32]string
	sorted   []uint32
	replicas int
	mu       sync.RWMutex
}

func NewRing(replicas int) *Ring {
	return &Ring{
		vNodes:   make(map[uint32]string),
		replicas: replicas,
	}
}

func (r *Ring) AddNode(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodes = append(r.nodes, node)
	for i := 0; i < r.replicas; i++ {
		hash := r.hash(node + strconv.Itoa(i))
		r.vNodes[hash] = node
		r.sorted = append(r.sorted, hash)
	}
	sort.Slice(r.sorted, func(i, j int) bool {
		return r.sorted[i] < r.sorted[j]
	})
}

func (r *Ring) GetNode(key string) string {
	nodes := r.GetNodes(key, 1)
	if len(nodes) == 0 {
		return ""
	}
	return nodes[0]
}

func (r *Ring) GetNodes(key string, n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodes) == 0 {
		return nil
	}

	hash := r.hash(key)
	idx := sort.Search(len(r.sorted), func(i int) bool {
		return r.sorted[i] >= hash
	})

	if idx == len(r.sorted) {
		idx = 0
	}

	distinctNodes := make(map[string]bool)
	var result []string

	// walk the ring
	for len(result) < n && len(distinctNodes) < len(r.nodes) {
		node := r.vNodes[r.sorted[idx]]
		if !distinctNodes[node] {
			distinctNodes[node] = true
			result = append(result, node)
		}
		idx++
		if idx >= len(r.sorted) {
			idx = 0
		}
	}

	return result
}

func (r *Ring) hash(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}
