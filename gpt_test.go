package gpt

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustGpt(t *testing.T) *Gpt {
	t.Helper()

	token := os.Getenv("OPENAI_API_KEY")
	require.NotEmpty(t, token)
	gpt, err := NewGpt(token)
	require.NoError(t, err)

	return gpt
}
