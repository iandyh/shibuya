package keys_test

import (
	"testing"

	"github.com/rakutentech/shibuya/shibuya/auth/keys"
	"github.com/stretchr/testify/assert"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := keys.GenerateAPIKey()
	assert.Nil(t, err)
	assert.NotEqual(t, "", key)
	assert.Equal(t, 64, len(key))
}
