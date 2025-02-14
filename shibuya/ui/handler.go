package ui

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/rakutentech/shibuya/shibuya/api"
	"github.com/rakutentech/shibuya/shibuya/auth"
	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/http/auth/token"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
)

type UI struct {
	sc         config.ShibuyaConfig
	tmpl       *template.Template
	authMethod auth.InputRequiredAuth
}

var (
	staticFileServer = http.FileServer(http.Dir("/static"))
)

func serveStaticFile(w http.ResponseWriter, req *http.Request) {
	// Set the cache expiration time to 7 days
	w.Header().Set("Cache-Control", "public, max-age=604800")
	filepath := req.PathValue("filepath")
	req.URL.Path = filepath
	staticFileServer.ServeHTTP(w, req)
}

func NewUI(sc config.ShibuyaConfig) *UI {
	authMethod, conf := config.GetInputAuthBackend(*sc.AuthConfig)
	authFunc := auth.GetInputBackendAuthFunc(authMethod)
	u := &UI{
		sc:         sc,
		tmpl:       template.Must(template.ParseGlob("/templates/*.html")),
		authMethod: authFunc(conf),
	}
	return u
}

type HomeResp struct {
	Account               string
	BackgroundColour      string
	Context               string
	IsAdmin               bool
	ResultDashboard       string
	EnableSid             bool
	EngineHealthDashboard string
	ProjectHome           string
	UploadFileHelp        string
	GCDuration            float64
}

func (u *UI) homeHandler(w http.ResponseWriter, r *http.Request) {
	account := api.GetAccountBySession(r)
	if account == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	IsAdmin := account.IsAdmin(u.sc.AuthConfig)
	sc := u.sc
	enableSid := sc.EnableSid
	resultDashboardURL := sc.DashboardConfig.Url + sc.DashboardConfig.RunDashboard
	engineHealthDashboardURL := sc.DashboardConfig.Url + sc.DashboardConfig.EnginesDashboard
	if sc.DashboardConfig.EnginesDashboard == "" {
		engineHealthDashboardURL = ""
	}
	template := u.tmpl.Lookup("app.html")
	gcDuration := sc.ExecutorConfig.Cluster.GCDuration
	template.Execute(w, &HomeResp{account.Name, sc.BackgroundColour, sc.Context,
		IsAdmin, resultDashboardURL, enableSid,
		engineHealthDashboardURL, sc.ProjectHome, sc.UploadFileHelp, gcDuration})
}

func (u *UI) loginHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.Form.Get("username")
	password := r.Form.Get("password")
	authResult, err := u.authMethod.ValidateInput(username, password)
	if err != nil {
		loginUrl := fmt.Sprintf("/login?error_msg=%v", err)
		http.Redirect(w, r, loginUrl, http.StatusSeeOther)
		return
	}
	jwtToken, err := token.GenToken(username, authResult.ML, 0)
	cookie := token.MakeTokenCookie(jwtToken, !u.sc.DevMode)
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (u *UI) logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   token.CookieName,
		Path:   "/",
		MaxAge: -1,
	})
}

type LoginResp struct {
	ErrorMsg string
}

func (u *UI) loginPageHandler(w http.ResponseWriter, r *http.Request) {
	template := u.tmpl.Lookup("login.html")
	qs := r.URL.Query()
	errMsgs := qs["error_msg"]
	e := new(LoginResp)
	e.ErrorMsg = ""
	if len(errMsgs) > 0 {
		e.ErrorMsg = errMsgs[0]
	}
	template.Execute(w, e)
}

func (u *UI) Router() *httproute.Router {
	router := &httproute.Router{
		Name: "ui",
		Path: "",
	}
	routes := httproute.Routes{
		{
			Name:        "home",
			Method:      "GET",
			Path:        "/{$}",
			HandlerFunc: u.homeHandler,
		},
		{
			Name:        "login",
			Method:      "POST",
			Path:        "/login",
			HandlerFunc: u.loginHandler,
		},
		{
			Name:        "login",
			Method:      "GET",
			Path:        "/login",
			HandlerFunc: u.loginPageHandler,
		},
		{
			Name:        "providers login",
			Method:      "GET",
			Path:        "/login/{provider}",
			HandlerFunc: u.loginByProvider,
		},
		{
			Name:        "providers login call back",
			Method:      "GET",
			Path:        "/login/callback/{provider}",
			HandlerFunc: u.callbackHandler,
		},
		{
			Name:        "logout",
			Method:      "POST",
			Path:        "/logout",
			HandlerFunc: u.logoutHandler,
		},
		{
			Name:        "Get static files",
			Method:      "GET",
			Path:        "/static/{filepath...}",
			HandlerFunc: serveStaticFile,
		},
	}
	router.AddRoutes(routes)
	return router
}
