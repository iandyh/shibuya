package scheduler

import (
	e "errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
	model "github.com/rakutentech/shibuya/shibuya/model"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	extbeta1 "k8s.io/api/extensions/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	metricsc "k8s.io/metrics/pkg/client/clientset/versioned"
)

type K8sClientManager struct {
	*config.ExecutorConfig
	client         *kubernetes.Clientset
	metricClient   *metricsc.Clientset
	serviceAccount string
}

var (
	clientMap   sync.Map
	kubectlLock sync.Mutex
)

type projectContext struct {
	clusterID string
	context   string
	client    *kubernetes.Clientset
}

func fetchProjectContexts() {
	contexts, err := model.GetProjectContexts()
	if err != nil {
		log.Fatal(err)
	}
	for _, pc := range contexts {
		client, err := config.GetKubeClientByContext(pc.Context)
		if err != nil {
			log.Print(err)
		}
		log.Printf("Store client %s for %d", pc.Context, pc.ProjectID)
		clientMap.Store(pc.ProjectID, &projectContext{
			client:    client,
			clusterID: pc.ClusterID,
			context:   pc.Context,
		})
	}
	cfg := config.SC.ExecutorConfig.Cluster
	client, err := config.GetKubeClientByContext(cfg.Context)
	if err != nil {
		log.Print("Cannot create default client %v", err)
	}
	clientMap.Store(0, &projectContext{
		clusterID: cfg.ClusterID,
		context:   cfg.Context,
		client:    client,
	})
}

func NewK8sClientManager(cfg *config.ClusterConfig) *K8sClientManager {
	c, err := config.GetKubeClient()
	if err != nil {
		log.Warning(err)
	}
	metricsc, err := config.GetMetricsClient()
	if err != nil {
		log.Fatal(err)
	}
	fetchProjectContexts()
	return &K8sClientManager{
		config.SC.ExecutorConfig, c, metricsc, "shibuya-ingress-serviceaccount",
	}
}

func makeNodeAffinity(key, value string) *apiv1.NodeAffinity {
	nodeAffinity := &apiv1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &apiv1.NodeSelector{
			NodeSelectorTerms: []apiv1.NodeSelectorTerm{
				{
					MatchExpressions: []apiv1.NodeSelectorRequirement{
						{
							Key:      key,
							Operator: apiv1.NodeSelectorOpIn,
							Values: []string{
								value,
							},
						},
					},
				},
			},
		},
	}
	return nodeAffinity
}

func collectionNodeAffinity(collectionID int64) *apiv1.NodeAffinity {
	collectionIDStr := fmt.Sprintf("%d", collectionID)
	return makeNodeAffinity("collection_id", collectionIDStr)
}

func prepareAffinity(collectionID int64) *apiv1.Affinity {
	affinity := &apiv1.Affinity{}
	if config.SC.ExecutorConfig.Cluster.OnDemand {
		affinity.NodeAffinity = collectionNodeAffinity(collectionID)
		return affinity
	}
	na := config.SC.ExecutorConfig.NodeAffinity
	if len(na) > 0 {
		t := na[0]
		affinity.NodeAffinity = makeNodeAffinity(t["key"], t["value"])
		return affinity
	}
	return affinity
}

func (kcm *K8sClientManager) makeHostAliases() []apiv1.HostAlias {
	if kcm.HostAliases != nil {
		hostAliases := []apiv1.HostAlias{}
		for _, ha := range kcm.HostAliases {
			hostAliases = append(hostAliases, apiv1.HostAlias{
				Hostnames: []string{ha.Hostname},
				IP:        ha.IP,
			})
		}
		return hostAliases
	}
	return []apiv1.HostAlias{}
}

func (kcm *K8sClientManager) generateEngineDeployment(engineName string, labels map[string]string,
	containerConfig *config.ExecutorContainer, affinity *apiv1.Affinity) appsv1.Deployment {
	t := true
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       engineName,
			DeletionGracePeriodSeconds: new(int64),
			Labels:                     labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: apiv1.PodSpec{
					Affinity:                     affinity,
					ServiceAccountName:           kcm.serviceAccount,
					AutomountServiceAccountToken: &t,
					ImagePullSecrets: []apiv1.LocalObjectReference{
						{
							Name: kcm.ImagePullSecret,
						},
					},
					TerminationGracePeriodSeconds: new(int64),
					HostAliases:                   kcm.makeHostAliases(),
					Containers: []apiv1.Container{
						{
							Name:            engineName,
							Image:           containerConfig.Image,
							ImagePullPolicy: kcm.ImagePullPolicy,
							Resources: apiv1.ResourceRequirements{
								Limits: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse(containerConfig.CPU),
									apiv1.ResourceMemory: resource.MustParse(containerConfig.Mem),
								},
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse(containerConfig.CPU),
									apiv1.ResourceMemory: resource.MustParse(containerConfig.Mem),
								},
							},
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}

func (kcm *K8sClientManager) deploy(deployment *appsv1.Deployment, client *kubernetes.Clientset) error {
	deploymentsClient := client.AppsV1().Deployments(kcm.Namespace)
	_, err := deploymentsClient.Create(deployment)
	if errors.IsAlreadyExists(err) {
		// do nothing if already exists
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

func (kcm *K8sClientManager) loadClientCache(projectID int64) *projectContext {
	raw, ok := clientMap.Load(projectID)
	if !ok {
		raw, _ := clientMap.Load(0)
		return raw.(*projectContext)
	}
	return raw.(*projectContext)
}

func (kcm *K8sClientManager) findClientByProject(projectID int64) *kubernetes.Clientset {
	// TODO: need to deal with newly created projects
	pc := kcm.loadClientCache(projectID)
	return pc.client
}

func (kcm *K8sClientManager) findContextByProject(projectID int64) string {
	pc := kcm.loadClientCache(projectID)
	return pc.context
}

func (kcm *K8sClientManager) GetClusterIDByProject(projectID int64) string {
	pc := kcm.loadClientCache(projectID)
	return pc.clusterID
}

func (kcm *K8sClientManager) expose(client *kubernetes.Clientset, name string, deployment *appsv1.Deployment) error {
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"networking.istio.io/exportTo": ".",
			},
			Labels: deployment.Spec.Template.ObjectMeta.Labels,
		},
		Spec: apiv1.ServiceSpec{
			Selector: deployment.Spec.Template.ObjectMeta.Labels,
			Ports: []apiv1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromString("http"),
				},
			},
		},
	}
	if deployment.Labels["kind"] == "ingress-controller" {
		if !kcm.InCluster {
			service.Spec.Type = apiv1.ServiceTypeNodePort
		}
		if kcm.Cluster.OnDemand {
			service.Spec.ExternalTrafficPolicy = "Local"
			service.Spec.Type = apiv1.ServiceTypeLoadBalancer
		}
	}
	_, err := client.CoreV1().Services(kcm.Namespace).Create(service)
	if errors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

func (kcm *K8sClientManager) getRandomHostIP(client *kubernetes.Clientset) (string, error) {
	podList, err := client.CoreV1().Pods(kcm.Namespace).
		List(metav1.ListOptions{
			Limit: 1,
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

func (kcm *K8sClientManager) DeployEngine(projectID, collectionID, planID int64,
	engineID int, containerConfig *config.ExecutorContainer) error {
	engineName := makeEngineName(projectID, collectionID, planID, engineID)
	labels := makeEngineLabel(projectID, collectionID, planID, engineName)
	affinity := prepareAffinity(collectionID)
	engineConfig := kcm.generateEngineDeployment(engineName, labels, containerConfig, affinity)
	client := kcm.findClientByProject(projectID)
	if err := kcm.deploy(&engineConfig, client); err != nil {
		return err
	}
	engineSvcName := makeServiceName(projectID, collectionID, planID, engineID)
	if err := kcm.expose(client, engineSvcName, &engineConfig); err != nil {
		return err
	}
	ingressClass := makeIngressClass(collectionID)
	ingressName := makeIngressName(projectID, collectionID, planID, engineID)
	if err := kcm.CreateIngress(client, ingressClass, ingressName, engineSvcName, collectionID, projectID); err != nil {
		return err
	}
	log.Printf("Finish creating one engine for %s", engineName)
	return nil
}

func (kcm *K8sClientManager) GetIngressUrl(projectID, collectionID int64) (string, error) {
	client := kcm.findClientByProject(projectID)
	igName := makeIngressClass(collectionID)
	serviceClient, err := client.CoreV1().Services(kcm.Namespace).
		Get(igName, metav1.GetOptions{})
	if err != nil {
		return "", makeSchedulerIngressError(err)
	}
	if kcm.InCluster {
		return serviceClient.Spec.ClusterIP, nil
	}
	if kcm.Cluster.OnDemand {
		// in case of GCP getting public IP is enough since it exposes to port 80
		if len(serviceClient.Status.LoadBalancer.Ingress) == 0 {
			return "", makeIPNotAssignedError()
		}
		return serviceClient.Status.LoadBalancer.Ingress[0].IP, nil
	}
	ip_addr, err := kcm.getRandomHostIP(client)
	if err != nil {
		return "", makeSchedulerIngressError(err)
	}
	exposedPort := serviceClient.Spec.Ports[0].NodePort
	return fmt.Sprintf("%s:%d", ip_addr, exposedPort), nil
}

func (kcm *K8sClientManager) GetPods(client *kubernetes.Clientset, labelSelector, fieldSelector string) ([]apiv1.Pod, error) {
	podsClient, err := client.CoreV1().Pods(kcm.Namespace).
		List(metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
		})
	if err != nil {
		return nil, err
	}
	return podsClient.Items, nil
}

func (kcm *K8sClientManager) GetPodsByCollection(client *kubernetes.Clientset, collectionID int64, fieldSelector string) []apiv1.Pod {
	labelSelector := fmt.Sprintf("collection=%d", collectionID)
	pods, err := kcm.GetPods(client, labelSelector, fieldSelector)
	if err != nil {
		log.Warn(err)
	}
	return pods
}

func (kcm *K8sClientManager) FetchEngineUrlsByPlan(collectionID, planID int64, opts *smodel.EngineOwnerRef) ([]string, error) {
	projectID := opts.ProjectID
	collectionUrl, err := kcm.GetIngressUrl(projectID, collectionID)
	if err != nil {
		return nil, err
	}
	urls := []string{}
	for i := 0; i < opts.EnginesCount; i++ {
		engineSvcName := makeServiceName(opts.ProjectID, collectionID, planID, i)
		u := fmt.Sprintf("%s/%s", collectionUrl, engineSvcName)
		urls = append(urls, u)
	}
	return urls, nil
}

func (kcm *K8sClientManager) CollectionStatus(projectID, collectionID int64, eps []*model.ExecutionPlan) (*smodel.CollectionStatus, error) {
	planStatuses := make(map[int64]*smodel.PlanStatus)
	var engineReachable bool
	client := kcm.findClientByProject(projectID)
	cs := &smodel.CollectionStatus{}
	pods := kcm.GetPodsByCollection(client, collectionID, "")
	ingressControllerDeployed := false
	for _, ep := range eps {
		ps := &smodel.PlanStatus{
			PlanID:  ep.PlanID,
			Engines: ep.Engines,
		}
		planStatuses[ep.PlanID] = ps
	}
	enginesReady := true
	for _, pod := range pods {
		if pod.Labels["kind"] == "ingress-controller" {
			ingressControllerDeployed = true
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
		if pod.Status.Phase != apiv1.PodRunning {
			enginesReady = false
		}
	}
	// if it's unrechable, we can assume it's not in progress as well
	fieldSelector := fmt.Sprintf("status.phase=Running")
	ingressPods := kcm.GetPodsByCollection(client, collectionID, fieldSelector)
	ingressControllerDeployed = len(ingressPods) >= 1
	if !ingressControllerDeployed || !enginesReady {
		for _, ps := range planStatuses {
			cs.Plans = append(cs.Plans, ps)
		}
		return cs, nil
	}
	engineReachable = false
	randomPlan := eps[0]
	opts := &smodel.EngineOwnerRef{
		ProjectID:    projectID,
		EnginesCount: randomPlan.Engines,
	}
	engineUrls, err := kcm.FetchEngineUrlsByPlan(collectionID, randomPlan.PlanID, opts)
	if err == nil {
		randomEngine := engineUrls[0]
		engineReachable = kcm.ServiceReachable(randomEngine)
	}
	jobs := make(chan *smodel.PlanStatus)
	result := make(chan *smodel.PlanStatus)
	for w := 0; w < len(eps); w++ {
		go smodel.GetPlanStatus(collectionID, jobs, result)
	}
	for _, ps := range planStatuses {
		jobs <- ps
	}
	defer close(jobs)
	defer close(result)
	for range eps {
		ps := <-result
		if ps.Engines == ps.EnginesDeployed && engineReachable {
			ps.EnginesReachable = true
		}
		cs.Plans = append(cs.Plans, ps)
	}
	return cs, nil
}

func (kcm *K8sClientManager) GetPodsByCollectionPlan(client *kubernetes.Clientset, collectionID, planID int64) ([]apiv1.Pod, error) {
	labelSelector := fmt.Sprintf("plan=%d,collection=%d", planID, collectionID)
	fieldSelector := ""
	return kcm.GetPods(client, labelSelector, fieldSelector)
}

func (kcm *K8sClientManager) FetchLogFromPod(client *kubernetes.Clientset, pod apiv1.Pod) (string, error) {
	logOptions := &apiv1.PodLogOptions{
		Follow: false,
	}
	req := client.CoreV1().RESTClient().Get().
		Namespace(pod.Namespace).
		Name(pod.Name).
		Resource("pods").
		SubResource("log").
		Param("follow", strconv.FormatBool(logOptions.Follow)).
		Param("container", logOptions.Container).
		Param("previous", strconv.FormatBool(logOptions.Previous)).
		Param("timestamps", strconv.FormatBool(logOptions.Timestamps))
	readCloser, err := req.Stream()
	if err != nil {
		return "", err
	}
	defer readCloser.Close()
	c, err := ioutil.ReadAll(readCloser)
	if err != nil {
		return "", err
	}
	return string(c), nil
}

func (kcm *K8sClientManager) DownloadPodLog(projectID, collectionID, planID int64) (string, error) {
	client := kcm.findClientByProject(projectID)
	pods, err := kcm.GetPodsByCollectionPlan(client, collectionID, planID)
	if err != nil {
		return "", err
	}
	if len(pods) > 0 {
		return kcm.FetchLogFromPod(client, pods[0])
	}
	return "", fmt.Errorf("Cannot find pod for the plan %d", planID)
}

func (kcm *K8sClientManager) PodReadyCount(projectID, collectionID int64) int {
	client := kcm.findClientByProject(projectID)
	label := makeCollectionLabel(collectionID)
	podsClient, err := client.CoreV1().Pods(kcm.Namespace).
		List(metav1.ListOptions{
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
	resp, err := http.Get(fmt.Sprintf("http://%s/start", engineUrl))
	if err != nil {
		log.Warn(err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (kcm *K8sClientManager) deleteService(context string, collectionID int64) error {
	// Delete services by collection is not supported as of yet
	// Wait for this PR to be merged - https://github.com/kubernetes/kubernetes/pull/85802
	kubectlLock.Lock()
	defer kubectlLock.Unlock()

	cmd := exec.Command("kubectl", "config", "use-context", context)
	o, err := cmd.Output()
	if err != nil {
		log.Printf("Cannot switch context for %s", context)
		return err
	}
	cmd = exec.Command("kubectl", "-n", kcm.Namespace, "delete", "svc", "--force", "--grace-period=0", "-l", fmt.Sprintf("collection=%d", collectionID))
	o, err = cmd.Output()
	if err != nil {
		log.Printf("Cannot delete services for collection %d", collectionID)
		return err
	}
	log.Print(string(o))
	return nil
}

func (kcm *K8sClientManager) deleteDeployment(client *kubernetes.Clientset, collectionID int64) error {
	deploymentsClient := client.AppsV1().Deployments(kcm.Namespace)
	err := deploymentsClient.DeleteCollection(&metav1.DeleteOptions{
		GracePeriodSeconds: new(int64),
	}, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("collection=%d", collectionID),
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (kcm *K8sClientManager) PurgeCollection(projectID int64, collectionID int64) error {
	client := kcm.findClientByProject(projectID)
	err := kcm.deleteDeployment(client, collectionID)
	if err != nil {
		return err
	}
	context := kcm.findContextByProject(projectID)
	err = kcm.deleteService(context, collectionID)
	if err != nil {
		return err
	}
	err = kcm.deleteIngressRules(client, collectionID)
	if err != nil {
		return err
	}
	return nil
}

func int32Ptr(i int32) *int32 { return &i }

func (kcm *K8sClientManager) generateControllerDeployment(igName string, collectionID, projectID int64) appsv1.Deployment {
	publishService := fmt.Sprintf("--publish-service=$(POD_NAMESPACE)/%s", igName)
	affinity := prepareAffinity(collectionID)
	t := true
	labels := makeIngressLabel(collectionID, projectID)
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   igName,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"prometheus.io/port":   "10254",
						"prometheus.io/scrape": "true",
					},
				},
				Spec: apiv1.PodSpec{
					Affinity:                      affinity,
					ServiceAccountName:            kcm.serviceAccount,
					TerminationGracePeriodSeconds: new(int64),
					AutomountServiceAccountToken:  &t,
					Containers: []apiv1.Container{
						{
							Name:  "nginx-ingress-controller",
							Image: config.SC.IngressConfig.Image,
							Args: []string{
								"/nginx-ingress-controller",
								fmt.Sprintf("--ingress-class=%s", igName),
								"--configmap=$(POD_NAMESPACE)/nginx-configuration",
								publishService,
								"--annotations-prefix=nginx.ingress.kubernetes.io",
								fmt.Sprintf("--watch-namespace=%s", kcm.Namespace),
							},
							SecurityContext: &apiv1.SecurityContext{
								Capabilities: &apiv1.Capabilities{
									Drop: []apiv1.Capability{
										"ALL",
									},
									Add: []apiv1.Capability{
										"NET_BIND_SERVICE",
									},
								},
							},
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
								},
								{
									Name:          "https",
									ContainerPort: 443,
								},
							},
							Env: []apiv1.EnvVar{
								apiv1.EnvVar{
									Name: "POD_NAME",
									ValueFrom: &apiv1.EnvVarSource{
										FieldRef: &apiv1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								apiv1.EnvVar{
									Name: "POD_NAMESPACE",
									ValueFrom: &apiv1.EnvVarSource{
										FieldRef: &apiv1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}

func (kcm *K8sClientManager) ApplyIngressPrerequisite(projectID int64) {
	mandatory := fmt.Sprintf("/ingress/mandatory.yaml")
	namespace := kcm.Namespace
	cmd := exec.Command("kubectl", "-n", namespace, "apply", "-f", mandatory)
	o, err := cmd.Output()
	if err != nil {
		log.Printf("Cannot apply mandatory.yaml")
		log.Error(err)
	}
	log.Print(string(o))
	client := kcm.findClientByProject(projectID)
	err = kcm.CreateRoleBinding(client)
	if err != nil {
		log.Error(err)
	}
	log.Printf("Prerequisites are applied to the cluster")
}

func (kcm *K8sClientManager) ExposeCollection(projectID, collectionID int64) error {
	//kcm.ApplyIngressPrerequisite()
	igName := makeIngressClass(collectionID)
	deployment := kcm.generateControllerDeployment(igName, collectionID, projectID)
	client := kcm.findClientByProject(projectID)
	if err := kcm.deploy(&deployment, client); err != nil {
		return err
	}
	if err := kcm.expose(client, igName, &deployment); err != nil {
		return err
	}
	return nil
}

func (kcm *K8sClientManager) CreateIngress(client *kubernetes.Clientset, ingressClass, ingressName, serviceName string, collectionID, projectID int64) error {
	ingressRule := extbeta1.IngressRule{}
	ingressRule.HTTP = &extbeta1.HTTPIngressRuleValue{
		Paths: []extbeta1.HTTPIngressPath{
			extbeta1.HTTPIngressPath{
				Path: fmt.Sprintf("/%s", serviceName),
				Backend: extbeta1.IngressBackend{
					ServiceName: serviceName,
					ServicePort: intstr.FromInt(80),
				},
			},
		},
	}
	ingress := extbeta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: ingressName,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                    ingressClass,
				"nginx.ingress.kubernetes.io/rewrite-target":     "/",
				"nginx.ingress.kubernetes.io/ssl-redirect":       "false",
				"nginx.ingress.kubernetes.io/proxy-body-size":    "100m",
				"nginx.ingress.kubernetes.io/proxy-send-timeout": "600",
				"nginx.ingress.kubernetes.io/proxy-read-timeout": "600",
			},
			Labels: makeIngressLabel(collectionID, projectID),
		},
		Spec: extbeta1.IngressSpec{
			Rules: []extbeta1.IngressRule{ingressRule},
		},
	}
	_, err := client.ExtensionsV1beta1().Ingresses(kcm.Namespace).Create(&ingress)
	if err != nil {
		log.Error(err)
	}
	return nil
}

func (kcm *K8sClientManager) deleteIngressRules(client *kubernetes.Clientset, collectionID int64) error {
	deletePolicy := metav1.DeletePropagationForeground
	return client.ExtensionsV1beta1().Ingresses(kcm.Namespace).DeleteCollection(&metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("collection=%d", collectionID),
	})
}

func (kcm *K8sClientManager) CreateRoleBinding(client *kubernetes.Clientset) error {
	namespace := kcm.Namespace
	nginxRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shibuya-ingress-role-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      kcm.serviceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "shibuya-ingress-role",
		},
	}
	_, err := client.RbacV1().RoleBindings(namespace).Create(nginxRoleBinding)
	if errors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func (kcm *K8sClientManager) GetNodesByCollection(projectID, collectionID int64) ([]apiv1.Node, error) {
	client := kcm.findClientByProject(projectID)
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("collection_id=%d", collectionID),
	}
	return kcm.getNodes(client, opts)
}

func (kcm *K8sClientManager) getNodes(client *kubernetes.Clientset, opts metav1.ListOptions) ([]apiv1.Node, error) {
	nodeList, err := client.CoreV1().Nodes().List(opts)
	if err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func (kcm *K8sClientManager) GetAllNodesInfo() (smodel.AllNodesInfo, error) {
	opts := metav1.ListOptions{}
	r := make(smodel.AllNodesInfo)
	clientMap.Range(func(key interface{}, value interface{}) bool {
		client := value.(*projectContext).client
		nodes, err := kcm.getNodes(client, opts)
		if err == nil {
			for _, node := range nodes {
				nodeInfo := r[node.ObjectMeta.Labels["collection_id"]]
				if nodeInfo == nil {
					nodeInfo = &smodel.NodesInfo{}
					r[node.ObjectMeta.Labels["collection_id"]] = nodeInfo
				}
				nodeInfo.Size++
				if nodeInfo.LaunchTime.IsZero() || nodeInfo.LaunchTime.After(node.ObjectMeta.CreationTimestamp.Time) {
					nodeInfo.LaunchTime = node.ObjectMeta.CreationTimestamp.Time
				}
			}
		}
		return true
	})
	return r, nil
}

func (kcm *K8sClientManager) GetDeployedCollections() (map[int64]time.Time, error) {
	deployedCollections := make(map[int64]time.Time)
	clientMap.Range(func(key interface{}, value interface{}) bool {
		client := value.(*projectContext).client
		labelSelector := fmt.Sprintf("kind=executor")
		pods, err := kcm.GetPods(client, labelSelector, "")
		if err == nil {
			for _, pod := range pods {
				collectionID, err := strconv.ParseInt(pod.Labels["collection"], 10, 64)
				if err == nil {
					deployedCollections[collectionID] = pod.CreationTimestamp.Time
				}
			}
		}
		return true
	})

	return deployedCollections, nil
}

func (kcm *K8sClientManager) GetPodsMetrics(collectionID, planID int64) (map[string]apiv1.ResourceList, error) {
	metricsList, err := kcm.metricClient.MetricsV1beta1().PodMetricses(kcm.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("collection=%d,plan=%d", collectionID, planID),
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]apiv1.ResourceList, len(metricsList.Items))
	for _, pm := range metricsList.Items {
		for _, cm := range pm.Containers {
			result[getEngineNumber(pm.GetName())] = cm.Usage
		}
	}
	return result, nil
}

func getEngineNumber(podName string) string {
	return strings.Split(podName, "-")[4]
}
