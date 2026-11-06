package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeLike(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "teks biasa", input: "Budi Santoso", want: "Budi Santoso"},
		{name: "persen", input: "100%", want: `100\%`},
		{name: "underscore", input: "a_b", want: `a\_b`},
		{name: "backslash", input: `a\b`, want: `a\\b`},
		{name: "kombinasi", input: `%_\`, want: `\%\_\\`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, EscapeLike(tc.input))
		})
	}
}
