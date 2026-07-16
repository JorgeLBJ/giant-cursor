package cursor

import "testing"

func TestEnlargedSize(t *testing.T) {
	cases := []struct {
		normal, scale, want int
	}{
		{32, 4, 128},
		{32, 8, 256},
		{32, 10, 256}, // clamped to MaxBaseSize
		{48, 6, 256},  // 288 -> clamped
		{0, 4, 128},   // normal defaults to 32
		{32, 0, 32},   // scale defaults to 1
	}
	for _, c := range cases {
		if got := EnlargedSize(c.normal, c.scale); got != c.want {
			t.Errorf("EnlargedSize(%d, %d) = %d, want %d", c.normal, c.scale, got, c.want)
		}
	}
}
