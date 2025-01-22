package scheduler

import (
	"fmt"
	"strconv"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/engines/metrics"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type collectionResource int64

func (cr collectionResource) makeScraperConfig(namespace string,
	metricStorage []config.MetricStorage) (*apiv1.ConfigMap, error) {
	pc, err := metrics.MakeScraperConfig(int64(cr), namespace, metricStorage)
	if err != nil {
		return nil, err
	}
	c, err := yaml.Marshal(pc)
	if err != nil {
		return nil, err
	}
	data := map[string]string{}
	data["prometheus.yml"] = string(c)
	labels := cr.makeScraperLabels()
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.makePromConfigName(),
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}, nil
}

func (cr collectionResource) makeScraperDeployment(serviceAccount, namespace string, nodeAffinity []map[string]string,
	cfgTolerations []config.Toleration, scraperContainer config.ScraperContainer) *appsv1.StatefulSet {
	workloadName := cr.makeScraperDeploymentName()
	labels := cr.makeScraperLabels()
	// Currently scraper shares the affinity and tolerations with executors
	affinity := prepareAffinity(int64(cr), nodeAffinity)
	tolerations := prepareTolerations(cfgTolerations)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workloadName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: apiv1.PodSpec{
					ServiceAccountName: serviceAccount,
					Affinity:           affinity,
					Tolerations:        tolerations,
					Containers: []apiv1.Container{
						{
							Name:  "prom",
							Image: scraperContainer.Image,
							Resources: apiv1.ResourceRequirements{
								Limits: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse(scraperContainer.CPU),
									apiv1.ResourceMemory: resource.MustParse(scraperContainer.Mem),
								},
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse(scraperContainer.CPU),
									apiv1.ResourceMemory: resource.MustParse(scraperContainer.Mem),
								},
							},
							Ports: []apiv1.ContainerPort{
								{
									ContainerPort: int32(9090),
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "prom-config",
									MountPath: "/etc/prometheus",
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: "prom-config",
							VolumeSource: apiv1.VolumeSource{
								ConfigMap: &apiv1.ConfigMapVolumeSource{
									LocalObjectReference: apiv1.LocalObjectReference{
										Name: cr.makePromConfigName(),
									},
									DefaultMode: int32Ptr(420),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (cr collectionResource) makeScraperDeploymentName() string {
	return fmt.Sprintf("prom-collection-%d", cr)
}

func (cr collectionResource) makeScraperLabels() map[string]string {
	return map[string]string{
		"kind":       "scraper",
		"collection": strconv.FormatInt(int64(cr), 10),
	}
}

func (cr collectionResource) makePromConfigName() string {
	return fmt.Sprintf("prom-collection-%d", cr)
}
