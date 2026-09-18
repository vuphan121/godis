package cache

import "time"

type entry struct {
	value      any
	expiration time.Time
	createdAt  time.Time
}

func (e *entry) expired(now time.Time) bool {
	return !e.expiration.IsZero() && !now.Before(e.expiration)
}

func (e *entry) hotEligible(now time.Time, minimumTTL time.Duration) bool {
	return e.expiration.IsZero() || e.expiration.Sub(now) >= minimumTTL
}

type sampledEntry struct {
	key   string
	entry *entry
}
