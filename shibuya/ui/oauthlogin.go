package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/rakutentech/shibuya/shibuya/auth"
	"github.com/rakutentech/shibuya/shibuya/auth/keys"
	"github.com/rakutentech/shibuya/shibuya/config"
	"golang.org/x/oauth2"
)

var (
	IncorrectAuthProviderError = errors.New("Incorrect auth provider")
	IncorrectOauthState        = errors.New("Incorrect oauth state")
	TokenExchangeFailed        = errors.New("Failed to exchange token")
	oauthStateCookie           = "oauth_state"
	ProviderUserAPI            = map[string]string{
		config.AuthProviderGoogle: "https://www.googleapis.com/oauth2/v2/userinfo",
	}
	ProviderUserDecoder = map[string]func(*http.Response) (string, error){
		config.AuthProviderGoogle: fetchGoogleUserInfo,
	}
	oauthHTTPClient = &http.Client{
		Timeout: 3 * time.Second,
	}
)

func fetchGoogleUserInfo(resp *http.Response) (string, error) {
	var user map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}
	email := user["email"].(string)
	return email, nil
}

func makeOAuthStateCookie(oauthState string) *http.Cookie {
	return &http.Cookie{
		Name:     oauthStateCookie,
		Value:    oauthState,
		Expires:  time.Now().Add(5 * time.Minute),
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	}
}

func (u *UI) callbackHandler(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	config, ok := u.sc.AuthConfig.OauthProvider[provider]
	if !ok {
		http.Error(w, IncorrectAuthProviderError.Error(), http.StatusBadRequest)
		return
	}
	session, err := auth.SessionStore.Get(r, u.sc.AuthConfig.SessionKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := handleOauthCallback(r, config, provider, session); err != nil {
		if errors.Is(IncorrectOauthState, err) || errors.Is(TokenExchangeFailed, err) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (u *UI) loginByProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	authConfig, ok := u.sc.AuthConfig.OauthProvider[provider]
	if !ok {
		http.Error(w, IncorrectAuthProviderError.Error(), http.StatusBadRequest)
		return
	}
	oauthState, err := keys.GenerateAPIKey()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	url := authConfig.AuthCodeURL(oauthState)
	http.SetCookie(w, makeOAuthStateCookie(oauthState))
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleOauthCallback(r *http.Request, oauthConfig oauth2.Config, provider string, session *sessions.Session) error {
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil {
		return err
	}
	if r.URL.Query().Get("state") != cookie.Value {
		return IncorrectOauthState
	}
	code := r.URL.Query().Get("code")
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return TokenExchangeFailed
	}
	client := oauthConfig.Client(context.WithValue(context.Background(),
		oauth2.HTTPClient, oauthHTTPClient), token)
	userAPI := ProviderUserAPI[provider]
	resp, err := client.Get(userAPI)
	if err != nil {
		return errors.New("Failed to get user info")
	}
	defer resp.Body.Close()
	username, err := ProviderUserDecoder[provider](resp)
	if err != nil {
		return err
	}
	ml := []string{username}
	session.Values[auth.AccountKey] = username
	session.Values[auth.MLKey] = ml
	return nil
}
