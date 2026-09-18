package cache

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vuphan121/godis/config"
)

type historyOperation struct {
	kind      byte
	value     int
	start     uint64
	end       uint64
	getValue  int
	getOK     bool
	deleteOK  bool
	setFailed bool
}

type historyState struct {
	present bool
	value   int
}

type historyMemo struct {
	mask    uint64
	present bool
	value   int
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
		var sequence atomic.Uint64
		for start := 0; start < len(history); start += 4 {
			var wait sync.WaitGroup
			gate := make(chan struct{})
			for index := start; index < start+4; index++ {
				index := index
				wait.Add(1)
				go func() {
					defer wait.Done()
					<-gate
					history[index].start = sequence.Add(1)
					switch history[index].kind {
					case 's':
						history[index].setFailed = cache.Set("key", history[index].value) != nil
					case 'g':
						value, ok := cache.Get("key")
						history[index].getOK = ok
						if ok {
							history[index].getValue = value.(int)
						}
					case 'd':
						history[index].deleteOK = cache.Delete("key")
					}
					history[index].end = sequence.Add(1)
				}()
			}
			close(gate)
			wait.Wait()
		}
		if !historyIsLinearizable(history) {
			t.Fatalf("round %d produced a non-linearizable history: %+v", round, history)
		}
		assertCacheInvariants(t, cache)
		cache.Close()
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
		memo := historyMemo{mask: mask, present: state.present, value: state.value}
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
	switch operation.kind {
	case 's':
		if operation.setFailed {
			return state, false
		}
		return historyState{present: true, value: operation.value}, true
	case 'g':
		if operation.getOK != state.present || operation.getOK && operation.getValue != state.value {
			return state, false
		}
		return state, true
	case 'd':
		if operation.deleteOK != state.present {
			return state, false
		}
		return historyState{}, true
	default:
		return state, false
	}
}
