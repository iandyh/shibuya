package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/model"
)

const (
	accountKey = "account"
)

func authWithSession(r *http.Request, authConfig *config.AuthConfig) (*model.Account, error) {
	account := model.GetAccountBySession(r, authConfig)
	if account == nil {
		return nil, makeLoginError()
	}
	return account, nil
}

// TODO add JWT token auth in the future
func authWithToken(_ *http.Request) (*model.Account, error) {
	return nil, errors.New("No token presented")
}

func authRequired(next http.HandlerFunc, authConfig *config.AuthConfig) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var account *model.Account
		var err error
		account, err = authWithSession(r, authConfig)
		if err != nil {
			handleErrors(w, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), accountKey, account)))
	})
}
