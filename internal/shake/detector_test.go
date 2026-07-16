package shake

import "testing"

func TestNewDetectorStartsNormal(t *testing.T) {
	d := New(Config{})
	if d.State() != StateNormal {
		t.Fatalf("want NORMAL, got %v", d.State())
	}
}
