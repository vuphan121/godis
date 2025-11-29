package cache

import (
	"hash/fnv"
	"math/rand"
	"sort"
)

type CountMinSketch struct {
	depth int
	width int
	table [][]uint
}

func NewCountMinSketch(depth, width int) *CountMinSketch {
	table := make([][]uint, depth)
	for i := range table {
		table[i] = make([]uint, width)
	}
	return &CountMinSketch{depth, width, table}
}

func (cms *CountMinSketch) Add(key string) {
	for i := 0; i < cms.depth; i++ {
		index := cms.hash(key, uint(i)) % uint(cms.width)
		cms.table[i][index]++
	}
}

func (cms *CountMinSketch) Count(key string) uint {
	minVal := ^uint(0)
	for i := 0; i < cms.depth; i++ {
		index := cms.hash(key, uint(i)) % uint(cms.width)
		if cms.table[i][index] < minVal {
			minVal = cms.table[i][index]
		}
	}
	return minVal
}

func (cms *CountMinSketch) Reset(key string) {
	for i := 0; i < cms.depth; i++ {
		index := cms.hash(key, uint(i)) % uint(cms.width)
		cms.table[i][index] = 0
	}
}

func (cms *CountMinSketch) hash(key string, seed uint) uint {
	h := fnv.New32a()
	h.Write([]byte(key))
	return uint(h.Sum32()) + seed*0x9e3779b9
}

func (cms *CountMinSketch) ApproxTopThreshold(percentage float64, sampleSize int) uint {
	if sampleSize <= 0 {
		sampleSize = 1000
	}

	counts := make([]uint, 0, sampleSize)
	for i := 0; i < sampleSize; i++ {
		d := rand.Intn(cms.depth)
		w := rand.Intn(cms.width)
		counts = append(counts, cms.table[d][w])
	}

	sort.Slice(counts, func(i, j int) bool { return counts[i] > counts[j] })

	index := int(float64(len(counts)) * percentage)
	if index >= len(counts) {
		index = len(counts) - 1
	}
	return counts[index]
}
