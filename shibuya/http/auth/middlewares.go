package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	authtoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
)

const (
	isExcludedKey = "isExcluded"
)

func AuthRequiredWithToken(next http.Handler, requiredToken string) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		token, err := authtoken.FindBearerToken(r.Header)
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

func ExcludePathsFromLogger(next http.Handler, excludedPaths map[string]struct{}, excludedKeywords []string) func(next http.Handler) http.Handler {
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
