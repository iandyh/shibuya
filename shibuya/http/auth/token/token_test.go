package token_test

import (
	"fmt"
	"net/http"
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

func TestFindToken(t *testing.T) {
	validBearer := fmt.Sprintf("%s asdf", authtoken.BEARER_PREFIX)
	header := http.Header{}
	header.Add(authtoken.AuthHeader, validBearer)
	token, err := authtoken.FindBearerToken(header)
	assert.Nil(t, err)
	assert.Equal(t, "asdf", token)

	header.Del(authtoken.AuthHeader)
	inValidBearer := "b asdf"
	header.Add(authtoken.AuthHeader, inValidBearer)
	token, err = authtoken.FindBearerToken(header)
	assert.ErrorIs(t, err, authtoken.InvalidToken)

	emptyBearer := ""
	header.Del(authtoken.AuthHeader)
	header.Add(authtoken.AuthHeader, emptyBearer)
	token, err = authtoken.FindBearerToken(header)
	assert.ErrorIs(t, err, authtoken.EmptyTokenError)
}
