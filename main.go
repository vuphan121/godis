package main

import (
	"fmt"
	"github.com/vuphan121/godis/cache"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	m := make(map[int]int)

	// ------------ INSERT TEST ------------
	numKeys := 100_000

	startInsert := time.Now()
	for i := 0; i < numKeys; i++ {
		m[i] = rand.Int()
	}
	insertDuration := time.Since(startInsert).Nanoseconds()

	// ------------ READ TEST ------------
	numReads := 200_000 // read twice as many times
	startRead := time.Now()
	var sum int
	for i := 0; i < numReads; i++ {
		key := rand.Intn(numKeys)
		sum += m[key] // normal read
	}
	readDuration := time.Since(startRead).Nanoseconds()

	// ------------ OUTPUT ------------
	fmt.Printf("Inserted %d keys in %d ns (avg %d ns/op)\n",
		numKeys, insertDuration, insertDuration/int64(numKeys))

	fmt.Printf("Performed %d reads in %d ns (avg %d ns/op)\n",
		numReads, readDuration, readDuration/int64(numReads))

	// Use sum so compiler cannot optimize out reads
	fmt.Println("Dummy sum:", sum)
}

func checkHot(c *cache.Cache, key string) {
	hotIdx := getShardIndex(key, len(c.HotShards))
	coldIdx := getShardIndex(key, len(c.ColdShards))

	_, hotOK := c.HotShards[hotIdx].Get(key)
	_, coldOK := c.ColdShards[coldIdx].Get(key)

	if hotOK {
		fmt.Printf("HOT 🔥 Key '%s' is in HOT shard %d\n", key, hotIdx)
		return
	}
	if coldOK {
		fmt.Printf("cold ❄️ Key '%s' is in COLD shard %d\n", key, coldIdx)
		return
	}

	fmt.Printf("Key '%s' not found in any shard\n", key)
}

// We must re-import your shard hash function
func getShardIndex(key string, shardCount int) int {
	h := fnv32(key)
	return int(h % uint32(shardCount))
}

func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return hash
}

func testGet(c *cache.Cache, key string) {
	v, ok := c.Get(key)
	if !ok {
		fmt.Printf("❌ Get(%s) returned false\n", key)
		return
	}
	fmt.Printf("✔️  Get(%s) = %v\n", key, v)
}
