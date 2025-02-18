package config

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	apiv1 "k8s.io/api/core/v1"
)

const (
	ConfigFileName      = "config.json"
	AuthProviderGoogle  = "google"
	LDAPAuthBackend     = "ldap"
	DatabaseAuthBackend = "database"
)

var (
	ConfigFilePath = path.Join("/", ConfigFileName)
)

type LdapConfig struct {
	Enabled        bool   `json:"enabled"`
	BaseDN         string `json:"base_dn"`
	SystemUser     string `json:"system_user"`
	SystemPassword string `json:"system_password"`
	LdapServer     string `json:"ldap_server"`
	LdapPort       string `json:"ldap_port"`
}

type OauthClient struct {
	ClientID     string
	ClientSecret string
	Endpoint     string
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
}

type GoogleOauth struct {
	OauthClient
	Scopes []string
}

type InputBackendConf interface {
	*LdapConfig
}

type AuthConfig struct {
	AdminUsers        []string               `json:"admin_users"`
	EnableGoogleLogin bool                   `json:"enable_google_login"`
	OauthLogins       map[string]OauthClient `json:"oauth_logins"`
	OauthProvider     map[string]oauth2.Config
	LdapConfig        LdapConfig `json:"ldap_config"`
}

func GetInputAuthBackend[conf InputBackendConf](ac AuthConfig) (string, conf) {
	if ac.LdapConfig.Enabled {
		return LDAPAuthBackend, &ac.LdapConfig
	}
	return DatabaseAuthBackend, nil
}

type ClusterConfig struct {
	Project     string  `json:"project"`
	Zone        string  `json:"zone"`
	Region      string  `json:"region"`
	ClusterID   string  `json:"cluster_id"`
	Kind        string  `json:"kind"`
	APIEndpoint string  `json:"api_endpoint"`
	GCDuration  float64 `json:"gc_duration"` // in minutes
	ServiceType string  `json:"service_type"`
}

type HostAlias struct {
	Hostname string `json:"hostname"`
	IP       string `json:"IP"`
}

type Toleration struct {
	Key    string            `json:"key"`
	Value  string            `json:"value"`
	Effect apiv1.TaintEffect `json:"effect"`
}

type ExecutorConfig struct {
	InCluster              bool                          `json:"in_cluster"`
	Namespace              string                        `json:"namespace"`
	Cluster                *ClusterConfig                `json:"cluster"`
	ImagePullSecret        string                        `json:"pull_secret"`
	ImagePullPolicy        apiv1.PullPolicy              `json:"pull_policy"`
	EnginesContainer       map[string]*ExecutorContainer `json:"engines_container"`
	HostAliases            []*HostAlias                  `json:"host_aliases,omitempty"`
	NodeAffinity           []map[string]string           `json:"node_affinity"`
	Tolerations            []Toleration                  `json:"tolerations"`
	MaxEnginesInCollection int                           `json:"max_engines_in_collection"`
}

type ExecutorContainer struct {
	Image string `json:"image"`
	CPU   string `json:"cpu"`
	Mem   string `json:"mem"`
}

type ScraperContainer struct {
	// TODO: we should make the name more neutrual
	*ExecutorContainer
}

type DashboardConfig struct {
	Url              string `json:"url"`
	RunDashboard     string `json:"run_dashboard"`
	EnginesDashboard string `json:"engine_dashboard"`
}

type HttpConfig struct {
	Proxy string `json:"proxy"`
}

type ObjectStorage struct {
	Provider     string `json:"provider"`
	Url          string `json:"url"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Bucket       string `json:"bucket"`
	RequireProxy bool   `json:"require_proxy"`
	// This is the secret name created in the cluster for authenticating with object storage
	SecretName string `json:"secret_name"`
	// This is the mounted keys file name. e.g. /auth/shibuya-gcp.json
	AuthFileName string `json:"auth_file_name"`
	// This is the configuration file
	ConfigMapName string `json:"config_map_name"`
}

type LogFormat struct {
	Json     bool   `json:"json"`
	JsonPath string `json:"path"`
}

type IngressConfig struct {
	Image    string `json:"image"`
	Replicas int32  `json:"replicas"`
	CPU      string `json:"cpu"`
	Mem      string `json:"mem"`

	//Ingress controllers should be kept longer than then engines
	Lifespan   string `json:"lifespan"`
	GCInterval string `json:"gc_period"`
}

type MetricStorage struct {
	Gateway          string `json:"gateway"`
	RemoteWriteUrl   string `json:"url"`
	RemoteWriteToken string `json:"token"`
}

type CAPair struct {
	Cert       *x509.Certificate
	PrivateKey *rsa.PrivateKey
}

type ShibuyaConfig struct {
	ProjectHome      string           `json:"project_home"`
	UploadFileHelp   string           `json:"upload_file_help"`
	DistributedMode  bool             `json:"distributed_mode"`
	DBConf           *MySQLConfig     `json:"db"`
	ExecutorConfig   *ExecutorConfig  `json:"executors"`
	DashboardConfig  *DashboardConfig `json:"dashboard"`
	HttpConfig       *HttpConfig      `json:"http_config"`
	AuthConfig       *AuthConfig      `json:"auth_config"`
	ObjectStorage    *ObjectStorage   `json:"object_storage"`
	LogFormat        *LogFormat       `json:"log_format"`
	BackgroundColour string           `json:"bg_color"`
	IngressConfig    *IngressConfig   `json:"ingress"`
	MetricStorage    []MetricStorage  `json:"metric_storage"`
	ScraperContainer ScraperContainer `json:"scraper_container"`
	EnableSid        bool             `json:"enable_sid"`

	// below are configs generated from above values
	DevMode         bool
	Context         string
	HTTPClient      *http.Client
	HTTPProxyClient *http.Client

	CAPair *CAPair
}

func loadContext() string {
	return os.Getenv("env")
}

func LoadCaCert(certPath, keyPath string) *CAPair {
	tlscert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Fatalf("Fail to load ca cert %v", err)
	}
	return &CAPair{
		PrivateKey: tlscert.PrivateKey.(*rsa.PrivateKey),
		Cert:       tlscert.Leaf,
	}
}

func (sc *ShibuyaConfig) makeHTTPClients() {
	sc.HTTPClient = &http.Client{}
	sc.HTTPProxyClient = sc.HTTPClient
	if sc.HttpConfig.Proxy == "" {
		return
	}
	proxyUrl, err := url.Parse(sc.HttpConfig.Proxy)
	if err != nil {
		log.Fatal(err)
	}
	rt := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
	}
	sc.HTTPProxyClient = &http.Client{Transport: rt}
}

func applyJsonLogging(sc ShibuyaConfig) {
	log.SetFormatter(&log.JSONFormatter{})
	err := os.MkdirAll(sc.LogFormat.JsonPath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.OpenFile(path.Join(sc.LogFormat.JsonPath, "shibuya.json"),
		os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to log to file. %v", err)
	}
	log.SetOutput(file)
}

func SetupLogging(sc ShibuyaConfig) {
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)
	if sc.LogFormat != nil {
		if sc.LogFormat.Json {
			applyJsonLogging(sc)
		}
	}
}

func LoadConfig() ShibuyaConfig {
	sc := new(ShibuyaConfig)
	certPath := "/tls/tls.crt"
	keyPath := "/tls/tls.key"
	sc.CAPair = LoadCaCert(certPath, keyPath)
	f, err := os.Open(ConfigFilePath)
	if err != nil {
		log.Fatal("Cannot find config file")
	}
	raw, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("Cannot read json file %v", err)
	}
	if err := json.Unmarshal(raw, sc); err != nil {
		log.Fatalf("Cannot unmarshal json %v", err)
	}
	sc.Context = loadContext()
	sc.DevMode = sc.Context == "local"
	if sc.HttpConfig != nil {
		sc.makeHTTPClients()
	}
	// In jmeter agent, we also rely on this module, therefore we need to check whether this is nil or not. As jmeter
	// configuration might provide an empty struct here
	// TODO: we should not let jmeter code rely on this part
	if sc.ExecutorConfig != nil {
		if sc.ExecutorConfig.Cluster.GCDuration == 0 {
			sc.ExecutorConfig.Cluster.GCDuration = 15
		}
		if sc.ExecutorConfig.Cluster.Kind == "" {
			// if not specified, use k8s as default
			sc.ExecutorConfig.Cluster.Kind = "k8s"
		}
		if sc.ExecutorConfig.MaxEnginesInCollection == 0 {
			sc.ExecutorConfig.MaxEnginesInCollection = 500
		}
	}
	if sc.IngressConfig.Lifespan == "" {
		sc.IngressConfig.Lifespan = "30m"
	}
	if sc.IngressConfig.GCInterval == "" {
		sc.IngressConfig.GCInterval = "30s"
	}
	sc.AuthConfig.OauthProvider = make(map[string]oauth2.Config)
	for provider, oac := range sc.AuthConfig.OauthLogins {
		sc.AuthConfig.OauthProvider[provider] = oauth2.Config{
			ClientID:     os.Getenv("google_client_id"),
			ClientSecret: os.Getenv("google_client_secret"),
			Endpoint:     google.Endpoint,
			RedirectURL:  oac.RedirectURL,
			Scopes:       oac.Scopes,
		}
		log.Infof("Setting Google Oauth redirectional url with %s", oac.RedirectURL)
	}
	return *sc
}
