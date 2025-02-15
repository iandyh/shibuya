package token

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rakutentech/shibuya/shibuya/auth/keys"
)

const (
	AuthHeader    = "Authorization"
	BEARER_PREFIX = "Bearer"
	CookieName    = "shibuya"
)

var (
	jwtSecret      = []byte(os.Getenv("jwt-secret"))
	CookieLifeSpan = 24 * time.Hour * 365
	TokenExpired   = errors.New("Token expired")
	tokenVerifier  = map[string]func(string) jwt.Keyfunc{
		"shibuya": func(s string) jwt.Keyfunc {
			return func(token *jwt.Token) (interface{}, error) {
				return jwtSecret, nil
			}
		},
		"example": func(jwksURL string) jwt.Keyfunc {
			//jwks, _ := keyfunc.Get(jwksURL, keyfunc.Options{})
			//return jwks.Keyfunc
			return nil
		},
	}
	EmptyTokenError = errors.New("Bearer header is empty")
	InvalidToken    = errors.New("Token is invalid")
)

type TokenClaim struct {
	Username string
	Groups   []string
}

func GenToken(username string, groups []string, exp time.Duration) (string, error) {
	if exp == 0 {
		exp = CookieLifeSpan
	}
	salt, err := keys.GenerateSalt()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub":    username,
		"groups": groups,
		"exp":    time.Now().Add(exp).Unix(), // Expires in 24 hours
		"salt":   hex.EncodeToString(salt),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func MakeTokenCookie(token string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Expires:  time.Now().Add(CookieLifeSpan),
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	}
}

func convertClaimSlice(orig interface{}) []string {
	s, ok := orig.([]interface{})
	if !ok {
		return nil
	}
	r := make([]string, len(s))
	for i, item := range s {
		r[i] = item.(string)
	}
	return r
}

func FindTokenClaim(token *jwt.Token) (TokenClaim, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return TokenClaim{}, fmt.Errorf("invalid claims")
	}
	username, err := claims.GetSubject()
	if err != nil {
		return TokenClaim{}, err
	}
	return TokenClaim{
		Username: username,
		Groups:   convertClaimSlice(claims["groups"]),
	}, nil
}

func VerifyJWT(value, issuer, jwksURL string) (*jwt.Token, error) {
	if issuer == "" {
		issuer = "shibuya"
	}
	keyFunc, ok := tokenVerifier[issuer]
	if !ok {
		return nil, errors.New("Unsupported issuer")
	}
	token, err := jwt.Parse(value, keyFunc(jwksURL))
	if err != nil || !token.Valid {
		return nil, err
	}
	return token, nil
}

func ParseToken(bearer string) (string, error) {
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

func FindBearerToken(header http.Header) (string, error) {
	bearer := header.Get(AuthHeader)
	token, err := ParseToken(bearer)
	if err != nil {
		return "", err
	}
	return token, nil
}
