package metrics

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	promCommonConfig "github.com/prometheus/common/config"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	k8sDiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/rakutentech/shibuya/shibuya/config"
	authtoken "github.com/rakutentech/shibuya/shibuya/http/auth/token"
)

type GlobalConfig struct {
	ScrapeInterval time.Duration `yaml:"scrape_interval"`
}

type RelabelConfig struct {
	SourceLabels []string `yaml:"source_labels"`
	Regex        string   `yaml:"regex"`
	Action       string   `yaml:"action"`
}

type ScrapeConfig struct {
	JobName                 string             `yaml:"job_name"`
	RelabelConfigs          []*relabel.Config  `yaml:"relabel_configs"`
	ServiceDiscoveryConfigs []discovery.Config `yaml:"kubernetes_sd_configs"`
	HTTPConfig              HTTPConfig         `yaml:",inline"`
}

type Authorization struct {
	Type        string `yaml:"type"`
	Credentials string `yaml:"credentials"`
}

type RemoteWriteConfig struct {
	URL           string            `yaml:"url"`
	RemoteTimeout time.Duration     `yaml:"remote_timeout"`
	Authorization *Authorization    `yaml:"authorization"`
	Headers       map[string]string `yaml:"headers"`
}

type HTTPConfig struct {
	Authorization *Authorization `yaml:"authorization"`
}

type PromConfig struct {
	GlobalConfig       *GlobalConfig        `yaml:"global"`
	ScrapeConfigs      []*ScrapeConfig      `yaml:"scrape_configs"`
	RemoteWriteConfigs []*RemoteWriteConfig `yaml:"remote_write"`
}

func makePromRelabelConfig(sourceLabels []string, regex, action string) (*relabel.Config, error) {
	sls := make(promModel.LabelNames, len(sourceLabels))
	for i, sl := range sourceLabels {
		sls[i] = promModel.LabelName(sl)
	}
	rx, err := relabel.NewRegexp(regex)
	if err != nil {
		return nil, err
	}
	return &relabel.Config{
		SourceLabels: sls,
		Regex:        rx,
		Action:       relabel.Action(action),
	}, nil
}

func engineRelabelConfigs(collectionID int64) []RelabelConfig {
	return []RelabelConfig{
		{
			SourceLabels: []string{"__meta_kubernetes_pod_label_collection"},
			Regex:        fmt.Sprintf("%d", collectionID),
			Action:       "keep",
		},
		{
			SourceLabels: []string{"__meta_kubernetes_pod_label_kind"},
			Regex:        "executor",
			Action:       "keep",
		},
	}
}

func makeRemoteWriteConfig(item config.MetricStorage, headers map[string]string) (*RemoteWriteConfig, error) {
	if _, err := url.Parse(item.Gateway); err != nil {
		return nil, err
	}
	remoteWriteConfig := &RemoteWriteConfig{
		URL:           item.Gateway,
		RemoteTimeout: time.Duration(60 * time.Second),
		Headers:       headers,
	}
	az := &Authorization{
		Type:        "Bearer",
		Credentials: item.RemoteWriteToken,
	}
	remoteWriteConfig.Authorization = az
	return remoteWriteConfig, nil
}

func MakeScraperConfig(apiToken, token string, collectionID int64, namespace string, ms []config.MetricStorage) (*PromConfig, error) {
	remoteWriteConfigs := make([]*RemoteWriteConfig, len(ms))
	headers := map[string]string{
		"collection_id": strconv.Itoa(int(collectionID)),
	}
	for i, item := range ms {
		if item.Gateway != "" {
			item.RemoteWriteToken = apiToken
		}
		t, err := makeRemoteWriteConfig(item, headers)
		if err != nil {
			return nil, err
		}
		remoteWriteConfigs[i] = t
	}

	pc := &PromConfig{}
	pc.RemoteWriteConfigs = remoteWriteConfigs
	pc.GlobalConfig = &GlobalConfig{}
	pc.GlobalConfig.ScrapeInterval = time.Duration(time.Second)
	sd := &k8sDiscovery.SDConfig{
		Role: k8sDiscovery.Role("pod"),
		NamespaceDiscovery: k8sDiscovery.NamespaceDiscovery{
			Names: []string{namespace},
		},
		HTTPClientConfig: promCommonConfig.DefaultHTTPClientConfig,
	}
	erc := engineRelabelConfigs(collectionID)
	rcs := make([]*relabel.Config, len(erc))
	for i, rc := range erc {
		prc, err := makePromRelabelConfig(rc.SourceLabels, rc.Regex, rc.Action)
		if err != nil {
			return nil, err
		}
		rcs[i] = prc
	}
	pc.ScrapeConfigs = []*ScrapeConfig{
		{
			JobName:                 "shibuya-metrics",
			ServiceDiscoveryConfigs: []discovery.Config{sd},
			RelabelConfigs:          rcs,
			HTTPConfig: HTTPConfig{
				Authorization: &Authorization{
					Type:        authtoken.BEARER_PREFIX,
					Credentials: token,
				},
			},
		},
	}
	return pc, nil
}
