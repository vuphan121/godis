package cache

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"math/bits"
	"sort"
	"sync"
)

const maxCMSCells = 10_000_000

type CountMinSketch struct {
	mu         sync.RWMutex
	depth      int
	width      int
	generation uint64
	table      [][]counterCell
}

type counterCell struct {
	value      uint
	generation uint64
}

func NewCountMinSketch(depth, width int) (*CountMinSketch, error) {
	if depth <= 0 || width <= 0 {
		return nil, fmt.Errorf("Count-Min Sketch dimensions must be positive")
	}
	if depth > maxCMSCells/width {
		return nil, fmt.Errorf("Count-Min Sketch must not exceed %d counters", maxCMSCells)
	}
	table := make([][]counterCell, depth)
	for index := range table {
		table[index] = make([]counterCell, width)
	}
	return &CountMinSketch{depth: depth, width: width, table: table}, nil
}

func (cms *CountMinSketch) Add(key string) {
	cms.mu.Lock()
	defer cms.mu.Unlock()
	for row := 0; row < cms.depth; row++ {
		index := cms.hash(key, uint(row)) % uint(cms.width)
		cell := &cms.table[row][index]
		cell.value = agedValue(*cell, cms.generation)
		cell.generation = cms.generation
		if cell.value < ^uint(0) {
			cell.value++
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
	if cms.generation != ^uint64(0) {
		cms.generation++
		return
	}
	for row := range cms.table {
		for column := range cms.table[row] {
			cell := &cms.table[row][column]
			cell.value = agedValue(*cell, cms.generation)
			cell.generation = 0
		}
	}
	cms.generation = 1
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
		if value := agedValue(cms.table[row][index], cms.generation); value < minimum {
			minimum = value
		}
	}
	return minimum
}

func agedValue(cell counterCell, generation uint64) uint {
	if generation <= cell.generation {
		return cell.value
	}
	age := generation - cell.generation
	if age >= uint64(bits.UintSize) {
		return 0
	}
	return cell.value >> age
}

func (cms *CountMinSketch) hash(key string, seed uint) uint {
	hash := fnv.New32a()
	var seedBytes [8]byte
	binary.LittleEndian.PutUint64(seedBytes[:], uint64(seed))
	_, _ = hash.Write(seedBytes[:])
	_, _ = hash.Write([]byte(key))
	return uint(hash.Sum32())
}
