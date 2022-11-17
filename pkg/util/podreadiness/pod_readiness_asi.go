/*
Copyright 2022 The Kruise Authors.

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

package podreadiness

import (
	"encoding/json"
	"fmt"

	appspub "github.com/openkruise/kruise/apis/apps/pub"
	"github.com/openkruise/kruise/pkg/util/podadapter"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type asiControl struct {
	adp podadapter.Adapter
}

// ContainsReadinessGate always return true, because if the Pod doesn't
// have KruisePodReady gate, asiControl will try to inject one to it.
func (c *asiControl) ContainsReadinessGate(pod *v1.Pod) bool {
	return true
}

func (c *asiControl) AddNotReadyKey(pod *v1.Pod, msg Message) error {
	newPod, err := c.addReadinessGate(pod, appspub.KruisePodReadyConditionType)
	if err != nil || newPod == nil {
		klog.Errorf("Failed to patch readinessGate for pod %v, err: %v", client.ObjectKeyFromObject(pod), err)
		return err
	}

	err = addNotReadyKey(c.adp, newPod, msg, appspub.KruisePodReadyConditionType)
	return err
}

func (c *asiControl) RemoveNotReadyKey(pod *v1.Pod, msg Message) error {
	return removeNotReadyKey(c.adp, pod, msg, appspub.KruisePodReadyConditionType)
}

func (c *asiControl) addReadinessGate(pod *v1.Pod, conditionType v1.PodConditionType) (*v1.Pod, error) {
	if containsReadinessGate(pod, conditionType) {
		return pod, nil
	}
	newGate := v1.PodReadinessGate{ConditionType: conditionType}
	if adp, ok := c.adp.(podadapter.AdapterWithPatch); ok {
		newPod := pod.DeepCopy()
		readinessGates := append(pod.Spec.ReadinessGates, newGate)
		readinessGatesByte, _ := json.Marshal(readinessGates)
		readinessGatesData := []byte(fmt.Sprintf(`{"spec":{"readinessGates":%v}}`, string(readinessGatesByte)))
		patchBody := client.RawPatch(types.StrategicMergePatchType, readinessGatesData)
		return adp.PatchPod(newPod, patchBody)
	}
	var newPod *v1.Pod
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		podClone, err := c.adp.GetPod(pod.Namespace, pod.Name)
		if err != nil {
			return err
		}
		if containsReadinessGate(podClone, conditionType) {
			return nil
		}
		podClone.Spec.ReadinessGates = append(podClone.Spec.ReadinessGates, newGate)
		newPod, err = c.adp.UpdatePod(podClone)
		return err
	})
	return newPod, err
}
