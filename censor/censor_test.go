package censor

import "testing"

func TestIs(t *testing.T) {
	tests := []struct {
		text    string
		want    bool
		badword string
	}{
		{"хуй сосал долго", true, "хуй"},
		{"сосал долго", false, ""},
		{"хороший качество шалава", true, "шалава"},
	}
	for _, tt := range tests {
		w, res := Is(tt.text)
		if res != tt.want {
			t.Errorf("Is(%q) = %v, want - %v", tt.text, res, tt.want)
		}
		if res && w != tt.badword {
			t.Errorf("Is(%q) = %v, want - %v", tt.text, w, tt.badword)
		}
	}
}
