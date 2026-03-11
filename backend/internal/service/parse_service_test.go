package service

import "testing"

func TestExtractSongID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "raw id",
			input:   "1869943939",
			want:    "1869943939",
			wantErr: false,
		},
		{
			name:    "query id",
			input:   "https://music.163.com/song?id=1869943939",
			want:    "1869943939",
			wantErr: false,
		},
		{
			name:    "fragment id",
			input:   "https://music.163.com/#/song?id=1869943939",
			want:    "1869943939",
			wantErr: false,
		},
		{
			name:    "mobile query id",
			input:   "https://y.music.163.com/m/song?id=1410647903&foo=bar",
			want:    "1410647903",
			wantErr: false,
		},
		{
			name:    "short link should not parse local path digit",
			input:   "https://163cn.tv/2SxiQSa",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSongID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("id mismatch: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestExtractPlaylistID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "raw id",
			input:   "123456789",
			want:    "123456789",
			wantErr: false,
		},
		{
			name:    "query id",
			input:   "https://music.163.com/playlist?id=123456789",
			want:    "123456789",
			wantErr: false,
		},
		{
			name:    "fragment id",
			input:   "https://music.163.com/#/playlist?id=123456789",
			want:    "123456789",
			wantErr: false,
		},
		{
			name:    "short link should not parse local path digit",
			input:   "https://163cn.tv/2SxiQSa",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPlaylistID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("id mismatch: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestNormalizeQuality(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback string
		want     string
	}{
		{name: "standard", input: "standard", fallback: "lossless", want: "standard"},
		{name: "sky", input: "sky", fallback: "standard", want: "sky"},
		{name: "jyeffect", input: "jyeffect", fallback: "standard", want: "jyeffect"},
		{name: "master alias", input: "master", fallback: "standard", want: "jymaster"},
		{name: "flac alias", input: "flac", fallback: "standard", want: "lossless"},
		{name: "invalid uses fallback", input: "unknown", fallback: "sky", want: "sky"},
		{name: "invalid fallback uses default", input: "unknown", fallback: "unknown", want: "standard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeQuality(tt.input, tt.fallback)
			if got != tt.want {
				t.Fatalf("quality mismatch: got=%q want=%q", got, tt.want)
			}
		})
	}
}
