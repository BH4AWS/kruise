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
	"encoding/json"

	policyv1alpha1 "github.com/openkruise/kruise/apis/policy/v1alpha1"
	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	"github.com/openkruise/kruise/pkg/util/controllerfinder"
	"github.com/openkruise/kruise/pkg/utilasi"
	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	//快下Pods label
	PodNamingRegisterStateLabel = "pod.beta1.sigma.ali/naming-register-state"
	PodWaitOnlineValue          = "wait_online"

	// Debug request defined by users
	AnnotationPodDebug      = "pod.beta1.sigma.ali/debug"
	AnnotationVCPodOwnerRef = "tenancy.x-k8s.io/ownerReferences"
)

type asiControl struct {
	client.Client
	controllerFinder *controllerfinder.ControllerFinder
	commonControl    *commonControl
}

func newAsiControl(client client.Client, finder *controllerfinder.ControllerFinder) pubControl {
	control := &asiControl{controllerFinder: finder, Client: client}
	control.commonControl = &commonControl{controllerFinder: finder, Client: client}
	return control
}

func (c *asiControl) IsPodReady(pod *corev1.Pod) bool {
	//快下的pod，认为Not Ready，进而删除该pod webhook会直接放过
	if pod.Labels[PodNamingRegisterStateLabel] == PodWaitOnlineValue {
		return false
	}
	// debug pod
	if pod.Annotations[AnnotationPodDebug] != "" {
		return false
	}
	return c.commonControl.IsPodReady(pod)
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
	if utilasi.GetPodSpecHashString(oldPod) != utilasi.GetPodSpecHashString(newPod) {
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
func (c *asiControl) GetPodsForPub(pub *policyv1alpha1.PodUnavailableBudget) ([]*corev1.Pod, int32, error) {
	// if targetReference isn't nil, priority to take effect
	matchedPods, expectedCount, err := c.commonControl.GetPodsForPub(pub)
	if err != nil {
		return nil, 0, err
	}

	// 支持快上快下场景
	onlinePods := make([]*corev1.Pod, 0)
	var waitOnlineLength int32
	for i := range matchedPods {
		pod := matchedPods[i]
		if pod.Labels[PodNamingRegisterStateLabel] == PodWaitOnlineValue {
			waitOnlineLength++
			continue
		}
		onlinePods = append(onlinePods, pod)
	}
	expectedCount = expectedCount - waitOnlineLength
	return onlinePods, expectedCount, nil
}

func (c *asiControl) GetPubForPod(pod *corev1.Pod) (*policyv1alpha1.PodUnavailableBudget, error) {
	return c.commonControl.GetPubForPod(pod)
}

func (c *asiControl) GetPodControllerOf(pod *corev1.Pod) *metav1.OwnerReference {
	if ref := c.commonControl.GetPodControllerOf(pod); ref != nil {
		return ref
	}
	// vc/csk, tenancy.x-k8s.io/ownerReferences:
	// '[{"apiVersion":"apps/v1","kind":"ReplicaSet","name":"deployment-5bd47cb4b9","uid":"9792c697-ed69-4466-9c4d-f2a9f46e3d44","controller":true,"blockOwnerDeletion":true}]'
	if pod.Annotations == nil {
		return nil
	}
	refStr := pod.Annotations[AnnotationVCPodOwnerRef]
	if refStr == "" {
		return nil
	}
	var refs []metav1.OwnerReference
	if err := json.Unmarshal([]byte(refStr), &refs); err != nil {
		klog.Errorf("pub control Unmarshal pod(%s/%s) annotations(%s) body(%s) failed: %s", pod.Namespace, pod.Name, AnnotationVCPodOwnerRef, refStr, err.Error())
		return nil
	}
	for i := range refs {
		if refs[i].Controller != nil && *refs[i].Controller {
			return &refs[i]
		}
	}
	return nil
}
