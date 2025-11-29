package cache

import "time"

type Entry struct {
	Value      interface{}
	Expiration time.Time
}
