package app

import (
	"strings"
	"testing"
)

func TestSanitizeArmored(t *testing.T) {
	const clean = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\naGVsbG8=\n=AAAA\n-----END PGP PUBLIC KEY BLOCK-----\n"

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already clean",
			input: clean,
			want:  clean,
		},
		{
			name:  "leading and trailing blank lines",
			input: "\n\n\n" + clean + "\n\n",
			want:  clean,
		},
		{
			name:  "leading whitespace on key lines",
			input: "  -----BEGIN PGP PUBLIC KEY BLOCK-----\n\n  aGVsbG8=\n  =AAAA\n  -----END PGP PUBLIC KEY BLOCK-----\n",
			want:  clean,
		},
		{
			name:  "trailing whitespace on key lines",
			input: "-----BEGIN PGP PUBLIC KEY BLOCK-----   \n\naGVsbG8=   \n=AAAA   \n-----END PGP PUBLIC KEY BLOCK-----   \n",
			want:  clean,
		},
		{
			name:  "CRLF line endings",
			input: "-----BEGIN PGP PUBLIC KEY BLOCK-----\r\n\r\naGVsbG8=\r\n=AAAA\r\n-----END PGP PUBLIC KEY BLOCK-----\r\n",
			want:  clean,
		},
		{
			name:  "BOM prefix",
			input: "\ufeff" + clean,
			want:  clean,
		},
		{
			name:  "garbage text before and after block",
			input: "Here is my key:\n\n" + clean + "\nThanks!",
			want:  clean,
		},
		{
			name:  "non-breaking spaces on lines",
			input: "-----BEGIN PGP PUBLIC KEY BLOCK-----\u00a0\n\naGVsbG8=\u00a0\n=AAAA\u00a0\n-----END PGP PUBLIC KEY BLOCK-----\u00a0\n",
			want:  clean,
		},
		{
			name:  "no PGP markers — return trimmed input",
			input: "   not a pgp key   ",
			want:  "not a pgp key",
		},
		{
			name:  "private key block header",
			input: "-----BEGIN PGP PRIVATE KEY BLOCK-----\n\naGVsbG8=\n=AAAA\n-----END PGP PRIVATE KEY BLOCK-----\n",
			want:  "-----BEGIN PGP PRIVATE KEY BLOCK-----\n\naGVsbG8=\n=AAAA\n-----END PGP PRIVATE KEY BLOCK-----\n",
		},
		{
			name:  "zero-width space prefix",
			input: "\u200b" + clean,
			want:  clean,
		},
		{
			name:  "tabs on lines",
			input: "-----BEGIN PGP PUBLIC KEY BLOCK-----\t\n\naGVsbG8=\t\n=AAAA\t\n-----END PGP PUBLIC KEY BLOCK-----\t\n",
			want:  clean,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeArmored(tt.input)
			if got != tt.want {
				t.Errorf("\ngot:  %q\nwant: %q", got, tt.want)
				// show line-by-line diff for readability
				gotLines := strings.Split(got, "\n")
				wantLines := strings.Split(tt.want, "\n")
				max := len(gotLines)
				if len(wantLines) > max {
					max = len(wantLines)
				}
				for i := 0; i < max; i++ {
					g, w := "", ""
					if i < len(gotLines) {
						g = gotLines[i]
					}
					if i < len(wantLines) {
						w = wantLines[i]
					}
					if g != w {
						t.Logf("line %d: got %q, want %q", i, g, w)
					}
				}
			}
		})
	}
}
