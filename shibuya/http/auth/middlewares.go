package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	AuthHeader    = "Authorization"
	BEARER_PREFIX = "Bearer"
)

var (
	EmptyTokenError = errors.New("Bearer header is empty")
	InvalidToken    = errors.New("Token is invalid")
)

func FindToken(bearer string) (string, error) {
	if bearer == "" {
		return "", EmptyTokenError
	}
	t := strings.Split(bearer, " ")
	if len(t) != 2 {
		return "", InvalidToken
	}
	if t[0] != BEARER_PREFIX {
		return "", InvalidToken
	}
	return t[1], nil
}

func AuthRequiredWithToken(next http.Handler, requiredToken string) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get(AuthHeader)
		token, err := FindToken(bearer)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if token != requiredToken {
			http.Error(w, fmt.Sprintf("incorrect token %s", token), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
