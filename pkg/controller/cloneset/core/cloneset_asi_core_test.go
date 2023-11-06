package core

import (
	"reflect"
	"testing"

	"github.com/openkruise/kruise/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	podDemo = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod-1",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "test-image:v1",
				},
			},
		},
	}

	podTemplateDemo = &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod-1",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "test-image:v1",
					Env: []corev1.EnvVar{
						{
							Name:  "ali_run_mode",
							Value: "common_vm",
						},
					},
				},
			},
		},
	}
)

func TestRexp(t *testing.T) {
	successTests := []string{
		"/spec/containers/1/image",
		"/spec/containers/0/env/3/value",
		"/spec/containers/10/lifecycle/preStop",
		"/spec/containers/5/args/0",
	}
	failureTests := []string{
		"/spec/containers/1/name",
		"/spec/containers/0/volumeMounts/3",
		"/spec/initContainers/1/image",
		"/spec/volumes/3",
	}

	for _, s := range successTests {
		if !inPlaceUpdateTemplateSpecPatchASIRexp.MatchString(s) {
			t.Fatalf("expected %s success, but got not matched", s)
		}
	}
	for _, s := range failureTests {
		if inPlaceUpdateTemplateSpecPatchASIRexp.MatchString(s) {
			t.Fatalf("expected %s failure, but got matched", s)
		}
	}
}

func TestInjectAdditionalEnvsIntoPod(t *testing.T) {
	cases := []struct {
		name           string
		getPods        func() *corev1.Pod
		getPodTemplate func() *corev1.PodTemplateSpec
		getInjectEnvs  func() []corev1.EnvVar
		expectPod      func() *corev1.Pod
	}{
		{
			name: "common_vm, and inject",
			getPods: func() *corev1.Pod {
				obj := podDemo.DeepCopy()
				obj.Spec.Containers[0].Env = []corev1.EnvVar{
					{
						Name:  "ali_run_mode",
						Value: "common_vm",
					},
				}
				return obj
			},
			getPodTemplate: func() *corev1.PodTemplateSpec {
				return podTemplateDemo.DeepCopy()
			},
			getInjectEnvs: func() []corev1.EnvVar {
				return []corev1.EnvVar{
					{
						Name:  "ali_safety_out",
						Value: "1",
					},
				}
			},
			expectPod: func() *corev1.Pod {
				obj := podDemo.DeepCopy()
				obj.Spec.Containers[0].Env = []corev1.EnvVar{
					{
						Name:  "ali_safety_out",
						Value: "1",
					},
					{
						Name:  "ali_run_mode",
						Value: "common_vm",
					},
				}
				return obj
			},
		},
		{
			name: "light, and inject",
			getPods: func() *corev1.Pod {
				obj := podDemo.DeepCopy()
				obj.Spec.Containers[0].Env = []corev1.EnvVar{
					{
						Name:  "ali_run_mode",
						Value: "light",
					},
				}
				return obj
			},
			getPodTemplate: func() *corev1.PodTemplateSpec {
				return podTemplateDemo.DeepCopy()
			},
			getInjectEnvs: func() []corev1.EnvVar {
				return []corev1.EnvVar{
					{
						Name:  "ali_safety_out",
						Value: "1",
					},
				}
			},
			expectPod: func() *corev1.Pod {
				obj := podDemo.DeepCopy()
				obj.Spec.Containers[0].Env = []corev1.EnvVar{
					{
						Name:  "ali_safety_out",
						Value: "1",
					},
					{
						Name:  "ali_run_mode",
						Value: "light",
					},
				}
				return obj
			},
		},
		{
			name: "other, and not inject",
			getPods: func() *corev1.Pod {
				obj := podDemo.DeepCopy()
				obj.Spec.Containers[0].Env = []corev1.EnvVar{
					{
						Name:  "ali_run_mode",
						Value: "other",
					},
				}
				return obj
			},
			getPodTemplate: func() *corev1.PodTemplateSpec {
				return podTemplateDemo.DeepCopy()
			},
			getInjectEnvs: func() []corev1.EnvVar {
				return []corev1.EnvVar{
					{
						Name:  "ali_safety_out",
						Value: "1",
					},
				}
			},
			expectPod: func() *corev1.Pod {
				obj := podDemo.DeepCopy()
				obj.Spec.Containers[0].Env = []corev1.EnvVar{
					{
						Name:  "ali_run_mode",
						Value: "other",
					},
				}
				return obj
			},
		},
	}
	c := &asiControl{}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			pod := cs.getPods()
			c.injectAdditionalEnvsIntoPod(pod, cs.getInjectEnvs(), cs.getPodTemplate())
			if !reflect.DeepEqual(pod, cs.expectPod()) {
				t.Fatalf("expect(%s), but get(%s)", util.DumpJSON(cs.expectPod()), util.DumpJSON(pod))
			}
		})
	}
}
