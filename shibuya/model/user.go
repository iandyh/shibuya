package model

import (
	"net/http"

	"github.com/rakutentech/shibuya/shibuya/config"
	authtoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
)

type Account struct {
	ML    []string
	MLMap map[string]interface{}
	Name  string
}

func GetAccountBySession(r *http.Request, authConfig *config.AuthConfig) *Account {
	a := new(Account)
	a.MLMap = make(map[string]interface{})
	if authConfig.NoAuth {
		a.Name = "shibuya"
		a.ML = []string{a.Name}
		a.MLMap[a.Name] = struct{}{}
		return a
	}
	cookie, err := r.Cookie(authtoken.CookieName)
	if err != nil {
		return nil
	}
	token, err := authtoken.VerifyJWT(cookie.Value, "", "")
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
	if authConfig.NoAuth {
		return true
	}
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
