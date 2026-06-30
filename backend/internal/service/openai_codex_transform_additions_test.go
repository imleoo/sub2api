package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// defaultCodexSynthInstructions：按模型选用真实 Codex base prompt。
func TestDefaultCodexSynthInstructionsModelAware(t *testing.T) {
	require.True(t, strings.Contains(defaultCodexSynthInstructions("gpt-5-codex"), "You are Codex, based on GPT-5"))
	require.True(t, strings.Contains(defaultCodexSynthInstructions("gpt-5.5"), "You are Codex, a coding agent based on GPT-5"))
	require.False(t, strings.Contains(defaultCodexSynthInstructions("gpt-5.5"), "You are GPT-5.1 running in the Codex CLI"))
	require.True(t, strings.Contains(defaultCodexSynthInstructions("gpt-5.2"), "You are GPT-5.2 running in the Codex CLI"))
	require.True(t, strings.Contains(defaultCodexSynthInstructions("gpt-5.1"), "You are GPT-5.1 running in the Codex CLI"))
}
