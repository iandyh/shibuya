package auth

import (
	"fmt"
	"net/http"
	"strings"
)

func AuthRequired(next http.Handler, requiredToken string) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		if bearer == "" {
			http.Error(w, "bearer header is empty", http.StatusForbidden)
			return
		}
		t := strings.Split(bearer, " ")
		if len(t) != 2 {
			http.Error(w, "bearer header is invalid", http.StatusBadRequest)
			return
		}
		token := t[1]
		if token != requiredToken {
			http.Error(w, fmt.Sprintf("incorrect token %s", token), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
