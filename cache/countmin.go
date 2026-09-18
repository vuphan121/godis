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

type CountMinSketch struct {
	mu    sync.RWMutex
	depth int
	width int
	table [][]uint
}

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

func (cms *CountMinSketch) Count(key string) uint {
	cms.mu.RLock()
	defer cms.mu.RUnlock()
	return cms.countLocked(key)
}

func (cms *CountMinSketch) Decay() {
	cms.mu.Lock()
	defer cms.mu.Unlock()
	for row := range cms.table {
		for column := range cms.table[row] {
			cms.table[row][column] /= 2
		}
	}
}

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
