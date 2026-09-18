package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vuphan121/godis/config"
)

type historyOperation struct {
	kind      byte
	key       string
	value     int
	ttl       time.Duration
	at        time.Time
	start     uint64
	end       uint64
	getValue  int
	getOK     bool
	deleteOK  bool
	setFailed bool
}

type historyState struct {
	present   bool
	key       string
	value     int
	expiresAt int64
}

type historyMemo struct {
	mask      uint64
	present   bool
	key       string
	value     int
	expiresAt int64
}

type manualClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (clock *manualClock) Now() time.Time {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.now
}

func (clock *manualClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	clock.mu.Unlock()
}

func TestConcurrentHistoryLinearizable(t *testing.T) {
	for round := 0; round < 200; round++ {
		cache := newTestCache(t, config.WithCapacity(32, 32))
		history := []historyOperation{
			{kind: 's', value: round*10 + 1},
			{kind: 'g'},
			{kind: 'd'},
			{kind: 's', value: round*10 + 2},
			{kind: 'g'},
			{kind: 's', value: round*10 + 3},
			{kind: 'd'},
			{kind: 'g'},
			{kind: 's', value: round*10 + 4},
			{kind: 'g'},
			{kind: 'd'},
			{kind: 'g'},
		}
		runHistory(cache, history, 4, nil)
		if !historyIsLinearizable(history) {
			t.Fatalf("round %d produced a non-linearizable history: %+v", round, history)
		}
		assertCacheInvariants(t, cache)
		cache.Close()
	}
}

func TestConcurrentTTLHistoryLinearizable(t *testing.T) {
	for round := 0; round < 100; round++ {
		cache := newTestCache(t)
		clock := &manualClock{now: time.Unix(1_000, 0)}
		cache.now = clock.Now
		history := []historyOperation{
			{kind: 't', value: round*10 + 1, ttl: time.Second},
			{kind: 'g'},
			{kind: 't', value: round*10 + 2, ttl: 2 * time.Second},
			{kind: 'g'},
			{kind: 'g'},
			{kind: 'g'},
			{kind: 't', value: round*10 + 3, ttl: time.Second},
			{kind: 'g'},
			{kind: 'g'},
			{kind: 's', value: round*10 + 4},
			{kind: 'g'},
			{kind: 'g'},
		}
		runHistory(cache, history, 4, func(wave int) {
			if wave == 1 {
				clock.Advance(time.Second)
			} else if wave > 1 {
				clock.Advance(500 * time.Millisecond)
			}
		})
		if !historyIsLinearizable(history) {
			t.Fatalf("round %d produced a non-linearizable TTL history: %+v", round, history)
		}
		assertCacheInvariants(t, cache)
		cache.Close()
	}
}

func TestConcurrentCapacityHistoryLinearizable(t *testing.T) {
	for round := 0; round < 100; round++ {
		cache := newTestCache(t, config.WithCapacity(1, 1))
		history := []historyOperation{
			{kind: 's', key: "a", value: round*10 + 1},
			{kind: 's', key: "b", value: round*10 + 2},
			{kind: 'g', key: "a"},
			{kind: 'g', key: "b"},
			{kind: 's', key: "c", value: round*10 + 3},
			{kind: 'g', key: "b"},
			{kind: 'd', key: "a"},
			{kind: 'g', key: "c"},
			{kind: 'd', key: "c"},
			{kind: 's', key: "a", value: round*10 + 4},
			{kind: 'g', key: "a"},
			{kind: 'g', key: "c"},
		}
		runHistory(cache, history, 4, nil)
		if !historyIsLinearizable(history) {
			t.Fatalf("round %d produced a non-linearizable capacity history: %+v", round, history)
		}
		assertCacheInvariants(t, cache)
		cache.Close()
	}
}

func TestConcurrentTierMaintenanceInvariants(t *testing.T) {
	cache := newTestCache(t,
		config.WithCapacity(64, 16),
		config.WithHotMinHits(1),
		config.WithColdCleanup(2*time.Millisecond, 1, 1),
		config.WithColdCleanupMaxSample(64),
		config.WithHotDemotion(2*time.Millisecond, 1, 1, 64),
	)
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		worker := worker
		wait.Add(1)
		go func() {
			defer wait.Done()
			for operation := 0; operation < 300; operation++ {
				key := string(rune('a' + (worker+operation)%16))
				switch operation % 4 {
				case 0:
					_ = cache.Set(key, operation, 5*time.Millisecond)
				case 1, 2:
					cache.Get(key)
				case 3:
					cache.Delete(key)
				}
			}
		}()
	}
	wait.Wait()
	assertCacheInvariants(t, cache)
}

func runHistory(cache *Cache, history []historyOperation, waveSize int, beforeWave func(int)) {
	var sequence atomic.Uint64
	for start := 0; start < len(history); start += waveSize {
		if beforeWave != nil {
			beforeWave(start / waveSize)
		}
		var wait sync.WaitGroup
		gate := make(chan struct{})
		end := start + waveSize
		if end > len(history) {
			end = len(history)
		}
		for index := start; index < end; index++ {
			index := index
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-gate
				operation := &history[index]
				operation.at = cache.now()
				operation.start = sequence.Add(1)
				key := operation.key
				if key == "" {
					key = "key"
				}
				switch operation.kind {
				case 's':
					operation.setFailed = cache.Set(key, operation.value) != nil
				case 't':
					operation.setFailed = cache.Set(key, operation.value, operation.ttl) != nil
				case 'g':
					value, ok := cache.Get(key)
					operation.getOK = ok
					if ok {
						operation.getValue = value.(int)
					}
				case 'd':
					operation.deleteOK = cache.Delete(key)
				}
				operation.end = sequence.Add(1)
			}()
		}
		close(gate)
		wait.Wait()
	}
}

func TestHistoryCheckerRejectsImpossibleResult(t *testing.T) {
	history := []historyOperation{
		{kind: 's', value: 1, start: 1, end: 2},
		{kind: 'g', getValue: 2, getOK: true, start: 3, end: 4},
	}
	if historyIsLinearizable(history) {
		t.Fatal("checker accepted an impossible history")
	}
}

func TestHistoryCheckerAcceptsValidResult(t *testing.T) {
	history := []historyOperation{
		{kind: 's', value: 1, start: 1, end: 4},
		{kind: 'g', getValue: 1, getOK: true, start: 2, end: 3},
	}
	if !historyIsLinearizable(history) {
		t.Fatal("checker rejected a valid history")
	}
}

func historyIsLinearizable(history []historyOperation) bool {
	predecessors := make([]uint64, len(history))
	for before := range history {
		for after := range history {
			if history[before].end < history[after].start {
				predecessors[after] |= uint64(1) << before
			}
		}
	}
	complete := uint64(1)<<len(history) - 1
	failed := make(map[historyMemo]struct{})
	var search func(uint64, historyState) bool
	search = func(mask uint64, state historyState) bool {
		if mask == complete {
			return true
		}
		memo := historyMemo{mask: mask, present: state.present, key: state.key, value: state.value, expiresAt: state.expiresAt}
		if _, seen := failed[memo]; seen {
			return false
		}
		for index, operation := range history {
			bit := uint64(1) << index
			if mask&bit != 0 || predecessors[index]&^mask != 0 {
				continue
			}
			next, valid := applyHistoryOperation(state, operation)
			if valid && search(mask|bit, next) {
				return true
			}
		}
		failed[memo] = struct{}{}
		return false
	}
	return search(0, historyState{})
}

func applyHistoryOperation(state historyState, operation historyOperation) (historyState, bool) {
	if state.present && state.expiresAt != 0 && !operation.at.IsZero() && operation.at.UnixNano() >= state.expiresAt {
		state = historyState{}
	}
	key := operation.key
	if key == "" {
		key = "key"
	}
	switch operation.kind {
	case 's':
		if operation.setFailed {
			return state, false
		}
		return historyState{present: true, key: key, value: operation.value}, true
	case 't':
		if operation.setFailed {
			return state, false
		}
		return historyState{present: true, key: key, value: operation.value, expiresAt: operation.at.Add(operation.ttl).UnixNano()}, true
	case 'g':
		present := state.present && state.key == key
		if operation.getOK != present || operation.getOK && operation.getValue != state.value {
			return state, false
		}
		return state, true
	case 'd':
		present := state.present && state.key == key
		if operation.deleteOK != present {
			return state, false
		}
		if present {
			return historyState{}, true
		}
		return state, true
	default:
		return state, false
	}
}
