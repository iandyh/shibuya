package scheduler

import (
	"fmt"

	"github.com/rakutentech/shibuya/shibuya/config"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func makeTolerations(key string, value string, effect apiv1.TaintEffect) apiv1.Toleration {
	toleration := apiv1.Toleration{
		Effect:   effect,
		Key:      key,
		Operator: apiv1.TolerationOpEqual,
		Value:    value,
	}
	return toleration
}

func collectionNodeAffinity(collectionID int64) *apiv1.NodeAffinity {
	collectionIDStr := fmt.Sprintf("%d", collectionID)
	return makeNodeAffinity("collection_id", collectionIDStr)
}

func makePodAffinity(key, value string) *apiv1.PodAffinity {
	podAffinity := &apiv1.PodAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []apiv1.WeightedPodAffinityTerm{
			{
				Weight: 100,
				PodAffinityTerm: apiv1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							key: value,
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
	return podAffinity
}

func collectionPodAffinity(collectionID int64) *apiv1.PodAffinity {
	collectionIDStr := fmt.Sprintf("%d", collectionID)
	return makePodAffinity("collection", collectionIDStr)
}

func prepareAffinity(collectionID int64, nodeAffinity []map[string]string) *apiv1.Affinity {
	affinity := &apiv1.Affinity{}
	affinity.PodAffinity = collectionPodAffinity(collectionID)
	if len(nodeAffinity) > 0 {
		t := nodeAffinity[0]
		affinity.NodeAffinity = makeNodeAffinity(t["key"], t["value"])
		return affinity
	}
	return affinity
}

func prepareTolerations(stolerations []config.Toleration) []apiv1.Toleration {
	tolerations := []apiv1.Toleration{}

	if len(stolerations) > 0 {
		for _, t := range stolerations {
			tolerations = append(tolerations, makeTolerations(t.Key, t.Value, t.Effect))
		}
	}
	return tolerations
}

func makeHostAliases(preConfighostAliases []*config.HostAlias) []apiv1.HostAlias {
	if preConfighostAliases != nil {
		hostAliases := []apiv1.HostAlias{}
		for _, ha := range preConfighostAliases {
			hostAliases = append(hostAliases, apiv1.HostAlias{
				Hostnames: []string{ha.Hostname},
				IP:        ha.IP,
			})
		}
		return hostAliases
	}
	return []apiv1.HostAlias{}
}

func makeCollectionLabel(collectionID int64) string {
	return fmt.Sprintf("collection=%d", collectionID)
}
