package cache

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"sync"
)

const maxCMSCells = 10_000_000

// CountMinSketch is a concurrency-safe approximate frequency counter.
type CountMinSketch struct {
	mu    sync.RWMutex
	depth int
	width int
	table [][]uint
}

// NewCountMinSketch creates a sketch with validated positive dimensions.
func NewCountMinSketch(depth, width int) (*CountMinSketch, error) {
	if depth <= 0 || width <= 0 {
		return nil, fmt.Errorf("Count-Min Sketch dimensions must be positive")
	}
	if depth > maxCMSCells/width {
		return nil, fmt.Errorf("Count-Min Sketch must not exceed %d counters", maxCMSCells)
	}
	table := make([][]uint, depth)
	for index := range table {
		table[index] = make([]uint, width)
	}
	return &CountMinSketch{depth: depth, width: width, table: table}, nil
}

// Add records one access without allowing a counter to wrap around.
func (cms *CountMinSketch) Add(key string) {
	cms.mu.Lock()
	defer cms.mu.Unlock()
	for row := 0; row < cms.depth; row++ {
		index := cms.hash(key, uint(row)) % uint(cms.width)
		if cms.table[row][index] < ^uint(0) {
			cms.table[row][index]++
		}
	}
}

// Count returns the approximate access count for key.
func (cms *CountMinSketch) Count(key string) uint {
	cms.mu.RLock()
	defer cms.mu.RUnlock()
	return cms.countLocked(key)
}

// Decay halves all counters so old traffic does not dominate forever.
func (cms *CountMinSketch) Decay() {
	cms.mu.Lock()
	defer cms.mu.Unlock()
	for row := range cms.table {
		for column := range cms.table[row] {
			cms.table[row][column] /= 2
		}
	}
}

// TopThreshold returns the minimum count in the requested hottest fraction of
// actual resident keys, bounded by minHits.
func (cms *CountMinSketch) TopThreshold(keys []string, fraction float64, minHits uint) uint {
	if len(keys) == 0 {
		return minHits
	}
	cms.mu.RLock()
	counts := make([]uint, 0, len(keys))
	for _, key := range keys {
		counts = append(counts, cms.countLocked(key))
	}
	cms.mu.RUnlock()

	sort.Slice(counts, func(i, j int) bool { return counts[i] > counts[j] })
	index := int(math.Ceil(float64(len(counts))*fraction)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(counts) {
		index = len(counts) - 1
	}
	if counts[index] < minHits {
		return minHits
	}
	return counts[index]
}

func (cms *CountMinSketch) countLocked(key string) uint {
	minimum := ^uint(0)
	for row := 0; row < cms.depth; row++ {
		index := cms.hash(key, uint(row)) % uint(cms.width)
		if value := cms.table[row][index]; value < minimum {
			minimum = value
		}
	}
	return minimum
}

func (cms *CountMinSketch) hash(key string, seed uint) uint {
	hash := fnv.New32a()
	var seedBytes [8]byte
	binary.LittleEndian.PutUint64(seedBytes[:], uint64(seed))
	_, _ = hash.Write(seedBytes[:])
	_, _ = hash.Write([]byte(key))
	return uint(hash.Sum32())
}
