package token_test

import (
	"testing"
	"time"

	authtoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
	"github.com/stretchr/testify/assert"
)

func TestGenToken(t *testing.T) {
	username := "asdf"
	groups := []string{username}
	token1, err := authtoken.GenToken(username, groups, 5*time.Second)
	assert.Nil(t, err)
	token2, err := authtoken.GenToken(username, groups, 5*time.Second)
	assert.NotEqual(t, token1, token2)
}

func TestTokenCookie(t *testing.T) {
	username := "asdf"
	groups := []string{username}
	token, _ := authtoken.GenToken(username, groups, 1*time.Hour)
	cookie := authtoken.MakeTokenCookie(token, false)
	vt, err := authtoken.VerifyJWT(cookie.Value, "", "")
	assert.Nil(t, err)
	tc, err := authtoken.FindTokenClaim(vt)
	assert.Nil(t, err)
	assert.Equal(t, username, tc.Username)

	expToken, _ := authtoken.GenToken(username, groups, 1*time.Second)
	cookie = authtoken.MakeTokenCookie(expToken, false)
	time.Sleep(1 * time.Second)
	_, err = authtoken.VerifyJWT(cookie.Value, "", "")
	assert.Error(t, err)
}
