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

package mutating

import (
	"context"
	"github.com/openkruise/kruise/pkg/util/containerexitpriority"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"path"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"testing"
)

var (
	exitPriorityDemo = corev1.Pod{
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

func TestContainerExitPriorityMutatingPod(t *testing.T) {
	cases := []struct {
		name      string
		getPod    func() *corev1.Pod
		expectPod func() *corev1.Pod
	}{
		{
			name: "no exit priority",
			getPod: func() *corev1.Pod {
				return exitPriorityDemo.DeepCopy()
			},
			expectPod: func() *corev1.Pod {
				return exitPriorityDemo.DeepCopy()
			},
		},
		{
			name: "container-exit-priority",
			getPod: func() *corev1.Pod {
				demo := exitPriorityDemo.DeepCopy()
				demo.Annotations[containerexitpriority.AutoInjectContainerExitPriorityPreStopAnnotation] = "true"
				demo.Annotations[containerexitpriority.PodContainerExitPriorityAnnotation] = `{"log": 2, "envoy": 1}`
				demo.Spec.Containers[2].Lifecycle = &corev1.Lifecycle{
					PreStop: &corev1.Handler{
						Exec: &corev1.ExecAction{
							Command: []string{
								"/bin/bash",
								"-c",
								"/usr/local/bin/pilot-agent exit",
							},
						},
					},
				}
				return demo
			},
			expectPod: func() *corev1.Pod {
				expect := exitPriorityDemo.DeepCopy()
				expect.Annotations[containerexitpriority.AutoInjectContainerExitPriorityPreStopAnnotation] = "true"
				expect.Annotations[containerexitpriority.PodContainerExitPriorityAnnotation] = `{"log": 2, "envoy": 1}`
				hostPathType := corev1.HostPathDirectoryOrCreate
				expect.Spec.Volumes = []corev1.Volume{
					{
						Name: "kruise-container-exit-info",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: path.Join(containerexitpriority.KruiseDaemonShareRootVolume, "pods"),
								Type: &hostPathType,
							},
						},
					},
				}
				expect.Spec.Containers[1].Lifecycle = &corev1.Lifecycle{
					PreStop: &corev1.Handler{
						Exec: &corev1.ExecAction{
							Command: []string{
								path.Join(containerexitpriority.KruiseDaemonShareRootVolume, "entrypoint"),
								"-wait-container-exit",
							},
						},
					},
				}
				expect.Spec.Containers[1].VolumeMounts = []corev1.VolumeMount{
					{
						Name:        "kruise-container-exit-info",
						MountPath:   containerexitpriority.KruiseDaemonShareRootVolume,
						SubPathExpr: path.Join("$(POD_UID)", "log"),
					},
				}
				expect.Spec.Containers[1].Env = []corev1.EnvVar{
					{
						Name: "POD_UID",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.uid",
							},
						},
					},
				}
				expect.Spec.Containers[2].Lifecycle = &corev1.Lifecycle{
					PreStop: &corev1.Handler{
						Exec: &corev1.ExecAction{
							Command: []string{
								path.Join(containerexitpriority.KruiseDaemonShareRootVolume, "entrypoint"),
								"-wait-container-exit",
								"-entrypoint",
								"/bin/bash",
								"--",
								"-c",
								"/usr/local/bin/pilot-agent exit",
							},
						},
					},
				}
				expect.Spec.Containers[2].VolumeMounts = []corev1.VolumeMount{
					{
						Name:        "kruise-container-exit-info",
						MountPath:   containerexitpriority.KruiseDaemonShareRootVolume,
						SubPathExpr: path.Join("$(POD_UID)", "envoy"),
					},
				}
				expect.Spec.Containers[2].Env = []corev1.EnvVar{
					{
						Name: "POD_UID",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.uid",
							},
						},
					},
				}
				return expect
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			podIn := cs.getPod()
			decoder, _ := admission.NewDecoder(scheme.Scheme)
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			podHandler := &PodCreateHandler{Decoder: decoder, Client: client}
			req := newAdmission(admissionv1.Create, runtime.RawExtension{}, runtime.RawExtension{}, "")
			_, err := podHandler.containerExitPriorityMutatingPod(context.Background(), req, podIn)
			if err != nil {
				t.Fatalf("inject sidecar into pod failed, err: %v", err)
			}
			if !reflect.DeepEqual(cs.expectPod(), podIn) {
				t.Fatalf("pod DeepEqual failed")
			}
		})
	}
}
