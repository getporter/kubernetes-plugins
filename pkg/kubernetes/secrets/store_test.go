package secrets_test

import (
	"testing"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	"github.com/stretchr/testify/require"
)

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		desc     string
		input    string
		expected string
	}{
		{"with non-alphanumeric character at the start and end of the string", "-hello_", "000hello000"},
		{"with invalid symbols in the string", "-he_*llo.", "000he-llo000"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.expected, secrets.SanitizeKey(tt.input), "failed to sanitize input: %s", tt.input)
		})
	}
}
