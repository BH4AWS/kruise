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

package sidecarcontrol

import (
	"testing"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	scheme *runtime.Scheme

	sidecarSetDemo1 = &appsv1alpha1.SidecarSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sidecarset",
			Labels: map[string]string{
				LabelSidecarSetMode: SidecarSetASI,
			},
		},
		Spec: appsv1alpha1.SidecarSetSpec{
			Containers: []appsv1alpha1.SidecarContainer{
				{
					Container: corev1.Container{
						Name: "test-sidecar1",
					},
				},
				{
					Container: corev1.Container{
						Name: "test-sidecar2",
					},
				},
			},
		},
	}

	podDemo1 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				SidecarSetHashAnnotation: `{"test-sidecarset":{"hash":"aaa","sidecarList":["test-sidecar1", "test-sidecar2"]}}`,
			},
			Name: "test-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "nginx",
				},
				{
					Name: "test-sidecar1",
				},
				{
					Name: "test-sidecar2",
				},
			},
		},
	}
)

func TestIsSidecarContainersChanged(t *testing.T) {
	cases := []struct {
		name          string
		getPod        func() *corev1.Pod
		getSidecarset func() *appsv1alpha1.SidecarSet
		except        bool
	}{
		{
			name: "not change v1",
			getPod: func() *corev1.Pod {
				return podDemo1.DeepCopy()
			},
			getSidecarset: func() *appsv1alpha1.SidecarSet {
				return sidecarSetDemo1.DeepCopy()
			},
			except: false,
		},
		{
			name: "add sidecar containers",
			getPod: func() *corev1.Pod {
				return podDemo1.DeepCopy()
			},
			getSidecarset: func() *appsv1alpha1.SidecarSet {
				demo := sidecarSetDemo1.DeepCopy()
				demo.Spec.Containers = append(demo.Spec.Containers, appsv1alpha1.SidecarContainer{Container: corev1.Container{Name: "test-sidecar3"}})
				return demo
			},
			except: true,
		},
		{
			name: "remove sidecar containers",
			getPod: func() *corev1.Pod {
				return podDemo1.DeepCopy()
			},
			getSidecarset: func() *appsv1alpha1.SidecarSet {
				demo := sidecarSetDemo1.DeepCopy()
				demo.Spec.Containers = demo.Spec.Containers[:1]
				return demo
			},
			except: true,
		},
		{
			name: "old pods don't have sidecarList filed",
			getPod: func() *corev1.Pod {
				pod := podDemo1.DeepCopy()
				pod.Annotations[SidecarSetHashAnnotation] = `{"test-sidecarset":{"hash":"aaa"}}`
				return pod
			},
			getSidecarset: func() *appsv1alpha1.SidecarSet {
				demo := sidecarSetDemo1.DeepCopy()
				return demo
			},
			except: false,
		},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			if cs.except != isSidecarContainersChanged(cs.getSidecarset(), cs.getPod()) {
				t.Fatalf("except(%v), but get(%v)", cs.except, isSidecarContainersChanged(cs.getSidecarset(), cs.getPod()))
			}
		})
	}
}
