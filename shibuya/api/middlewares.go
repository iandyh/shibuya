package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rakutentech/shibuya/shibuya/model"
)

const (
	accountKey    = "account"
	isExcludedKey = "isExcluded"
)

var (
	excludedPaths = map[string]struct{}{
		"/metrics": {},
		"/health":  {},
	}
	excludedKeywords = []string{
		"stream",
	}
)

func authWithSession(r *http.Request) (*model.Account, error) {
	account := model.GetAccountBySession(r)
	if account == nil {
		return nil, makeLoginError()
	}
	return account, nil
}

func authRequired(next http.HandlerFunc) http.HandlerFunc {
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

func RequestLoggerWithoutPaths(next http.Handler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			isExcluded := r.Context().Value(isExcludedKey).(bool)
			if !isExcluded {
				middleware.Logger(next).ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// This should be the last middleware to be wrapper
func ExcludePathsFromLogger(next http.Handler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			isExcluded := false
			if _, ok := excludedPaths[r.URL.Path]; ok {
				isExcluded = true
			}
			// This should be used with care for two reasons
			// 1. Since every request goes through the check, string.Contains is slow
			// 2. Using keyword based approach is not an exact match, so it could kill other requests
			// that contain the same keyword
			for _, k := range excludedKeywords {
				if strings.Contains(r.URL.Path, k) {
					isExcluded = true
					break
				}
			}
			contextR := r.WithContext(context.WithValue(r.Context(), isExcludedKey, isExcluded))
			next.ServeHTTP(w, contextR)
		}
		return http.HandlerFunc(fn)
	}
}
