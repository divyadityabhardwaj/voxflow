package update

import "testing"

func TestNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.2.3", "1.2.2", true},
		{"v1.2.3", "1.2.3", false},
		{"1.2.3", "v1.2.3", false},
		{"1.10.0", "1.9.9", true},
		{"1.9.9", "1.10.0", false},
		{"2.0", "1.99.99", true},
		{"1.2", "1.2.0", false},
		{"1.2.1", "1.2", true},
		{"1.3.0-beta.1", "1.2.9", true},
		{"1.2.3", "1.2.3-rc1", false},
		{"dev", "1.0.0", false},
		{"1.0.0", "dev", false},
		{"", "1.0.0", false},
		{"1.0.x", "0.9", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.a, tt.b); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
