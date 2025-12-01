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
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodes) == 0 {
		return ""
	}

	hash := r.hash(key)
	idx := sort.Search(len(r.sorted), func(i int) bool {
		return r.sorted[i] >= hash
	})

	if idx == len(r.sorted) {
		idx = 0
	}

	return r.vNodes[r.sorted[idx]]
}

func (r *Ring) hash(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}
