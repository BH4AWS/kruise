/*
Copyright 2023 The Kruise Authors.

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetPodControllerOf(t *testing.T) {
	cases := []struct {
		name   string
		getPod func() *corev1.Pod
		expect metav1.OwnerReference
	}{
		{
			name: "PodControllerOf test1",
			getPod: func() *corev1.Pod {
				demo := podDemo.DeepCopy()
				demo.OwnerReferences = nil
				demo.Annotations[AnnotationVCPodOwnerRef] = `[{"apiVersion":"apps/v1","kind":"ReplicaSet","name":"data-service-proxy-deployment-5bd47cb4b9","uid":"9792c697-ed69-4466-9c4d-f2a9f46e3d44","controller":true,"blockOwnerDeletion":true}]`
				return demo
			},
			expect: metav1.OwnerReference{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "data-service-proxy-deployment-5bd47cb4b9",
				UID:        types.UID("9792c697-ed69-4466-9c4d-f2a9f46e3d44"),
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			control := asiControl{}
			ref := control.GetPodControllerOf(cs.getPod())
			if ref == nil {
				t.Fatalf("GetPodControllerOf failed")
			}
			if cs.expect.APIVersion != ref.APIVersion ||
				cs.expect.Kind != ref.Kind ||
				cs.expect.Name != ref.Name ||
				cs.expect.UID != ref.UID {
				t.Fatalf("GetPodControllerOf failed")
			}
		})
	}
}
