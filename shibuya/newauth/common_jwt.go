package newauth

import "github.com/golang-jwt/jwt/v5"

func ParseTokenWithKeyFunc(tokenString string, claim jwt.Claims, signingFunc func(token *jwt.Token) (interface{}, error)) (*jwt.Token, error) {
	if claim == nil {
		claim = jwt.MapClaims{}
	}
	if signingFunc == nil {
		token, _, err := new(jwt.Parser).ParseUnverified(tokenString, claim)
		return token, err
	}
	token, err := new(jwt.Parser).ParseWithClaims(tokenString, claim, signingFunc)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func SimpleParsing(tokenString string) (jwt.Claims, error) {
	signingFunc, err := makeSigningFunc(jwksURL)
	if err != nil {
		return nil, err
	}
	token, err := ParseTokenWithKeyFunc(tokenString, nil, signingFunc.Keyfunc)
	if err != nil {
		return nil, err
	}
	return token.Claims, nil
}
