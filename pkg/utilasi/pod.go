package utilasi

import (
	"encoding/json"

	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	"github.com/openkruise/kruise/pkg/util"
	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

func SetUpdateSpecHash(pod *v1.Pod, hash string) {
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	if pod.Labels[apiinternal.LabelNodeType] != apiinternal.NodeTypeVirtualKubelet {
		pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = hash
		return
	}
	updateSpec := apiinternal.ContainerUpdateSpec{Version: hash}
	for _, c := range pod.Spec.Containers {
		updateSpec.Containers = append(updateSpec.Containers, apiinternal.ContainerVersion{Name: c.Name, Version: hash})
	}
	pod.Annotations[apiinternal.AnnotationUpdateSpec] = util.DumpJSON(updateSpec)
}

func getSigmaUpdateSpec(pod *v1.Pod) string {
	return pod.Annotations[sigmak8sapi.AnnotationPodSpecHash]
}

func getUnifiedUpdateSpec(pod *v1.Pod) *apiinternal.ContainerUpdateSpec {
	updateSpecStr := pod.Annotations[apiinternal.AnnotationUpdateSpec]
	if updateSpecStr == "" {
		return nil
	}
	updateSpec := apiinternal.ContainerUpdateSpec{}
	if err := json.Unmarshal([]byte(updateSpecStr), &updateSpec); err != nil {
		klog.Errorf(`Failed to unmarshal %s=%s in Pod %s/%s: %v`,
			apiinternal.AnnotationUpdateSpec, updateSpecStr, pod.Namespace, pod.Name, err)
		return nil
	}
	return &updateSpec
}

func getSigmaUpdateStatus(pod *v1.Pod) map[string]string {
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

func getUnifiedUpdateStatus(pod *v1.Pod) *apiinternal.ContainerUpdateStatus {
	updateStatusStr := pod.Annotations[apiinternal.AnnotationUpdateStatus]
	if updateStatusStr == "" {
		return nil
	}
	updateStatus := apiinternal.ContainerUpdateStatus{}
	err := json.Unmarshal([]byte(updateStatusStr), &updateStatus)
	if err != nil {
		klog.Errorf(`Failed to unmarshal %s=%s in Pod %s/%s: %v`,
			apiinternal.AnnotationUpdateStatus, updateStatusStr, pod.Namespace, pod.Name, err)
		return nil
	}
	return &updateStatus
}

func GetPodSpecHashString(pod *v1.Pod) string {
	if pod.Labels[apiinternal.LabelNodeType] != apiinternal.NodeTypeVirtualKubelet {
		return getSigmaUpdateSpec(pod)
	}
	updateSpec := getUnifiedUpdateSpec(pod)
	if updateSpec == nil {
		return ""
	}
	return updateSpec.Version
}

func IsPodSpecHashConsistent(pod *v1.Pod) bool {
	return IsPodSpecHashPartConsistent(pod, nil)
}

// 1. containers为空时，判断pod中的所有容器
// 2. containers包含值时，只判断containers
func IsPodSpecHashPartConsistent(pod *v1.Pod, containers sets.String) bool {
	if pod.Labels[apiinternal.LabelNodeType] != apiinternal.NodeTypeVirtualKubelet {
		return isSigmaUpdateConsistent(pod, containers)
	}
	return isUnifiedUpdateConsistent(pod, containers)
}

func isSigmaUpdateConsistent(pod *v1.Pod, containers sets.String) bool {
	podSpecHash := getSigmaUpdateSpec(pod)
	containerSpecHashes := getSigmaUpdateStatus(pod)
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

func isUnifiedUpdateConsistent(pod *v1.Pod, containers sets.String) bool {
	updateSpec := getUnifiedUpdateSpec(pod)
	updateStatus := getUnifiedUpdateStatus(pod)
	if updateSpec == nil && updateStatus == nil {
		return true
	} else if updateSpec == nil || updateStatus == nil {
		return false
	}
	if containers.Len() == 0 {
		return updateSpec.Version == updateStatus.Version
	}
	specContainerVersions := make(map[string]string, len(updateSpec.Containers))
	for _, c := range updateSpec.Containers {
		specContainerVersions[c.Name] = c.Version
	}
	var consistentCount int
	for _, c := range updateStatus.Containers {
		if !containers.Has(c.Name) {
			continue
		}
		if c.Version == specContainerVersions[c.Name] {
			consistentCount++
		}
	}
	return consistentCount == containers.Len()
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
