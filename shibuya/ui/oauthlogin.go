package ui

// in order to debug the logic here locally, make sure you do the following things:
// 1. run the script to generate the secret for client_id and client_secret. It's under the shibuya/shibuya folder
// 2. disable the no_auth flag. (enable the auth) and enabled the google login in the values.yaml
// 3. Set the session_key to something like: test
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rakutentech/shibuya/shibuya/auth/keys"
	"github.com/rakutentech/shibuya/shibuya/config"
	authtoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
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
	cookie, err := u.handleOauthCallback(r, config, provider)
	if err != nil {
		if errors.Is(IncorrectOauthState, err) || errors.Is(TokenExchangeFailed, err) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, cookie)
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

func (u *UI) handleOauthCallback(r *http.Request, oauthConfig oauth2.Config, provider string) (*http.Cookie, error) {
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil {
		return nil, err
	}
	if r.URL.Query().Get("state") != cookie.Value {
		return nil, IncorrectOauthState
	}
	code := r.URL.Query().Get("code")
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, TokenExchangeFailed
	}
	client := oauthConfig.Client(context.WithValue(context.Background(),
		oauth2.HTTPClient, oauthHTTPClient), token)
	userAPI := ProviderUserAPI[provider]
	resp, err := client.Get(userAPI)
	if err != nil {
		return nil, errors.New("Failed to get user info")
	}
	defer resp.Body.Close()
	username, err := ProviderUserDecoder[provider](resp)
	if err != nil {
		return nil, err
	}
	jwtToken, err := authtoken.GenToken(username, []string{username}, 0)
	if err != nil {
		return nil, err
	}
	secure := !u.sc.DevMode
	return authtoken.MakeTokenCookie(jwtToken, secure), nil
}
