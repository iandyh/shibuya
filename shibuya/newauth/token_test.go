package newauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	tokenString = ""
)

func TestParseToken(t *testing.T) {
	c, err := ROCIAMAuth(tokenString)
	assert.Nil(t, err)

	sub := c.GetUser()
	assert.Nil(t, err)
	assert.Equal(t, "6bb9db99-d687-4c5f-8f37-ad3ea560aad4", sub)
	assert.Equal(t, true, len(c.GetOrgs()) > 1)
	r := NewRole("caas", c)
	assert.True(t, r.IsSubscribed("shibuya"))
	assert.True(t, r.IsSubscribed("lbaas"))
	assert.True(t, r.IsAdmin("caas"))
}

func TestSimpleParsing(t *testing.T) {
	c, err := SimpleParsing(tokenString)
	assert.Nil(t, err)

	sub, err := c.GetSubject()
	assert.Equal(t, "6bb9db99-d687-4c5f-8f37-ad3ea560aad4", sub)
}
