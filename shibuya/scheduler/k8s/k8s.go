package k8s

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	e "errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/rakutentech/shibuya/shibuya/auth/keys"
	"github.com/rakutentech/shibuya/shibuya/config"
	model "github.com/rakutentech/shibuya/shibuya/model"
	serrors "github.com/rakutentech/shibuya/shibuya/scheduler/errors"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
	log "github.com/sirupsen/logrus"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type K8sClientManager struct {
	sc                    config.ShibuyaConfig
	client                *kubernetes.Clientset
	cdrServiceAccount     string
	scraperServiceAccount string
	Namespace             string
	httpClient            *http.Client
	CAPair                *config.CAPair
}

func NewK8sClientManager(cfg config.ShibuyaConfig) *K8sClientManager {
	c, err := config.GetKubeClient(cfg.ExecutorConfig)
	if err != nil {
		log.Warning(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cfg.CAPair.Cert)
	httpClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}
	return &K8sClientManager{
		cfg, c, "shibuya-coordinator", "shibuya-scraper", cfg.ExecutorConfig.Namespace, httpClient, cfg.CAPair,
	}
}

func (kcm *K8sClientManager) getRandomHostIP() (string, error) {
	podList, err := kcm.client.CoreV1().Pods(kcm.Namespace).
		List(context.TODO(), metav1.ListOptions{
			Limit: 1,
			// we need to add the selector here because pod's hostIP could be empty if it's in pending state
			// So we want to only find the pod that is running so it would have hostIP.
			FieldSelector: fmt.Sprintf("status.phase=Running"),
		})
	if err != nil {
		log.Error(err)
		return "", err
	}
	if len(podList.Items) == 0 {
		return "", e.New("No pods in Namespace")
	} else {
		return podList.Items[0].Status.HostIP, nil
	}
}

func (kcm *K8sClientManager) DeployPlan(projectID, collectionID, planID int64, enginesNo int, serviceIP string, containerconfig *config.ExecutorContainer) error {
	pr := planResource{projectID, collectionID, planID}
	planSts := pr.makePlanDeployment(enginesNo, serviceIP, kcm.sc, containerconfig)
	if _, err := kcm.client.AppsV1().StatefulSets(kcm.Namespace).Create(context.TODO(), planSts, metav1.CreateOptions{}); err != nil {
		return err
	}
	service := pr.makePlanService()
	serviceClient := kcm.client.CoreV1().Services(kcm.Namespace)
	if _, err := serviceClient.Create(context.TODO(), service, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func (kcm *K8sClientManager) GetServiceIP(projectID int64) (string, error) {
	igName := projectResource(projectID).makeName()
	service, err := kcm.client.CoreV1().Services(kcm.Namespace).
		Get(context.TODO(), igName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return service.Spec.ClusterIP, nil
}

func (kcm *K8sClientManager) GetIngressUrl(projectID int64) (string, error) {
	igName := projectResource(projectID).makeName()
	serviceClient, err := kcm.client.CoreV1().Services(kcm.Namespace).
		Get(context.TODO(), igName, metav1.GetOptions{})
	if err != nil {
		return "", serrors.MakeSchedulerIngressError(err)
	}
	if kcm.sc.ExecutorConfig.InCluster {
		return serviceClient.Spec.ClusterIP, nil
	}
	if kcm.sc.ExecutorConfig.Cluster.ServiceType == "LoadBalancer" {
		// in case of GCP getting public IP is enough since it exposes to port 80
		if len(serviceClient.Status.LoadBalancer.Ingress) == 0 {
			return "", serrors.MakeIPNotAssignedError()
		}
		return serviceClient.Status.LoadBalancer.Ingress[0].IP, nil
	}
	ip_addr, err := kcm.getRandomHostIP()
	if err != nil {
		return "", serrors.MakeSchedulerIngressError(err)
	}
	exposedPort := serviceClient.Spec.Ports[0].NodePort
	return fmt.Sprintf("%s:%d", ip_addr, exposedPort), nil
}

func (kcm *K8sClientManager) GetProjectAPIKey(projectID int64) (string, error) {
	keySecretName := projectResource(projectID).makeAPIKeySecretName()
	secret, err := kcm.client.CoreV1().Secrets(kcm.Namespace).Get(context.TODO(), keySecretName,
		metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data["api_key"]), nil
}

func (kcm *K8sClientManager) GetPods(labelSelector, fieldSelector string) ([]apiv1.Pod, error) {
	podsClient, err := kcm.client.CoreV1().Pods(kcm.Namespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
		})
	if err != nil {
		return nil, err
	}
	return podsClient.Items, nil
}

func (kcm *K8sClientManager) GetPodsByCollection(collectionID int64, fieldSelector string) ([]apiv1.Pod, error) {
	labelSelector := makeCollectionLabel(collectionID)
	return kcm.GetPods(labelSelector, fieldSelector)
}

func (kcm *K8sClientManager) GetEnginesByProject(projectID int64) ([]apiv1.Pod, error) {
	labelSelector := fmt.Sprintf("project=%d, kind=%s", projectID, smodel.Executor)
	pods, err := kcm.GetPods(labelSelector, "")
	if err != nil {
		return nil, err
	}
	sort.Slice(pods, func(i, j int) bool {
		p1 := pods[i]
		p2 := pods[j]
		return p1.CreationTimestamp.Time.After(p2.CreationTimestamp.Time)
	})
	return pods, nil
}

func (kcm *K8sClientManager) FetchEngineUrlsByPlan(collectionID, planID int64, opts *smodel.EngineOwnerRef) ([]string, error) {
	collectionUrl, err := kcm.GetIngressUrl(opts.ProjectID)
	if err != nil {
		return nil, err
	}
	pr := planResource{opts.ProjectID, collectionID, planID}
	urls := []string{}
	for i := 0; i < opts.EnginesCount; i++ {
		engineSvcName := pr.makeEngineName(i)
		u := fmt.Sprintf("%s/%s", collectionUrl, engineSvcName)
		urls = append(urls, u)
	}
	return urls, nil
}

func (kcm *K8sClientManager) CollectionStatus(projectID, collectionID int64, eps []*model.ExecutionPlan) (*smodel.CollectionStatus, error) {
	planStatuses := make(map[int64]*smodel.PlanStatus)
	cs := &smodel.CollectionStatus{}
	pods, err := kcm.GetPodsByCollection(collectionID, "")
	if err != nil {
		return cs, err
	}
	for _, ep := range eps {
		ps := &smodel.PlanStatus{
			PlanID:  ep.PlanID,
			Engines: ep.Engines,
		}
		planStatuses[ep.PlanID] = ps
	}
	for _, pod := range pods {
		if pod.Labels["kind"] == smodel.Scraper {
			if pod.Status.Phase == apiv1.PodRunning {
				cs.ScraperDeployed = true
			}
			continue
		}
		planID, err := strconv.Atoi(pod.Labels["plan"])
		if err != nil {
			log.Error(err)
		}
		ps, ok := planStatuses[int64(planID)]
		if !ok {
			log.Error("Could not find running pod in ExecutionPlan")
			continue
		}
		ps.EnginesDeployed += 1
	}
	cs.Plans = make([]*smodel.PlanStatus, len(planStatuses))
	n := 0
	for _, ps := range planStatuses {
		cs.Plans[n] = ps
		n += 1
	}
	return cs, err
}

func (kcm *K8sClientManager) GetPodsByCollectionPlan(collectionID, planID int64) ([]apiv1.Pod, error) {
	labelSelector := fmt.Sprintf("plan=%d,collection=%d", planID, collectionID)
	fieldSelector := ""
	return kcm.GetPods(labelSelector, fieldSelector)
}

func (kcm *K8sClientManager) FetchLogFromPod(pod apiv1.Pod) (string, error) {
	logOptions := &apiv1.PodLogOptions{
		Follow: false,
	}
	req := kcm.client.CoreV1().RESTClient().Get().
		Namespace(pod.Namespace).
		Name(pod.Name).
		Resource("pods").
		SubResource("log").
		Param("follow", strconv.FormatBool(logOptions.Follow)).
		Param("container", logOptions.Container).
		Param("previous", strconv.FormatBool(logOptions.Previous)).
		Param("timestamps", strconv.FormatBool(logOptions.Timestamps))
	readCloser, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer readCloser.Close()
	c, err := io.ReadAll(readCloser)
	if err != nil {
		return "", err
	}
	return string(c), nil
}

func (kcm *K8sClientManager) DownloadPodLog(collectionID, planID int64) (string, error) {
	pods, err := kcm.GetPodsByCollectionPlan(collectionID, planID)
	if err != nil {
		return "", err
	}
	if len(pods) > 0 {
		return kcm.FetchLogFromPod(pods[0])
	}
	return "", fmt.Errorf("Cannot find pod for the plan %d", planID)
}

func (kcm *K8sClientManager) PodReadyCount(collectionID int64) int {
	label := makeCollectionLabel(collectionID)
	podsClient, err := kcm.client.CoreV1().Pods(kcm.Namespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s", label),
		})
	if err != nil {
		log.Warn(err)
	}
	ready := 0
	for _, pod := range podsClient.Items {
		if pod.Status.Phase == "Running" {
			ready++
		}
	}
	return ready
}

func (kcm *K8sClientManager) ServiceReachable(engineUrl string) bool {
	resp, err := kcm.httpClient.Get(fmt.Sprintf("https://%s/start", engineUrl))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (kcm *K8sClientManager) deleteService(collectionID int64) error {
	// We could not delete services by label
	// So we firstly get them by label and then delete them one by one
	// you can check here: https://github.com/kubernetes/kubernetes/issues/68468#issuecomment-419981870
	corev1Client := kcm.client.CoreV1().Services(kcm.Namespace)
	resp, err := corev1Client.List(context.TODO(), metav1.ListOptions{
		LabelSelector: makeCollectionLabel(collectionID),
	})
	if err != nil {
		return err
	}

	// If there are any errors in deletion, we only return the last one
	// the errors could be similar so we should avoid return a long list of errors
	var lastError error
	for _, svc := range resp.Items {
		if err := corev1Client.Delete(context.TODO(), svc.Name, metav1.DeleteOptions{}); err != nil {
			lastError = err
		}
	}
	return lastError
}

func (kcm *K8sClientManager) PurgeCollection(collectionID int64) error {
	cr := collectionResource(collectionID)
	label := cr.makeCollectionLabelSelector()
	if err := kcm.client.AppsV1().StatefulSets(kcm.Namespace).DeleteCollection(context.TODO(),
		metav1.DeleteOptions{GracePeriodSeconds: new(int64)}, metav1.ListOptions{LabelSelector: label}); err != nil {
		return err
	}
	err := kcm.deleteService(collectionID)
	if err != nil {
		return err
	}
	if err := kcm.client.CoreV1().ConfigMaps(kcm.Namespace).Delete(context.TODO(), cr.makePromConfigName(),
		metav1.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

func (kcm *K8sClientManager) PurgeProjectIngress(projectID int64) error {
	pr := projectResource(projectID)
	name := pr.makeName()
	deleteOpts := metav1.DeleteOptions{}
	if err := kcm.client.AppsV1().Deployments(kcm.Namespace).Delete(context.TODO(), name, deleteOpts); err != nil {
		log.Error(err)
	}
	if err := kcm.client.CoreV1().Services(kcm.Namespace).Delete(context.TODO(), name, deleteOpts); err != nil {
		log.Error(err)
	}
	secretsClient := kcm.client.CoreV1().Secrets(kcm.Namespace)
	if err := secretsClient.Delete(context.TODO(), name, deleteOpts); err != nil {
		log.Error(err)
	}
	return secretsClient.Delete(context.TODO(), pr.makeAPIKeySecretName(), deleteOpts)
}

func (kcm *K8sClientManager) CreateCollectionScraper(collectionID int64) error {
	cr := collectionResource(collectionID)
	promDeployment := cr.makeScraperDeployment(kcm.scraperServiceAccount, kcm.Namespace,
		kcm.sc.ExecutorConfig.NodeAffinity, kcm.sc.ExecutorConfig.Tolerations, kcm.sc.ScraperContainer)
	promConfig, err := cr.makeScraperConfig(kcm.Namespace, kcm.sc.MetricStorage)
	if err != nil {
		return err
	}
	if _, err := kcm.client.CoreV1().ConfigMaps(kcm.Namespace).Create(context.TODO(), promConfig, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
	}
	if _, err := kcm.client.AppsV1().StatefulSets(kcm.Namespace).Create(context.TODO(), promDeployment, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func (kcm *K8sClientManager) ExposeProject(projectID int64) (*apiv1.Service, error) {
	prj := projectResource(projectID)
	service := prj.makeIngressService(kcm.sc.ExecutorConfig.Cluster.ServiceType)
	// We firstly need to expose the project because we need to the external
	// IP for the certs
	serviceClient := kcm.client.CoreV1().Services(kcm.Namespace)
	createdService, err := serviceClient.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return serviceClient.Get(context.TODO(), prj.makeName(), metav1.GetOptions{})
		}
		return nil, err
	}
	key, err := keys.GenerateAPIKey()
	if err != nil {
		return nil, err
	}
	apiKeySecret := prj.makeAPIKeySecret(key)
	secretClient := kcm.client.CoreV1().Secrets(kcm.Namespace)
	if _, err := secretClient.Create(context.TODO(), apiKeySecret, metav1.CreateOptions{}); err != nil {
		return nil, err
	}
	go func() {
		waitDuration := time.Duration(10 * time.Minute)
		ticker := time.NewTicker(3 * time.Second)
		externalIP := ""
		var err error
	waitLoop:
		for {
			select {
			case <-time.After(waitDuration):
				break waitLoop
			case <-ticker.C:
				externalIP, err = kcm.GetIngressUrl(projectID)
				if err != nil {
					continue waitLoop
				}
				if externalIP == "" {
					continue waitLoop
				}
				break waitLoop
			}
		}
		log.Infof("wait loop is out and the external IP is %s", externalIP)
		if externalIP != "" {
			secret, err := prj.makeKeyPairSecret(kcm.sc.CAPair, externalIP)
			if err != nil {
				log.Error(err)
				return
			}
			if _, err := kcm.client.CoreV1().Secrets(kcm.Namespace).Create(context.TODO(),
				secret, metav1.CreateOptions{}); err != nil {
				log.Error(err)
				return
			}
			igCfg := kcm.sc.IngressConfig
			deployment := prj.makeCoordinatorDeployment(kcm.cdrServiceAccount, igCfg.Image, igCfg.CPU,
				igCfg.Mem, igCfg.Replicas, kcm.sc.ExecutorConfig.Tolerations, secret, apiKeySecret)
			// there could be duplicated controller deployment from multiple collections
			// This method has already taken it into considertion.
			deployClient := kcm.client.AppsV1().Deployments(kcm.Namespace)
			if _, err := deployClient.Create(context.TODO(), deployment, metav1.CreateOptions{}); err != nil {
				log.Error(err)
			}
		}
	}()
	return createdService, nil
}

func (kcm *K8sClientManager) GetDeployedCollections() (map[int64]time.Time, error) {
	labelSelector := fmt.Sprintf("kind=%s", smodel.Executor)
	pods, err := kcm.GetPods(labelSelector, "")
	if err != nil {
		return nil, err
	}
	deployedCollections := make(map[int64]time.Time)
	for _, pod := range pods {
		collectionID, err := strconv.ParseInt(pod.Labels["collection"], 10, 64)
		if err != nil {
			return nil, err
		}
		deployedCollections[collectionID] = pod.CreationTimestamp.Time
	}
	return deployedCollections, nil
}

func (kcm *K8sClientManager) GetDeployedServices() (map[int64]time.Time, error) {
	labelSelector := fmt.Sprintf("kind=%s", smodel.IngressController)
	services, err := kcm.client.CoreV1().Services(kcm.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	deployedServices := make(map[int64]time.Time)
	for _, svc := range services.Items {
		projectID, err := strconv.ParseInt(svc.Labels["project"], 10, 64)
		if err != nil {
			return nil, err
		}
		deployedServices[projectID] = svc.CreationTimestamp.Time
	}
	return deployedServices, nil
}

func (kcm *K8sClientManager) GetCollectionEnginesDetail(projectID, collectionID int64) (*smodel.CollectionDetails, error) {
	labelSelector := fmt.Sprintf("collection=%d", collectionID)
	pods, err := kcm.GetPods(labelSelector, "")
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		return nil, &serrors.NoResourcesFoundErr{Err: err, Message: "Cannot find the engines"}
	}
	collectionDetails := new(smodel.CollectionDetails)
	ingressUrl, err := kcm.GetIngressUrl(projectID)
	if err != nil {
		collectionDetails.IngressIP = err.Error()
	} else {
		collectionDetails.IngressIP = ingressUrl
	}
	engines := []*smodel.EngineStatus{}
	for _, p := range pods {
		es := new(smodel.EngineStatus)
		if kind, _ := p.Labels["kind"]; kind != smodel.Executor {
			continue
		}
		es.Name = p.Name
		es.CreatedTime = p.ObjectMeta.CreationTimestamp.Time
		es.Status = string(p.Status.Phase)
		engines = append(engines, es)
	}
	collectionDetails.Engines = engines
	collectionDetails.ControllerReplicas = kcm.sc.IngressConfig.Replicas
	return collectionDetails, nil
}
