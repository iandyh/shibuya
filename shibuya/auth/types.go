package auth

import "github.com/rakutentech/shibuya/shibuya/config"

type (
	AuthResult struct {
		Username string
		ML       []string
	}
	AuthFunc[ac config.InputBackendConf] func(ac) InputRequiredAuth
	InputRequiredAuth                    interface {
		ValidateInput(string, string) (AuthResult, error)
	}
)

func GetInputBackendAuthFunc[ac config.InputBackendConf](method string) AuthFunc[ac] {
	switch method {
	case config.LDAPAuthBackend:
		return AuthFunc[ac](NewLdapAuth[ac])
	case config.DatabaseAuthBackend:
		return AuthFunc[ac](NewAuthWithDB[ac])
	}
	return nil
}
