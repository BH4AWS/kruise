/*
Copyright 2021 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sidecarset

import (
	"encoding/json"
	"testing"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/control/sidecarcontrol"
	"github.com/openkruise/kruise/pkg/utilasi"

	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	podHotUpgrade.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "v82f9d8f5444472w4wffxw5w6b642v6wxb48z74425564cc4cc7w5fdzc4z44cx4"
	podHotUpgrade.Annotations[sigmak8sapi.AnnotationPodUpdateStatus] = `{"statuses":{"test-sidecar-1":{"specHash":"v82f9d8f5444472w4wffxw5w6b642v6wxb48z74425564cc4cc7w5fdzc4z44cx4"},"test-sidecar-2":{"specHash":"v82f9d8f5444472w4wffxw5w6b642v6wxb48z74425564cc4cc7w5fdzc4z44cx4"},"nginx":{"specHash":"v82f9d8f5444472w4wffxw5w6b642v6wxb48z74425564cc4cc7w5fdzc4z44cx4"}}}`
}

func setSidecarSetASI(sidecarset *appsv1alpha1.SidecarSet) {
	sidecarset.Labels[sidecarcontrol.LabelSidecarSetMode] = sidecarcontrol.SidecarSetASI
	for i := range sidecarset.Spec.Containers {
		container := &sidecarset.Spec.Containers[i]
		container.UpgradeStrategy.HotUpgradeEmptyImage = hotUpgradeEmptyImage
	}
}

func setPodUpdateStatus(pod *corev1.Pod) {
	podHash := utilasi.GetPodSpecHashString(pod)
	updateStatus := sigmak8sapi.ContainerStateStatus{
		Statuses: map[sigmak8sapi.ContainerInfo]sigmak8sapi.ContainerStatus{},
	}
	imageInPodSpec := map[string]string{}
	for _, container := range pod.Spec.Containers {
		info := sigmak8sapi.ContainerInfo{
			Name: container.Name,
		}
		updateStatus.Statuses[info] = sigmak8sapi.ContainerStatus{
			SpecHash: podHash,
		}
		imageInPodSpec[container.Name] = container.Image
	}
	by, _ := json.Marshal(updateStatus)
	pod.Annotations[sigmak8sapi.AnnotationPodUpdateStatus] = string(by)

	for index, _ := range pod.Status.ContainerStatuses {
		cStatus := &pod.Status.ContainerStatuses[index]
		cStatus.Image = imageInPodSpec[cStatus.Name]
	}
}

func TestUpdateWhenUseNotUpdateStrategyASI(t *testing.T) {
	sidecarSetInput := sidecarSetDemo.DeepCopy()
	setSidecarSetASI(sidecarSetInput)
	testUpdateWhenUseNotUpdateStrategy(t, sidecarSetInput)
}

func TestUpdateWhenSidecarSetPausedASI(t *testing.T) {
	sidecarSetInput := sidecarSetDemo.DeepCopy()
	setSidecarSetASI(sidecarSetInput)
	testUpdateWhenSidecarSetPaused(t, sidecarSetInput)
}

func TestUpdateWhenMaxUnavailableNotZeroASI(t *testing.T) {
	sidecarSetInput := sidecarSetDemo.DeepCopy()
	setSidecarSetASI(sidecarSetInput)
	testUpdateWhenMaxUnavailableNotZero(t, sidecarSetInput)
}

func TestUpdateWhenPartitionFinishedASI(t *testing.T) {
	sidecarSetInput := sidecarSetDemo.DeepCopy()
	setSidecarSetASI(sidecarSetInput)
	testUpdateWhenPartitionFinished(t, sidecarSetInput)
}

func TestRemoveSidecarSetASI(t *testing.T) {
	sidecarSetInput := sidecarSetDemo.DeepCopy()
	setSidecarSetASI(sidecarSetInput)
	testRemoveSidecarSet(t, sidecarSetInput)
}

func TestUpdateColdUpgradeSidecarASI(t *testing.T) {
	sidecarSetInput := sidecarSetDemo.DeepCopy()
	sidecarSetInput.Spec.Containers[0].ShareVolumePolicy.Type = appsv1alpha1.ShareVolumePolicyEnabled
	sidecarSetInput.Spec.Containers[0].TransferEnv = []appsv1alpha1.TransferEnvVar{
		{
			SourceContainerName: "nginx",
			EnvName:             "nginx-env",
		},
	}
	setSidecarSetASI(sidecarSetInput)
	handlers := map[string]HandlePod{
		"pod test-pod-1 is upgrading": func(pods []*corev1.Pod) {
			setPodUpdateStatus(pods[0])
		},
		"pod test-pod-2 is upgrading": func(pods []*corev1.Pod) {
			setPodUpdateStatus(pods[1])
		},
	}
	testUpdateColdUpgradeSidecar(t, podDemo.DeepCopy(), sidecarSetInput, handlers)
}

func TestUpdateHotUpgradeSidecarASI(t *testing.T) {
	sidecarSetInput := sidecarSetHotUpgrade.DeepCopy()
	setSidecarSetASI(sidecarSetInput)
	handlers := map[string]HandlePod{
		"test-sidecar-2 container is upgrading": func(pods []*corev1.Pod) {
			setPodUpdateStatus(pods[0])
		},
		"test-sidecar-2 container upgrade complete, and reset test-sidecar-1 empty image": func(pods []*corev1.Pod) {
			setPodUpdateStatus(pods[0])
		},
	}
	testUpdateHotUpgradeSidecar(t, hotUpgradeEmptyImage, sidecarSetInput, handlers)
}

func factoryPodsASI(count, upgraded, upgradedAndReady int) []*corev1.Pod {
	sidecarSet := factorySidecarSetASI()
	pods := factoryPodsCommon(count, upgraded, sidecarSet)
	for i := 0; i < upgradedAndReady; i++ {
		setPodUpdateStatus(pods[i])
	}

	return pods
}

func factorySidecarSetASI() *appsv1alpha1.SidecarSet {
	sidecarSet := factorySidecarSet()
	setSidecarSetASI(sidecarSet)
	return sidecarSet
}

func TestGetNextUpgradePodsASI(t *testing.T) {
	testGetNextUpgradePods(t, factoryPodsASI, factorySidecarSetASI)
}

func TestSortNextUpgradePodsASI(t *testing.T) {
	factoryPodsASI := func(count, upgraded, upgradedAndReady int) []*corev1.Pod {
		sidecarSet := factorySidecarSetASI()
		pods := factoryPodsCommon(count, upgraded, sidecarSet)
		// 兼容场景："not ready priority, maxUnavailable(int=10) and pods(count=20, upgraded=10, upgradedAndReady=5)"
		pods[10].Status.ContainerStatuses[1].Ready = false
		pods[13].Status.ContainerStatuses[1].Ready = false
		for i := 0; i < upgradedAndReady; i++ {
			setPodUpdateStatus(pods[i])
		}

		return pods
	}

	testSortNextUpgradePods(t, factoryPodsASI, factorySidecarSetASI)
}
