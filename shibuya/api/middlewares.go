package api

import (
	"context"
	"errors"
	"net/http"

	httpauth "github.com/rakutentech/shibuya/shibuya/http/auth"
	authtoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
	"github.com/rakutentech/shibuya/shibuya/model"
)

const (
	accountKey = "account"
)

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

func GetAccountBySession(r *http.Request) *model.Account {
	a := new(model.Account)
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

func authWithSession(r *http.Request) (*model.Account, error) {
	account := GetAccountBySession(r)
	if account == nil {
		return nil, makeLoginError()
	}
	return account, nil
}

func sessionRequired(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var account *model.Account
		var err error
		account, err = authWithSession(r)
		if err != nil {
			handleErrors(w, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), accountKey, account)))
	})
}

// This should be the last middleware to be wrapper
