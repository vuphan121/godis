package cache_test

import (
	"fmt"
	"log"
	"time"

	"github.com/vuphan121/godis/cache"
	"github.com/vuphan121/godis/config"
)

func ExampleCache() {
	c, err := cache.NewCache(
		config.WithCapacity(1_000, 32),
		config.WithDefaultTTL(5*time.Minute),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if err := c.Set("user:42", "Ada"); err != nil {
		log.Fatal(err)
	}
	if value, ok := c.Get("user:42"); ok {
		fmt.Println(value)
	}

	// Output: Ada
}
