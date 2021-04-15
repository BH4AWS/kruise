package utilasi

import (
	"encoding/json"
	"k8s.io/apimachinery/pkg/util/sets"

	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	v1 "k8s.io/api/core/v1"
)

func GetPodSpecHash(pod *v1.Pod) string {
	return pod.Annotations[sigmak8sapi.AnnotationPodSpecHash]
}

func GetPodUpdatedSpecHashes(pod *v1.Pod) map[string]string {
	updateStatusStr, ok := pod.Annotations[sigmak8sapi.AnnotationPodUpdateStatus]
	if !ok {
		return nil
	}
	updateStatus := sigmak8sapi.ContainerStateStatus{}
	err := json.Unmarshal([]byte(updateStatusStr), &updateStatus)
	if err != nil {
		return nil
	}

	specHashes := make(map[string]string, len(updateStatus.Statuses))
	for info, containerStatus := range updateStatus.Statuses {
		specHashes[info.Name] = containerStatus.SpecHash
	}
	return specHashes
}

func IsPodSpecHashConsistent(pod *v1.Pod) bool {
	return IsPodSpecHashPartConsistent(pod, nil)
}

// 1. containers为空时，判断pod中的所有容器
// 2. containers包含值时，只判断containers
func IsPodSpecHashPartConsistent(pod *v1.Pod, containers sets.String) bool {
	podSpecHash := GetPodSpecHash(pod)
	containerSpecHashes := GetPodUpdatedSpecHashes(pod)
	for _, c := range pod.Spec.Containers {
		if containers.Len() > 0 && !containers.Has(c.Name) {
			continue
		}
		if containerSpecHashes[c.Name] != podSpecHash {
			return false
		}
	}
	return true
}

func ContainsInContainerStateSpec(containerStateSpec *sigmak8sapi.ContainerStateSpec, desired sigmak8sapi.ContainerState) bool {
	if containerStateSpec == nil {
		return false
	}
	for _, state := range containerStateSpec.States {
		if state == desired {
			return true
		}
	}
	return false
}

func IsContainerStateSpecAllRunning(containerStateSpec *sigmak8sapi.ContainerStateSpec) bool {
	if containerStateSpec == nil {
		return true
	}
	for _, state := range containerStateSpec.States {
		if state != sigmak8sapi.ContainerStateRunning {
			return false
		}
	}
	return true
}

func IsPodInplaceUpgrading(pod, oldPod *v1.Pod) bool {
	return pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] != oldPod.Annotations[sigmak8sapi.AnnotationPodSpecHash]
}
