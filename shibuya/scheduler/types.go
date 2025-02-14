package scheduler

import (
	"errors"
	"log"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/model"
	cloudrun "github.com/rakutentech/shibuya/shibuya/scheduler/cloudrun"
	k8s "github.com/rakutentech/shibuya/shibuya/scheduler/k8s"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
	apiv1 "k8s.io/api/core/v1"
)

type EngineScheduler interface {
	DeployPlan(projectID, collectionID, planID int64, replicas int, serviceIP string, containerConfig *config.ExecutorContainer) error
	CollectionStatus(projectID, collectionID int64, eps []*model.ExecutionPlan) (*smodel.CollectionStatus, error)
	CreateCollectionScraper(apiToken, token string, collectionID int64) error
	FetchEngineUrlsByPlan(collectionID, planID int64, opts *smodel.EngineOwnerRef) ([]string, error)
	PurgeCollection(collectionID int64) error
	GetDeployedCollections() (map[int64]time.Time, error)
	PodReadyCount(collectionID int64) int
	DownloadPodLog(collectionID, planID int64) (string, error)
	GetCollectionEnginesDetail(projectID, collectionID int64) (*smodel.CollectionDetails, error)
	GetDeployedServices() (map[int64]time.Time, error)
	ExposeProject(projectID int64) (*apiv1.Service, error)
	PurgeProjectIngress(projectID int64) error
	GetEnginesByProject(projectID int64) ([]apiv1.Pod, error)
	GetIngressUrl(projectID int64) (string, error)
	GetProjectAPIKey(projectID int64) (string, error)
}

var FeatureUnavailable = errors.New("Feature unavailable")

func NewEngineScheduler(cfg config.ShibuyaConfig) EngineScheduler {
	switch cfg.ExecutorConfig.Cluster.Kind {
	case "k8s":
		return k8s.NewK8sClientManager(cfg)
	case "cloudrun":
		return cloudrun.NewCloudRun(cfg)
	}
	log.Fatalf("Shibuya does not support %s as scheduler", cfg.ExecutorConfig.Cluster.Kind)
	return nil
}
