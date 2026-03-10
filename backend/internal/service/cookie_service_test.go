package service

import "testing"

func TestExtractMusicUFromRaw(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full cookie",
			input: "MUSIC_U=abc123; __csrf=xyz",
			want:  "abc123",
		},
		{
			name:  "value only",
			input: "abc123",
			want:  "abc123",
		},
		{
			name:  "not found",
			input: "__csrf=xyz",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMusicUFromRaw(tt.input)
			if got != tt.want {
				t.Fatalf("extractMusicUFromRaw(%q)=%q want=%q", tt.input, got, tt.want)
			}
		})
	}
}
