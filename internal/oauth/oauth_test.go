package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateState(t *testing.T) {
	state1, err := GenerateState()
	assert.NoError(t, err)
	assert.NotEmpty(t, state1)

	state2, err := GenerateState()
	assert.NoError(t, err)
	assert.NotEmpty(t, state2)

	// Each call should produce a different state
	assert.NotEqual(t, state1, state2)

	// State should be base64 URL encoded (44 chars for 32 bytes)
	assert.Len(t, state1, 44)
}
