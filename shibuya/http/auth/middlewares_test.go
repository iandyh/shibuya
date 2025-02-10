package auth_test

import (
	"fmt"
	"testing"

	"github.com/rakutentech/shibuya/shibuya/http/auth"
	"github.com/stretchr/testify/assert"
)

func TestFindToken(t *testing.T) {
	validBearer := fmt.Sprintf("%s asdf", auth.BEARER_PREFIX)
	token, err := auth.FindToken(validBearer)
	assert.Nil(t, err)
	assert.Equal(t, "asdf", token)

	inValidBearer := "b asdf"
	token, err = auth.FindToken(inValidBearer)
	assert.ErrorIs(t, err, auth.InvalidToken)

	emptyBearer := ""
	token, err = auth.FindToken(emptyBearer)
	assert.ErrorIs(t, err, auth.EmptyTokenError)
}
