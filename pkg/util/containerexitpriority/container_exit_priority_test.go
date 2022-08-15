/*
Copyright 2021.

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
	"reflect"
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	podDemo = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod",
			Annotations: map[string]string{},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "nginx:1.15.1",
				},
				{
					Name:  "log",
					Image: "logtail:1.0.0",
				},
				{
					Name:  "envoy",
					Image: "envoy:1.0.0",
				},
			},
		},
	}
)

func TestListExitPriorityContainers(t *testing.T) {
	cases := []struct {
		name   string
		getPod func() *corev1.Pod
		expect func() []ExitPriorityContainer
	}{
		{
			name: `annotations, {"log":2, "envoy":1}`,
			getPod: func() *corev1.Pod {
				demo := podDemo.DeepCopy()
				demo.Annotations[PodContainerExitPriorityAnnotation] = `{"log":2, "envoy":1}`
				return demo
			},
			expect: func() []ExitPriorityContainer {
				return []ExitPriorityContainer{
					{
						Name:           "log",
						Priority:       2,
						WaitContainers: []string{"main", "envoy"},
					},
					{
						Name:           "envoy",
						Priority:       1,
						WaitContainers: []string{"main"},
					},
				}
			},
		},
		{
			name: `invalid priority annotations, {"log":2000, "envoy":10}`,
			getPod: func() *corev1.Pod {
				demo := podDemo.DeepCopy()
				demo.Annotations[PodContainerExitPriorityAnnotation] = `{"log":2000, "envoy":10}`
				return demo
			},
			expect: func() []ExitPriorityContainer {
				return []ExitPriorityContainer{
					{
						Name:           "envoy",
						Priority:       10,
						WaitContainers: []string{"main", "log"},
					},
				}
			},
		},
		{
			name: `invalid name annotations, {"fail":2, "envoy":1}`,
			getPod: func() *corev1.Pod {
				demo := podDemo.DeepCopy()
				demo.Annotations[PodContainerExitPriorityAnnotation] = `{"fail":2, "envoy":1}`
				return demo
			},
			expect: func() []ExitPriorityContainer {
				return []ExitPriorityContainer{
					{
						Name:           "envoy",
						Priority:       1,
						WaitContainers: []string{"main", "log"},
					},
				}
			},
		},
		{
			name: "env, log=2,envoy=1",
			getPod: func() *corev1.Pod {
				demo := podDemo.DeepCopy()
				demo.Spec.Containers[1].Env = []corev1.EnvVar{
					{
						Name:  "other",
						Value: "test",
					},
					{
						Name:  PodContainerExitPriorityEnv,
						Value: "2",
					},
				}
				demo.Spec.Containers[2].Env = []corev1.EnvVar{
					{
						Name:  PodContainerExitPriorityEnv,
						Value: "1",
					},
				}
				return demo
			},
			expect: func() []ExitPriorityContainer {
				return []ExitPriorityContainer{
					{
						Name:           "log",
						Priority:       2,
						WaitContainers: []string{"main", "envoy"},
					},
					{
						Name:           "envoy",
						Priority:       1,
						WaitContainers: []string{"main"},
					},
				}
			},
		},
		{
			name: `envs and annotations, {"log":2, "envoy":1}, log=3`,
			getPod: func() *corev1.Pod {
				demo := podDemo.DeepCopy()
				demo.Annotations[PodContainerExitPriorityAnnotation] = `{"log":2, "envoy":1}`
				demo.Spec.Containers[1].Env = []corev1.EnvVar{
					{
						Name:  "other",
						Value: "test",
					},
					{
						Name:  PodContainerExitPriorityEnv,
						Value: "3",
					},
				}
				return demo
			},
			expect: func() []ExitPriorityContainer {
				return []ExitPriorityContainer{
					{
						Name:           "log",
						Priority:       3,
						WaitContainers: []string{"main", "envoy"},
					},
					{
						Name:           "envoy",
						Priority:       1,
						WaitContainers: []string{"main"},
					},
				}
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			podIn := cs.getPod()
			containers := ListExitPriorityContainers(podIn)
			if !deepEqual(cs.expect(), containers) {
				t.Fatalf("ExitPriorityContainers DeepEqual failed")
			}
		})
	}
}

type ArrayStr []string

func (c ArrayStr) Len() int {
	return len(c)
}
func (c ArrayStr) Less(i, j int) bool {
	return c[i] < c[j]
}
func (c ArrayStr) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func deepEqual(c1, c2 []ExitPriorityContainer) bool {
	for i := range c1 {
		c := &c1[i]
		sort.Sort(ArrayStr(c.WaitContainers))
	}
	for i := range c2 {
		c := &c2[i]
		sort.Sort(ArrayStr(c.WaitContainers))
	}
	return reflect.DeepEqual(c1, c2)
}
