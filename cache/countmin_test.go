package cache

import "testing"

func TestCountMinSketchCountThresholdAndDecay(t *testing.T) {
	sketch, err := NewCountMinSketch(4, 1_024)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 8; index++ {
		sketch.Add("hot")
	}
	for index := 0; index < 2; index++ {
		sketch.Add("warm")
	}

	if count := sketch.Count("hot"); count < 8 {
		t.Fatalf("hot count = %d, want at least 8", count)
	}
	if threshold := sketch.TopThreshold([]string{"hot", "warm", "cold"}, 1.0/3.0, 2); threshold < 8 {
		t.Fatalf("threshold = %d, want at least 8", threshold)
	}

	sketch.Decay()
	if count := sketch.Count("hot"); count < 4 || count >= 8 {
		t.Fatalf("decayed hot count = %d, want [4, 8)", count)
	}
}

func TestCountMinSketchRejectsInvalidDimensions(t *testing.T) {
	for _, dimensions := range [][2]int{{0, 1}, {1, 0}, {-1, 1}, {100_000, 100_000}} {
		if _, err := NewCountMinSketch(dimensions[0], dimensions[1]); err == nil {
			t.Fatalf("dimensions %v should fail", dimensions)
		}
	}
}
