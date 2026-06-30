package service

import "testing"

func TestSearchQueryReady(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"", false},
		{"  ", false},
		{"a", false},
		{"ab", false},
		{"abc", true},
		{"  wiki  ", true},
		{"abç", true},
		{"áb", false},
	}
	for _, tt := range tests {
		if got := searchQueryReady(tt.query); got != tt.want {
			t.Errorf("searchQueryReady(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}
