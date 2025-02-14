package auth

import "github.com/rakutentech/shibuya/shibuya/config"

type AuthWithDB struct{}

func NewAuthWithDB[C config.InputBackendConf](c C) InputRequiredAuth {
	return AuthWithDB{}
}

func (a AuthWithDB) ValidateInput(username, password string) (AuthResult, error) {
	return AuthResult{
		Username: username,
		ML:       []string{username},
	}, nil
}
