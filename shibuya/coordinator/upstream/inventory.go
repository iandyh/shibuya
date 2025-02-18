package upstream

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Inventory struct {
	namespace             string
	inventoryByCollection map[string][]EngineEndPoint
	engineInventory       map[string]EngineEndPoint
	mu                    sync.RWMutex
	client                *kubernetes.Clientset
}

func NewInventory(namespace string, inCluster bool) (*Inventory, error) {
	client, err := makeK8sClient(inCluster)
	if err != nil {
		return nil, err
	}
	return &Inventory{
		inventoryByCollection: make(map[string][]EngineEndPoint),
		engineInventory:       make(map[string]EngineEndPoint),
		client:                client,
		namespace:             namespace,
	}, nil
}

type EngineEndPoint struct {
	collectionID string
	addr         string
	path         string
	planID       string
}

func (ivt *Inventory) FindPodIP(url string) string {
	ivt.mu.RLock()
	defer ivt.mu.RUnlock()

	item, ok := ivt.engineInventory[url]
	if !ok {
		return ""
	}
	return item.addr
}

func (ivt *Inventory) GetEndpointsCountByCollection(collectionID string) int {
	ivt.mu.RLock()
	defer ivt.mu.RUnlock()

	return len(ivt.inventoryByCollection[collectionID])
}

func (ivt *Inventory) updateInventory(inventoryByCollection map[string][]EngineEndPoint) {
	ivt.mu.Lock()
	defer ivt.mu.Unlock()
	log.Debugf("Going to update inventory with following states %v", inventoryByCollection)

	for collectionID, ep := range inventoryByCollection {
		ivt.inventoryByCollection[collectionID] = ep
		log.Infof("Updated %s with number of eps %d", collectionID, len(ep))
		for _, ee := range ep {
			ivt.engineInventory[ee.path] = ee
			log.Infof("Added engine %s with addr %s into inventory", ee.path, ee.addr)
		}
	}
	for path, ee := range ivt.engineInventory {
		if _, ok := inventoryByCollection[ee.collectionID]; !ok {
			delete(ivt.engineInventory, path)
			log.Infof("Cleaned the inventory for engine with path %s", path)
		}
	}
}

func (ivt *Inventory) MakeInventory(projectID string) {
	labelSelector := fmt.Sprintf("project=%s", projectID)
	client := ivt.client
	for {
		time.Sleep(3 * time.Second)
		resp, err := client.CoreV1().Endpoints(ivt.namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			log.Error(err)
			continue
		}
		// can we have the race condition that the inventory we make could make the shibuya controller mistakenly thinks the engines are ready?
		// controller is already checking whether all the engines within one collection are in running state
		// How can ensure the atomicity?
		inventoryByCollection := make(map[string][]EngineEndPoint)
		skipedCollections := make(map[string]struct{})
		for _, planEndpoints := range resp.Items {
			// need to sort the endpoints and update the inventory
			collectionID := planEndpoints.Labels["collection"]

			// If any of the plans inside the collection is not ready, we skip the further check
			if _, ok := skipedCollections[collectionID]; ok {
				log.Debugf("Collection %s is not ready, skip.", collectionID)
				continue
			}
			projectID := planEndpoints.Labels["project"]
			planID := planEndpoints.Labels["plan"]
			kind := planEndpoints.Labels["kind"]

			if kind != "executor" {
				continue
			}
			collectionReady := true
			subsets := planEndpoints.Subsets
			var engineEndpoints []apiv1.EndpointAddress
			if len(subsets) == 0 {
				collectionReady = false
			} else { // only some engines could be in ready state. We need to check whether they are fully ready
				engineEndpoints = subsets[0].Addresses
				planEngineCount, err := ivt.getPlanEnginesCount(projectID, collectionID, planID)
				if err != nil {
					log.Debugf("Getting count error %v", err)
					collectionReady = false
				}
				// If the engpoints are less than the pod count, it means the pods are not ready yet, we should skip
				log.Debugf("Engine endpoints count %d", len(engineEndpoints))
				log.Debugf("Number of engines in the plan %d", planEngineCount)
				if len(engineEndpoints) < planEngineCount {
					collectionReady = false
				}
			}
			if !collectionReady {
				skipedCollections[collectionID] = struct{}{}
				continue
			}
			ports := subsets[0].Ports
			if len(ports) == 0 {
				//TODO is this an error? Shall we handle it?
				continue
			}
			port := ports[0].Port
			for _, e := range engineEndpoints {
				podName := e.TargetRef.Name
				inventoryByCollection[collectionID] = append(inventoryByCollection[collectionID], EngineEndPoint{
					path:         podName,
					addr:         fmt.Sprintf("%s:%d", e.IP, port),
					collectionID: collectionID,
					planID:       planID,
				})
			}
		}
		ivt.updateInventory(inventoryByCollection)
	}
}

func (ivt *Inventory) GetPlanEndpoints(collectionID, planID string) []string {
	ivt.mu.RLock()
	defer ivt.mu.RUnlock()
	endpointsByCollection := ivt.inventoryByCollection[collectionID]
	endpointsByPlan := make([]string, 0)
	for _, ep := range endpointsByCollection {
		if ep.planID != planID {
			continue
		}
		endpointsByPlan = append(endpointsByPlan, ep.addr)
	}
	return endpointsByPlan
}

func (ivt *Inventory) getPlanEnginesCount(projectID, collectionID, planID string) (int, error) {
	planName := fmt.Sprintf("engine-%s-%s-%s", projectID, collectionID, planID)
	resp, err := ivt.client.AppsV1().StatefulSets(ivt.namespace).Get(context.TODO(), planName, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}
	return int(*resp.Spec.Replicas), nil
}

func makeK8sClient(inCluster bool) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if inCluster {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		kubeconfig := path.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, err
}
