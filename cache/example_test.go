package cache_test

import (
	"testing"
	"time"

	"github.com/vuphan121/godis/cache"
	"github.com/vuphan121/godis/config"
)

func TestPublicUsage(t *testing.T) {
	c, err := cache.NewCache(
		config.WithCapacity(1_000, 32),
		config.WithDefaultTTL(5*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Set("user:42", "Ada"); err != nil {
		t.Fatal(err)
	}
	if value, ok := c.Get("user:42"); !ok || value != "Ada" {
		t.Fatalf("Get = (%v, %v), want (Ada, true)", value, ok)
	}
}
