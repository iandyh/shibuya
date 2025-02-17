package k8s

import (
	"fmt"
	"strconv"

	"github.com/rakutentech/shibuya/shibuya/certmanager"
	"github.com/rakutentech/shibuya/shibuya/config"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type projectResource int64

func makeIngressControllerLabel(projectID int64) map[string]string {
	base := map[string]string{}
	base["kind"] = smodel.IngressController
	base["project"] = strconv.FormatInt(projectID, 10)
	return base
}

func (p projectResource) makeName() string {
	return fmt.Sprintf("ig-%d", p)
}

func (p projectResource) makeAPIKeySecretName() string {
	return fmt.Sprintf("%s-key", p.makeName())
}

func (p projectResource) makeIngressService(serviceType string) *apiv1.Service {
	labels := makeIngressControllerLabel(int64(p))
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.makeName(),
			Annotations: map[string]string{
				"networking.istio.io/exportTo": ".",
			},
			Labels: labels,
		},
		Spec: apiv1.ServiceSpec{
			Selector: labels,
			Ports: []apiv1.ServicePort{
				{
					Name:       "http",
					Port:       443,
					TargetPort: intstr.FromString("http"),
				},
				{
					Name:       "pubsub",
					Port:       2416,
					TargetPort: intstr.FromInt(2416),
				},
			},
		},
	}
	switch serviceType {
	case "NodePort":
		service.Spec.Type = apiv1.ServiceTypeNodePort
	case "LoadBalancer":
		service.Spec.ExternalTrafficPolicy = "Local"
		service.Spec.Type = apiv1.ServiceTypeLoadBalancer
	}
	return service
}

func (p projectResource) makeCertKeySecret(cert, key []byte) *apiv1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.makeName(),
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": cert, // Certificate
			"tls.key": key,  // Private key
		},
	}
	return secret
}

func (p projectResource) makeKeyPairSecret(ca *config.CAPair, externalIP string) (*apiv1.Secret, error) {
	cert, key, err := certmanager.GenCertAndKey(ca, int64(p), externalIP)
	if err != nil {
		return nil, err
	}
	return p.makeCertKeySecret(cert, key), nil
}

func (p projectResource) makeAPIKeySecret(key string) *apiv1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.makeAPIKeySecretName(),
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"api_key": []byte(key),
		},
	}
}

func (p projectResource) makeCoordinatorDeployment(serviceAccount, image, cpu, memory string, replicas int32,
	cfgTolerations []config.Toleration, secret, apiKeySecret *apiv1.Secret) *appsv1.Deployment {
	name := p.makeName()
	volumeName := "tls"
	volumes := []apiv1.Volume{
		makeSecretVolume("tls", secret.Name),
	}
	volumeMounts := []apiv1.VolumeMount{
		makeVolumeMount(volumeName, "/tls", true)}
	tolerations := prepareTolerations(cfgTolerations)
	labels := makeIngressControllerLabel(int64(p))
	t := true
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(replicas),
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
					Tolerations:                  tolerations,
					ServiceAccountName:           serviceAccount,
					AutomountServiceAccountToken: &t,
					Volumes:                      volumes,
					Containers: []apiv1.Container{
						{
							Name:  smodel.IngressController,
							Image: image,
							Resources: apiv1.ResourceRequirements{
								// Limits are whatever Kubernetes sets as the max value
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse(cpu),
									apiv1.ResourceMemory: resource.MustParse(memory),
								},
								Limits: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse(cpu),
									apiv1.ResourceMemory: resource.MustParse(memory),
								},
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
									ContainerPort: 8080,
								},
								{
									Name:          "https",
									ContainerPort: 443,
								},
							},
							Env: []apiv1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &apiv1.EnvVarSource{
										FieldRef: &apiv1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &apiv1.EnvVarSource{
										FieldRef: &apiv1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "project_id",
									Value: fmt.Sprintf("%d", p),
								},
								{
									Name: "api_key",
									ValueFrom: &apiv1.EnvVarSource{
										SecretKeyRef: &apiv1.SecretKeySelector{
											LocalObjectReference: apiv1.LocalObjectReference{
												Name: apiKeySecret.Name,
											},
											Key: "api_key",
										},
									},
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
				},
			},
		},
	}
	return deployment
}

func makeSecretVolume(volumeName, secretName string) apiv1.Volume {
	return apiv1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
}

func makeVolumeMount(volumeName, mountPath string, readOnly bool) apiv1.VolumeMount {
	return apiv1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
		ReadOnly:  readOnly,
	}
}
