package scheduler

import (
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeCertKeySecret(projectID int64, cert, key []byte) *apiv1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: makeIngressClass(projectID),
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": cert, // Certificate
			"tls.key": key,  // Private key
		},
	}
	return secret
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
