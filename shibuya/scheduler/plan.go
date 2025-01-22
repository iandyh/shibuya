package scheduler

import (
	"fmt"
	"strconv"

	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type planResource struct {
	projectID, collectionID, planID int64
}

func (plan planResource) makePlanDeployment(replicas int, sc config.ShibuyaConfig,
) *appsv1.StatefulSet {
	planName := plan.makeName()
	envvars := plan.makeEngineMetaEnvvars()
	labels := plan.makePlanLabel()
	affinity := prepareAffinity(plan.collectionID, sc.ExecutorConfig.NodeAffinity)
	tolerations := prepareTolerations(sc.ExecutorConfig.Tolerations)
	executorConfig := sc.ExecutorConfig.JmeterContainer.ExecutorContainer
	t := true
	volumes := []apiv1.Volume{}
	volumeMounts := []apiv1.VolumeMount{}
	if object_storage.IsProviderGCP(sc.ObjectStorage.Provider) {
		volumeName := "shibuya-gcp-auth"
		secretName := sc.ObjectStorage.SecretName
		authFileName := sc.ObjectStorage.AuthFileName
		mountPath := fmt.Sprintf("/auth/%s", authFileName)
		v := apiv1.Volume{
			Name: volumeName,
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		volumes = append(volumes, v)
		vm := apiv1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
			SubPath:   authFileName,
		}
		volumeMounts = append(volumeMounts, vm)
		envvar := apiv1.EnvVar{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: mountPath,
		}
		envvars = append(envvars, envvar)
	}
	cmVolumeName := "shibuya-config"
	cmName := sc.ObjectStorage.ConfigMapName
	cmVolume := apiv1.Volume{
		Name: cmVolumeName,
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: cmName,
				},
			},
		},
	}
	volumes = append(volumes, cmVolume)
	cmVolumeMounts := apiv1.VolumeMount{
		Name:      cmVolumeName,
		MountPath: config.ConfigFilePath,
		SubPath:   config.ConfigFileName,
	}
	volumeMounts = append(volumeMounts, cmVolumeMounts)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       planName,
			DeletionGracePeriodSeconds: new(int64),
			Labels:                     labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:            int32Ptr(int32(replicas)),
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: apiv1.PodSpec{
					Affinity:                     affinity,
					Tolerations:                  tolerations,
					AutomountServiceAccountToken: &t,
					ImagePullSecrets: []apiv1.LocalObjectReference{
						{
							Name: sc.ExecutorConfig.ImagePullSecret,
						},
					},
					TerminationGracePeriodSeconds: new(int64),
					HostAliases:                   makeHostAliases(sc.ExecutorConfig.HostAliases),
					Volumes:                       volumes,
					Containers: []apiv1.Container{
						{
							Name:            planName,
							Image:           executorConfig.Image,
							ImagePullPolicy: sc.ExecutorConfig.ImagePullPolicy,
							Env:             envvars,
							Resources: apiv1.ResourceRequirements{
								Limits: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse(executorConfig.CPU),
									apiv1.ResourceMemory: resource.MustParse(executorConfig.Mem),
								},
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse(executorConfig.CPU),
									apiv1.ResourceMemory: resource.MustParse(executorConfig.Mem),
								},
							},
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 8080,
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
				},
			},
		},
	}
	return sts
}

func (plan planResource) makePlanService() *apiv1.Service {
	labels := plan.makePlanLabel()
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: plan.makeName(),
			Annotations: map[string]string{
				"networking.istio.io/exportTo": ".",
			},
			Labels: labels,
		},
		Spec: apiv1.ServiceSpec{
			Type:      apiv1.ServiceTypeClusterIP,
			ClusterIP: "None",
			Selector:  labels,
			Ports: []apiv1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
	return service
}

func (plan planResource) makeEngineName(engineID int) string {
	return fmt.Sprintf("engine-%d-%d-%d-%d", plan.projectID, plan.collectionID, plan.planID, engineID)
}
func (plan planResource) makeName() string {
	return fmt.Sprintf("engine-%d-%d-%d", plan.projectID, plan.collectionID, plan.planID)
}

func (plan planResource) makePlanLabel() map[string]string {
	return map[string]string{
		"collection": strconv.FormatInt(plan.collectionID, 10),
		"project":    strconv.FormatInt(plan.projectID, 10),
		"plan":       strconv.FormatInt(plan.planID, 10),
		"kind":       smodel.Executor,
	}
}

func (plan planResource) makeEngineMetaEnvvars() []apiv1.EnvVar {
	return []apiv1.EnvVar{
		{
			Name:  "collection_id",
			Value: fmt.Sprintf("%d", plan.collectionID),
		},
		{
			Name:  "plan_id",
			Value: fmt.Sprintf("%d", plan.planID),
		},
	}
}
