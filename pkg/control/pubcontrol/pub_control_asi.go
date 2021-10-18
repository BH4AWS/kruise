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

package pubcontrol

import (
	"context"
	"encoding/json"

	policyv1alpha1 "github.com/openkruise/kruise/apis/policy/v1alpha1"
	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	"github.com/openkruise/kruise/pkg/util"
	"github.com/openkruise/kruise/pkg/util/controllerfinder"
	"github.com/openkruise/kruise/pkg/utilasi"
	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LabelPubMode = "pub.kruise.io/mode"
	PubASI       = "asi"

	//快下Pods label
	PodNamingRegisterStateLabel = "pod.beta1.sigma.ali/naming-register-state"
	PodWaitOnlineValue = "wait_online"
)

type asiControl struct {
	client.Client
	*policyv1alpha1.PodUnavailableBudget
	controllerFinder *controllerfinder.ControllerFinder
}

func (c *asiControl) GetPodUnavailableBudget() *policyv1alpha1.PodUnavailableBudget {
	return c.PodUnavailableBudget
}

func (c *asiControl) IsPodReady(pod *corev1.Pod) bool {
	//快下的pod，认为Not Ready，进而删除该pod webhook会直接放过
	if pod.Labels[PodNamingRegisterStateLabel]==PodWaitOnlineValue {
		return false
	}

	// 1. pod.Status.Phase == v1.PodRunning
	// 2. pod.condition PodReady == true
	return util.IsRunningAndReady(pod)
}

func (c *asiControl) IsPodStateConsistent(pod *corev1.Pod) bool {
	// 检查所有容器对应的spec-hash都符合期望
	if !utilasi.IsPodSpecHashConsistent(pod) {
		return false
	}

	// 检查containers状态是否符合期望
	var containerStateSpec *sigmak8sapi.ContainerStateSpec
	if v, ok := pod.Annotations[sigmak8sapi.AnnotationContainerStateSpec]; ok {
		_ = json.Unmarshal([]byte(v), &containerStateSpec)
	}

	// 如果有container期望状态是paused，则算作还在发布中
	if utilasi.ContainsInContainerStateSpec(containerStateSpec, sigmak8sapi.ContainerStatePaused) {
		return false
	}

	// 如果在debug模式，则算作在发布中
	if pod.Annotations[apiinternal.AnnotationPodDebugContext] != "" {
		return false
	}

	return true
}

// 只有update event才会生效
func (c *asiControl) IsPodUnavailableChanged(oldPod, newPod *corev1.Pod) bool {
	// kruise workload in-place situation
	if newPod == nil || oldPod == nil {
		return true
	}
	// If pod.spec changed, pod will be in unavailable condition
	if utilasi.GetPodSpecHash(oldPod) != utilasi.GetPodSpecHash(newPod) {
		klog.V(3).Infof("pod(%s.%s) spec hash changed, and maybe cause unavailability", newPod.Namespace, newPod.Name)
		return true
	}
	// pod other changes will not cause unavailability situation, then return false
	return false
}

// GetPodsForPub returns Pods protected by the pub object.
// return two parameters
// 1. podList
// 2. expectedCount, the default is workload.Replicas
func (c *asiControl) GetPodsForPub() ([]*corev1.Pod, int32, error) {
	pub := c.GetPodUnavailableBudget()
	// if targetReference isn't nil, priority to take effect
	var listOptions *client.ListOptions
	var expectedCount int32
	var err error
	matchedPods := make([]*corev1.Pod, 0)
	if pub.Spec.TargetReference != nil {
		ref := pub.Spec.TargetReference
		matchedPods, expectedCount, err = c.controllerFinder.GetPodsForRef(ref.APIVersion, ref.Kind, ref.Name, pub.Namespace, true)
		if err!=nil {
			return nil, 0, err
		}
	} else {
		// get pods for selector
		labelSelector, err := util.GetFastLabelSelector(pub.Spec.Selector)
		if err != nil {
			klog.Warningf("pub(%s/%s) GetFastLabelSelector failed: %s", pub.Namespace, pub.Name, err.Error())
			return nil, 0, nil
		}
		listOptions = &client.ListOptions{Namespace: pub.Namespace, LabelSelector: labelSelector, DisableDeepCopy: true}
		podList := &corev1.PodList{}
		if err = c.List(context.TODO(), podList, listOptions); err != nil {
			return nil, 0, err
		}

		for i := range podList.Items {
			pod := &podList.Items[i]
			if kubecontroller.IsPodActive(pod) {
				matchedPods = append(matchedPods, pod)
			}
		}
		expectedCount, err = c.controllerFinder.GetExpectedScaleForPods(matchedPods)
		if err != nil {
			return nil, 0, err
		}
	}

	// 支持快上快下场景
	onlinePods := make([]*corev1.Pod, 0)
	var waitOnlineLength int32
	for i :=range matchedPods {
		pod := matchedPods[i]
		if pod.Labels[PodNamingRegisterStateLabel]==PodWaitOnlineValue {
			waitOnlineLength++
			continue
		}
		onlinePods = append(onlinePods, pod)
	}
	expectedCount = expectedCount - waitOnlineLength
	return onlinePods, expectedCount, nil
}
