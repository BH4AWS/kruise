/*
Copyright 2022.

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

package containerexitpriority

import (
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"strconv"
)

const (
	// PodContainerExitPriorityAnnotation for example: kruise.io/container-exit-priority: '{"log":2, "envoy":1}'
	PodContainerExitPriorityAnnotation = "kruise.io/container-exit-priority"
	// PodContainerExitPriorityEnv value uint8[0，255]
	PodContainerExitPriorityEnv = "KRUISE_EXIT_PRIORITY"
	// KruiseDaemonShareRootVolume is shared with hostPath root directory, the complete directory is /var/lib/kruise-daemon/pods/{pod.uid}/{container.name}
	KruiseDaemonShareRootVolume = "/var/lib/kruise-daemon"
	// KruiseContainerExitPriorityFile exit file
	KruiseContainerExitPriorityFile = "exit"
	// KruiseContainerAllowExitFlag exit content is 'allow'
	KruiseContainerAllowExitFlag = "allow"
	// AutoInjectContainerExitPriorityPreStopAnnotation indicates kruise will auto inject container exit priority preStop
	AutoInjectContainerExitPriorityPreStopAnnotation = "container-exit-priority.kruise.io/auto-inject-prestop"
)

type ExitPriorityContainer struct {
	// container name
	Name string
	// priority
	Priority uint8
	// wait exit container list
	WaitContainers []string
}

// ListExitPriorityContainers return exit priority containers, sort by exit priority from low to high
func ListExitPriorityContainers(pod *corev1.Pod) []ExitPriorityContainer {
	allContainers := map[string]uint8{}
	for _, o := range pod.Spec.Containers {
		allContainers[o.Name] = 0
	}
	if val := pod.Annotations[PodContainerExitPriorityAnnotation]; val != "" {
		// {"log":2, "envoy":1}
		exitPriority := map[string]uint8{}
		err := json.Unmarshal([]byte(val), &exitPriority)
		if err != nil {
			klog.Warningf("unmarshal pod(%s/%s) annotation[%s] failed: %s", pod.Namespace, pod.Name, PodContainerExitPriorityAnnotation, err.Error())
		}
		for name, priority := range exitPriority {
			// invalid container name
			if _, ok := allContainers[name]; ok {
				allContainers[name] = priority
			}
		}
	}

	containers := make([]ExitPriorityContainer, 0)
	for _, container := range pod.Spec.Containers {
		for _, env := range container.Env {
			if env.Name != PodContainerExitPriorityEnv {
				continue
			}
			priority, err := strconv.ParseUint(env.Value, 10, 8)
			if err != nil {
				continue
			}
			allContainers[container.Name] = uint8(priority)
		}

		if allContainers[container.Name] != 0 {
			containers = append(containers, ExitPriorityContainer{Name: container.Name, Priority: allContainers[container.Name]})
		}
	}

	for i := range containers {
		container := &containers[i]
		for name, priority := range allContainers {
			if name != container.Name && priority < container.Priority {
				container.WaitContainers = append(container.WaitContainers, name)
			}
		}
	}
	return containers
}
