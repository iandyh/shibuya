package model

import (
	"github.com/rakutentech/shibuya/shibuya/config"
)

type Account struct {
	ML    []string
	MLMap map[string]interface{}
	Name  string
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
