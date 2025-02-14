package model

import (
	"errors"
	"net/http"

	"github.com/rakutentech/shibuya/shibuya/config"
	httpauth "github.com/rakutentech/shibuya/shibuya/http/auth"
	authtoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
)

type Account struct {
	ML    []string
	MLMap map[string]interface{}
	Name  string
}

func FindTokenFromHeaders(r *http.Request) (string, error) {
	cookie, err := r.Cookie(authtoken.CookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			bearer := r.Header.Get(httpauth.AuthHeader)
			return httpauth.FindToken(bearer)
		}
		return "", err
	}
	return cookie.Value, nil
}

func GetAccountBySession(r *http.Request) *Account {
	a := new(Account)
	a.MLMap = make(map[string]interface{})
	tokenString, err := FindTokenFromHeaders(r)
	if err != nil {
		return nil
	}
	token, err := authtoken.VerifyJWT(tokenString, "", "")
	if err != nil {
		return nil
	}
	tokenClaim, err := authtoken.FindTokenClaim(token)
	if err != nil {
		return nil
	}
	a.Name = tokenClaim.Username
	a.ML = tokenClaim.Groups
	for _, m := range a.ML {
		a.MLMap[m] = struct{}{}
	}
	return a
}

func (a *Account) IsAdmin(authConfig *config.AuthConfig) bool {
	for _, ml := range a.ML {
		for _, admin := range authConfig.AdminUsers {
			if ml == admin {
				return true
			}
		}
	}
	// systemuser is the user used for LDAP auth. If a user login with that account
	// we can also treat it as a admin
	if a.Name == authConfig.LdapConfig.SystemUser {
		return true
	}
	return false
}
