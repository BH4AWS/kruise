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

package mutating

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/control/sidecarcontrol"
	"github.com/openkruise/kruise/pkg/util"

	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const EmptyImageForHotUpgrade = "reg.docker.alibaba-inc.com/sigma/empty:1.0"

func setSidecarSetASI(sidecarSet *appsv1alpha1.SidecarSet) {
	sidecarSet.Labels[sidecarcontrol.LabelSidecarSetMode] = sidecarcontrol.SidecarSetASI
}

func TestPodHasNoMatchedSidecarSetASI(t *testing.T) {
	sidecarSetIn := sidecarSet1.DeepCopy()
	setSidecarSetASI(sidecarSetIn)
	testPodHasNoMatchedSidecarSet(t, sidecarSetIn)
}

func TestSidecarSetPodInjectPolicyASI(t *testing.T) {
	sidecarSetIn := sidecarSet1.DeepCopy()
	setSidecarSetASI(sidecarSetIn)
	testSidecarSetPodInjectPolicy(t, sidecarSetIn)
}

func TestSidecarVolumesAppendASI(t *testing.T) {
	sidecarSetIn := sidecarsetWithTransferEnv.DeepCopy()
	setSidecarSetASI(sidecarSetIn)
	testSidecarVolumesAppend(t, sidecarSetIn)
}

func TestSidecarSetTransferEnvASI(t *testing.T) {
	sidecarSetIn := sidecarsetWithTransferEnv.DeepCopy()
	setSidecarSetASI(sidecarSetIn)
	testSidecarSetTransferEnv(t, sidecarSetIn)
}

func TestSidecarSetHashInjectASI(t *testing.T) {
	sidecarSetIn1 := sidecarSet1.DeepCopy()
	setSidecarSetASI(sidecarSetIn1)
	testSidecarSetHashInject(t, sidecarSetIn1)
}

func TestSidecarSetNameInjectASI(t *testing.T) {
	sidecarSetIn1 := sidecarSet1.DeepCopy()
	setSidecarSetASI(sidecarSetIn1)
	sidecarSetIn3 := sidecarSet3.DeepCopy()
	setSidecarSetASI(sidecarSetIn3)
	testSidecarSetNameInject(t, sidecarSetIn1, sidecarSetIn3)
}

func TestInjectHotUpgradeSidecarASI(t *testing.T) {
	sidecarSetIn := sidecarSet1.DeepCopy()
	setSidecarSetASI(sidecarSetIn)
	sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
	sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
	testInjectHotUpgradeSidecar(t, sidecarSetIn)
}

func TestNoUpdateSidecarDuringAppUpgrade(t *testing.T) {
	podInput := pod1.DeepCopy()
	podInput.Annotations = map[string]string{
		sigmak8sapi.AnnotationPodSpecHash: "pod-v1-aaa",
	}
	oldPodWithColdUpgradeTmpl := podInput.DeepCopy()
	oldPodWithColdUpgradeTmpl.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	oldPodWithColdUpgradeTmpl2 := podInput.DeepCopy()
	oldPodWithColdUpgradeTmpl2.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v3-ccc"

	// 老 pod 带有 dns-f sidecar
	injectedCtns := []v1.Container{v1.Container{
		Name:  "dns-f",
		Image: "dns-f-image:old",
		Env: []v1.EnvVar{
			{
				Name:  sidecarcontrol.SidecarEnvKey,
				Value: "true",
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "volume-a",
				MountPath: "/a/b",
			},
			{
				Name:      "volume-b",
				MountPath: "/e/f",
			},
		},
	}}

	oldPodWithColdUpgradeTmpl.Spec.Containers = append(injectedCtns, oldPodWithColdUpgradeTmpl.Spec.Containers...)
	oldPodWithColdUpgradeTmpl2.Spec.Containers = append(oldPodWithColdUpgradeTmpl2.Spec.Containers, injectedCtns...)

	oldPodWithHotUpgradeTmpl := podInput.DeepCopy()
	oldPodWithHotUpgradeTmpl.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	oldPodWithHotUpgradeTmpl2 := podInput.DeepCopy()
	oldPodWithHotUpgradeTmpl2.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"

	// 老 pod 带有 dns-f sidecar
	injectedCtns = []v1.Container{
		{
			Name:  "dns-f-1",
			Image: "dns-f-image:old",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "volume-a",
					MountPath: "/a/b",
				},
				{
					Name:      "volume-b",
					MountPath: "/e/f",
				},
			},
		},
		{
			Name:  "dns-f-2",
			Image: "dns-f-image:old2",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "volume-a",
					MountPath: "/a/b",
				},
				{
					Name:      "volume-b",
					MountPath: "/e/f",
				},
			},
		},
	}

	oldPodWithHotUpgradeTmpl.Spec.Containers = append(injectedCtns, oldPodWithHotUpgradeTmpl.Spec.Containers...)
	oldPodWithHotUpgradeTmpl2.Spec.Containers = append(oldPodWithHotUpgradeTmpl2.Spec.Containers, injectedCtns...)
	// 标记 内部ASI SidecarSet
	sidecarSetInput := sidecarSet1.DeepCopy()
	sidecarSetInput.Labels[sidecarcontrol.LabelSidecarSetMode] = sidecarcontrol.SidecarSetASI
	cases := []struct {
		name                   string
		getOldPod              func() *v1.Pod
		getNewPod              func(oldPod *v1.Pod) *v1.Pod
		getSidecarSet          func() *appsv1alpha1.SidecarSet
		expectContainerLen     int
		expectedSidecarCtnIdx  int
		expectedSidecarCtnName string
	}{
		{
			name: "inplaceset upgrade + cold upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "inplaceset upgrade + cold upgrade strategy + inject after app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "cloneset upgrade + cold upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "cloneset upgrade + cold upgrade strategy + inject after app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "inplaceset upgrade + hot upgrade strategy +inject before app",
			getOldPod: func() *v1.Pod {
				return oldPodWithHotUpgradeTmpl.DeepCopy()
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "inplaceset upgrade + hot upgrade strategy +inject after app",
			getOldPod: func() *v1.Pod {
				return oldPodWithHotUpgradeTmpl2.DeepCopy()
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "cloneset upgrade + hot upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				return oldPodWithHotUpgradeTmpl.DeepCopy()
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "cloneset upgrade + hot upgrade strategy + inject after app",
			getOldPod: func() *v1.Pod {
				return oldPodWithHotUpgradeTmpl2.DeepCopy()
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f-1",
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			oldPod := cs.getOldPod()
			newPod := cs.getNewPod(oldPod)
			sidecarSet := cs.getSidecarSet()
			sidecarSet.Spec.Containers[0].ShareVolumePolicy.Type = appsv1alpha1.ShareVolumePolicyEnabled
			sidecarSet.Spec.UpdateStrategy.Type = appsv1alpha1.NotUpdateSidecarSetStrategyType

			// ensure pod is not the latest sidecarset version
			oldPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`
			newPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`

			decoder, _ := admission.NewDecoder(scheme.Scheme)
			client := fake.NewFakeClient(sidecarSet)
			podOut := newPod.DeepCopy()
			podHandler := &PodCreateHandler{Decoder: decoder, Client: client}
			oldPodRaw := runtime.RawExtension{
				Raw: []byte(util.DumpJSON(oldPod)),
			}
			req := newAdmission(admissionv1.Update, runtime.RawExtension{}, oldPodRaw, "")
			err := podHandler.sidecarsetMutatingPod(context.Background(), req, podOut)
			if err != nil {
				t.Fatalf("inject sidecar into pod failed, err: %v", err)
			}

			if len(podOut.Spec.Containers) != cs.expectContainerLen {
				t.Fatalf("expect %v containers but got %v", cs.expectContainerLen, len(podOut.Spec.Containers))
			}

			idx := cs.expectedSidecarCtnIdx
			if podOut.Spec.Containers[idx].Name != cs.expectedSidecarCtnName {
				t.Fatalf("expect first sidecar name as %v but got %v", cs.expectedSidecarCtnName, podOut.Spec.Containers[idx].Name)
			}

			if podOut.Spec.Containers[idx].Image != "dns-f-image:old" {
				t.Fatalf("expect not update sidecar but got new image %v", podOut.Spec.Containers[idx].Image)
			}

			if sidecarSet.Spec.Containers[0].UpgradeStrategy.UpgradeType == appsv1alpha1.SidecarContainerHotUpgrade {
				if podOut.Spec.Containers[idx+1].Image != "dns-f-image:old2" {
					t.Fatalf("expect not update sidecar but got new image %v", podOut.Spec.Containers[idx+1].Image)
				}
			}

			if !reflect.DeepEqual(podOut.Spec.Containers[idx].VolumeMounts, oldPod.Spec.Containers[idx].VolumeMounts) {
				t.Fatalf("expect not update sidecar, but expected volumeMounts %v, got %v",
					podOut.Spec.Containers[idx].VolumeMounts, oldPod.Spec.Containers[idx].VolumeMounts)
			}
			if sidecarSet.Spec.Containers[0].UpgradeStrategy.UpgradeType == appsv1alpha1.SidecarContainerHotUpgrade {
				if !reflect.DeepEqual(podOut.Spec.Containers[idx+1].VolumeMounts, oldPod.Spec.Containers[idx+1].VolumeMounts) {
					t.Fatalf("expect not update sidecar, but expected volumeMounts %v, got %v",
						podOut.Spec.Containers[idx+1].VolumeMounts, oldPod.Spec.Containers[idx+1].VolumeMounts)
				}
			}
		})
	}
}

func TestTransferEnvDuringAppUpgrade(t *testing.T) {
	podInput := pod1.DeepCopy()
	podInput.Annotations = map[string]string{
		sigmak8sapi.AnnotationPodSpecHash: "pod-v1-aaa",
	}
	oldPodWithColdUpgradeTmpl := podInput.DeepCopy()
	oldPodWithColdUpgradeTmpl.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	oldPodWithColdUpgradeTmpl2 := podInput.DeepCopy()
	oldPodWithColdUpgradeTmpl2.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v3-ccc"

	// 老 pod 带有 dns-f sidecar
	injectedCtns := []v1.Container{v1.Container{Name: "dns-f", Image: "dns-f-image:old", Env: []v1.EnvVar{
		{
			Name:  sidecarcontrol.SidecarEnvKey,
			Value: "true",
		},
		{
			Name:  "hello2",
			Value: "world2",
		},
	}}}

	oldPodWithColdUpgradeTmpl.Spec.Containers = append(injectedCtns, oldPodWithColdUpgradeTmpl.Spec.Containers...)
	oldPodWithColdUpgradeTmpl2.Spec.Containers = append(oldPodWithColdUpgradeTmpl2.Spec.Containers, injectedCtns...)

	oldPodWithHotUpgradeTmpl := podInput.DeepCopy()
	oldPodWithHotUpgradeTmpl.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	oldPodWithHotUpgradeTmpl2 := podInput.DeepCopy()
	oldPodWithHotUpgradeTmpl2.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v3-ccc"

	// 老 pod 带有 dns-f sidecar
	injectedCtns = []v1.Container{
		{
			Name:  "dns-f-1",
			Image: "dns-f-image:old",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
				{
					Name:  "hello2",
					Value: "world2",
				},
			},
		},
		{
			Name:  "dns-f-2",
			Image: "dns-f-image:old2",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
			},
		},
	}

	oldPodWithHotUpgradeTmpl.Spec.Containers = append(injectedCtns, oldPodWithHotUpgradeTmpl.Spec.Containers...)
	oldPodWithHotUpgradeTmpl2.Spec.Containers = append(oldPodWithHotUpgradeTmpl2.Spec.Containers, injectedCtns...)
	// 标记 内部ASI SidecarSet
	sidecarSetInput := sidecarsetWithTransferEnv.DeepCopy()
	sidecarSetInput.Labels[sidecarcontrol.LabelSidecarSetMode] = sidecarcontrol.SidecarSetASI
	cases := []struct {
		name                   string
		getOldPod              func() *v1.Pod
		getNewPod              func(oldPod *v1.Pod) *v1.Pod
		getSidecarSet          func() *appsv1alpha1.SidecarSet
		expectContainerLen     int
		expectedSidecarCtnIdx  int
		expectedSidecarCtnName string
	}{
		{
			name: "inplaceset upgrade + cold upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "inplaceset upgrade + cold upgrade strategy + after before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "inplaceset upgrade + hot upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithHotUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "inplaceset upgrade + hot upgrade strategy + after before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithHotUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "cloneset upgrade + cold upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v1-aaa"
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "cloneset upgrade + cold upgrade strategy + after before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v1-aaa"
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "cloneset upgrade + hot upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithHotUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v1-aaa"
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "cloneset upgrade + hot upgrade strategy + after before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithHotUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v1-aaa"
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f-1",
		},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			oldPod := cs.getOldPod()
			newPod := cs.getNewPod(oldPod)

			// ensure pod is not the latest sidecarset version
			oldPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`
			newPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`

			sidecarSet := cs.getSidecarSet()
			sidecarSet.Spec.UpdateStrategy.Type = appsv1alpha1.NotUpdateSidecarSetStrategyType

			newCtn := util.GetContainer("nginx", newPod)
			newCtn.Env[1] = v1.EnvVar{Name: "hello2", Value: "world3"}

			decoder, _ := admission.NewDecoder(scheme.Scheme)
			client := fake.NewFakeClient(sidecarSet)
			podOut := newPod.DeepCopy()
			podHandler := &PodCreateHandler{Decoder: decoder, Client: client}
			oldPodRaw := runtime.RawExtension{
				Raw: []byte(util.DumpJSON(oldPod)),
			}
			req := newAdmission(admissionv1.Update, runtime.RawExtension{}, oldPodRaw, "")
			err := podHandler.sidecarsetMutatingPod(context.Background(), req, podOut)
			if err != nil {
				t.Fatalf("inject sidecar into pod failed, err: %v", err)
			}

			if len(podOut.Spec.Containers) != cs.expectContainerLen {
				t.Fatalf("expect %v containers but got %v", cs.expectContainerLen, len(podOut.Spec.Containers))
			}

			idx := cs.expectedSidecarCtnIdx
			if podOut.Spec.Containers[idx].Name != cs.expectedSidecarCtnName {
				t.Fatalf("expect first sidecar name as %v but got %v", cs.expectedSidecarCtnName, podOut.Spec.Containers[idx].Name)
			}

			if podOut.Spec.Containers[idx].Image != "dns-f-image:1.0" {
				t.Fatalf("expect sidecar image dns-f-image:1.0 but got old image %v", podOut.Spec.Containers[idx].Image)
			}

			if sidecarSet.Spec.Containers[0].UpgradeStrategy.UpgradeType == appsv1alpha1.SidecarContainerHotUpgrade {
				if podOut.Spec.Containers[idx+1].Image != EmptyImageForHotUpgrade {
					t.Fatalf("expect update sidecar but got old image %v", podOut.Spec.Containers[idx+1].Image)
				}
			}

			if val := util.GetContainerEnvValue(&podOut.Spec.Containers[idx], "hello2"); val != "world3" {
				t.Fatalf("expect env with value 'world3' but got %v", val)
			}
			if sidecarSet.Spec.Containers[0].UpgradeStrategy.UpgradeType == appsv1alpha1.SidecarContainerHotUpgrade {
				if val := util.GetContainerEnvValue(&podOut.Spec.Containers[idx+1], "hello2"); val != "world3" {
					t.Fatalf("expect env with value 'world3' but got %v", val)
				}
			}
		})
	}
}

func TestShareMountsDuringUpgrade(t *testing.T) {
	podInput := pod1.DeepCopy()
	podInput.Annotations = map[string]string{
		sigmak8sapi.AnnotationPodSpecHash: "pod-v1-aaa",
	}
	oldPodWithColdUpgradeTmpl := podInput.DeepCopy()
	oldPodWithColdUpgradeTmpl.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	oldPodWithColdUpgradeTmpl2 := podInput.DeepCopy()
	oldPodWithColdUpgradeTmpl2.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v3-ccc"

	// 老 pod 带有 dns-f sidecar
	injectedCtns := []v1.Container{
		{
			Name:  "dns-f",
			Image: "dns-f-image:old",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
				{
					Name:  "hello1",
					Value: "world1",
				},
				{
					Name:  "hello2",
					Value: "world2",
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "volume-1",
					MountPath: "/a/b/c",
				},
				{
					Name:      "volume-2",
					MountPath: "/d/e/f",
				},
				{
					Name:      "volume-staragent",
					MountPath: "/staragent",
				},
				{
					Name:      "volume-b",
					MountPath: "/e/f",
				},
			},
		},
	}

	oldPodWithColdUpgradeTmpl.Spec.Containers = append(injectedCtns, oldPodWithColdUpgradeTmpl.Spec.Containers...)
	oldPodWithColdUpgradeTmpl2.Spec.Containers = append(oldPodWithColdUpgradeTmpl2.Spec.Containers, injectedCtns...)

	oldPodWithHotUpgradeTmpl := podInput.DeepCopy()
	oldPodWithHotUpgradeTmpl.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	oldPodWithHotUpgradeTmpl2 := podInput.DeepCopy()
	oldPodWithHotUpgradeTmpl2.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v3-ccc"

	// 老 pod 带有 dns-f sidecar
	injectedCtns = []v1.Container{
		{
			Name:  "dns-f-1",
			Image: "dns-f-image:old",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
				{
					Name:  "hello1",
					Value: "world1",
				},
				{
					Name:  "hello2",
					Value: "world2",
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "volume-1",
					MountPath: "/a/b/c",
				},
				{
					Name:      "volume-2",
					MountPath: "/d/e/f",
				},
				{
					Name:      "volume-staragent",
					MountPath: "/staragent",
				},
				{
					Name:      "volume-b",
					MountPath: "/e/f",
				},
			},
		},
		{
			Name:  "dns-f-2",
			Image: "dns-f-image:old",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
				{
					Name:  "hello1",
					Value: "world1",
				},
				{
					Name:  "hello2",
					Value: "world2",
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "volume-1",
					MountPath: "/a/b/c",
				},
				{
					Name:      "volume-2",
					MountPath: "/d/e/f",
				},
				{
					Name:      "volume-staragent",
					MountPath: "/staragent",
				},
				{
					Name:      "volume-b",
					MountPath: "/e/f",
				},
			},
		},
	}

	oldPodWithHotUpgradeTmpl.Spec.Containers = append(injectedCtns, oldPodWithHotUpgradeTmpl.Spec.Containers...)
	oldPodWithHotUpgradeTmpl2.Spec.Containers = append(oldPodWithHotUpgradeTmpl2.Spec.Containers, injectedCtns...)
	// 标记 内部ASI SidecarSet
	sidecarSetInput := sidecarSetWithStaragent.DeepCopy()
	sidecarSetInput.Labels[sidecarcontrol.LabelSidecarSetMode] = sidecarcontrol.SidecarSetASI
	cases := []struct {
		name                   string
		getOldPod              func() *v1.Pod
		getNewPod              func(oldPod *v1.Pod) *v1.Pod
		getSidecarSet          func() *appsv1alpha1.SidecarSet
		expectContainerLen     int
		expectedSidecarCtnIdx  int
		expectedSidecarCtnName string
	}{
		{
			name: "inplaceset upgrade + cold upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "inplaceset upgrade + cold upgrade strategy + after before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "inplaceset upgrade + hot upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithHotUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "inplaceset upgrade + hot upgrade strategy + after before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithHotUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := podInput.DeepCopy()
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "cloneset upgrade + cold upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v1-aaa"
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "cloneset upgrade + cold upgrade strategy + after before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithColdUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v1-aaa"
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerColdUpgrade
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				return sidecarSetIn
			},
			expectContainerLen:     2,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f",
		},
		{
			name: "cloneset upgrade + hot upgrade strategy + inject before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithHotUpgradeTmpl.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v1-aaa"
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.BeforeAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  0,
			expectedSidecarCtnName: "dns-f-1",
		},
		{
			name: "cloneset upgrade + hot upgrade strategy + after before app",
			getOldPod: func() *v1.Pod {
				pod := oldPodWithHotUpgradeTmpl2.DeepCopy()
				return pod
			},
			getNewPod: func(oldPod *v1.Pod) *v1.Pod {
				pod := oldPod.DeepCopy()
				pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v1-aaa"
				return pod
			},
			getSidecarSet: func() *appsv1alpha1.SidecarSet {
				sidecarSetIn := sidecarSetInput.DeepCopy()
				sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
				sidecarSetIn.Spec.Containers[0].PodInjectPolicy = appsv1alpha1.AfterAppContainerType
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.UpgradeType = appsv1alpha1.SidecarContainerHotUpgrade
				sidecarSetIn.Spec.Containers[0].UpgradeStrategy.HotUpgradeEmptyImage = EmptyImageForHotUpgrade
				return sidecarSetIn
			},
			expectContainerLen:     3,
			expectedSidecarCtnIdx:  1,
			expectedSidecarCtnName: "dns-f-1",
		},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			oldPod := cs.getOldPod()
			newPod := cs.getNewPod(oldPod)
			sidecarSet := cs.getSidecarSet()
			sidecarSet.Spec.UpdateStrategy.Type = appsv1alpha1.NotUpdateSidecarSetStrategyType

			newCtn := util.GetContainer("nginx", newPod)
			newCtn.VolumeMounts = append(newCtn.VolumeMounts, v1.VolumeMount{Name: "volume-new", MountPath: "/mount-new"})
			newPod.Spec.Volumes = append(newPod.Spec.Volumes, v1.Volume{
				Name:         "volume-new",
				VolumeSource: v1.VolumeSource{},
			})

			// ensure pod is updated with latest sidecarset version
			// ensure pod is not the latest sidecarset version
			oldPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`
			newPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`

			decoder, _ := admission.NewDecoder(scheme.Scheme)
			client := fake.NewFakeClient(sidecarSet)
			podOut := newPod.DeepCopy()
			podHandler := &PodCreateHandler{Decoder: decoder, Client: client}
			oldPodRaw := runtime.RawExtension{
				Raw: []byte(util.DumpJSON(oldPod)),
			}
			req := newAdmission(admissionv1.Update, runtime.RawExtension{}, oldPodRaw, "")
			err := podHandler.sidecarsetMutatingPod(context.Background(), req, podOut)
			if err != nil {
				t.Fatalf("inject sidecar into pod failed, err: %v", err)
			}

			if len(podOut.Spec.Containers) != cs.expectContainerLen {
				t.Fatalf("expect %v containers but got %v", cs.expectContainerLen, len(podOut.Spec.Containers))
			}

			idx := cs.expectedSidecarCtnIdx
			if podOut.Spec.Containers[idx].Name != cs.expectedSidecarCtnName {
				t.Fatalf("expect first sidecar name as %v but got %v", cs.expectedSidecarCtnName, podOut.Spec.Containers[idx].Name)
			}

			if podOut.Spec.Containers[idx].Image != "dns-f-image:1.0" {
				t.Fatalf("expect sidecar image dns-f-image:1.0 but got new image %v", podOut.Spec.Containers[idx].Image)
			}

			if sidecarSet.Spec.Containers[0].UpgradeStrategy.UpgradeType == appsv1alpha1.SidecarContainerHotUpgrade {
				if podOut.Spec.Containers[idx+1].Image != EmptyImageForHotUpgrade {
					t.Fatalf("expect update sidecar but got old image %v", podOut.Spec.Containers[idx+1].Image)
				}
			}

			if val := util.GetContainerVolumeMount(&podOut.Spec.Containers[idx], "/mount-new"); val == nil {
				t.Fatalf("miss mount '/mount-new'")
			}
			if sidecarSet.Spec.Containers[0].UpgradeStrategy.UpgradeType == appsv1alpha1.SidecarContainerHotUpgrade {
				if val := util.GetContainerVolumeMount(&podOut.Spec.Containers[idx+1], "/mount-new"); val == nil {
					t.Fatalf("miss mount '/mount-new'")
				}
			}

			if vol := util.GetPodVolume(podOut, "volume-new"); vol == nil {
				t.Fatalf("miss volume 'volume-new'")
			}
		})
	}
}

func TestUpgradeSidecarWhenInplaceSetUpgrade(t *testing.T) {
	podInput := pod1.DeepCopy()
	podInput.Annotations = map[string]string{
		sigmak8sapi.AnnotationPodSpecHash: "pod-v1-aaa",
	}
	oldPod, newPod := podInput.DeepCopy(), podInput.DeepCopy()
	oldPod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	// 老 pod 带有 dns-f sidecar
	oldPod.Spec.Containers = append(oldPod.Spec.Containers, v1.Container{Name: "dns-f", Image: "dns-f-image:old", Env: []v1.EnvVar{
		{
			Name:  sidecarcontrol.SidecarEnvKey,
			Value: "true",
		},
	}})
	sidecarSetIn := sidecarSet1.DeepCopy()
	sidecarSetIn.Labels[sidecarcontrol.LabelSidecarSetMode] = sidecarcontrol.SidecarSetASI
	sidecarSetIn.Spec.UpdateStrategy.Type = appsv1alpha1.RollingUpdateSidecarSetStrategyType

	decoder, _ := admission.NewDecoder(scheme.Scheme)
	client := fake.NewFakeClient(sidecarSetIn)
	podOut := newPod.DeepCopy()
	podHandler := &PodCreateHandler{Decoder: decoder, Client: client}
	oldPodRaw := runtime.RawExtension{
		Raw: []byte(util.DumpJSON(oldPod)),
	}
	req := newAdmission(admissionv1.Update, runtime.RawExtension{}, oldPodRaw, "")
	err := podHandler.sidecarsetMutatingPod(context.Background(), req, podOut)
	if err != nil {
		t.Fatalf("inject sidecar into pod failed, err: %v", err)
	}

	if len(podOut.Spec.Containers) != 2 {
		t.Fatalf("expect 2 containers but got %v", len(podOut.Spec.Containers))
	}
	if podOut.Spec.Containers[0].Name != "dns-f" {
		t.Fatalf("expect dns-f injected but got %v", podOut.Spec.Containers[0].Name)
	}
	if podOut.Spec.Containers[0].Image != "dns-f-image:old" {
		t.Fatalf("expect old dns-f but got new image %v", podOut.Spec.Containers[0].Image)
	}
}

func TestUpgradeSidecarWhenCloneSetUpgrade(t *testing.T) {
	podInput := pod1.DeepCopy()
	podInput.Annotations = map[string]string{
		sigmak8sapi.AnnotationPodSpecHash: "pod-v1-aaa",
	}
	oldPod := podInput.DeepCopy()
	oldPod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	// 老 pod 带有 dns-f sidecar
	oldPod.Spec.Containers = append([]v1.Container{{Name: "dns-f", Image: "dns-f-image:old", Env: []v1.EnvVar{
		{
			Name:  sidecarcontrol.SidecarEnvKey,
			Value: "true",
		},
	}}}, oldPod.Spec.Containers...)
	oldPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`

	newPod := oldPod.DeepCopy()
	// ensure pod is not the latest sidecarset version
	//oldPod.Annotations = map[string]string{sidecarcontrol.SidecarSetHashAnnotation: fmt.Sprintf(`{"dns-f":"old-hash"}`)}
	//newPod.Annotations = map[string]string{sidecarcontrol.SidecarSetHashAnnotation: fmt.Sprintf(`{"dns-f":"old-hash"}`)}

	sidecarSetIn := sidecarSet1.DeepCopy()
	sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
	sidecarSetIn.Spec.UpdateStrategy.Type = appsv1alpha1.RollingUpdateSidecarSetStrategyType

	decoder, _ := admission.NewDecoder(scheme.Scheme)
	client := fake.NewFakeClient(sidecarSetIn)
	podOut := newPod.DeepCopy()
	podHandler := &PodCreateHandler{Decoder: decoder, Client: client}
	oldPodRaw := runtime.RawExtension{
		Raw: []byte(util.DumpJSON(oldPod)),
	}
	req := newAdmission(admissionv1.Update, runtime.RawExtension{}, oldPodRaw, "")
	err := podHandler.sidecarsetMutatingPod(context.Background(), req, podOut)
	if err != nil {
		t.Fatalf("inject sidecar into pod failed, err: %v", err)
	}

	if len(podOut.Spec.Containers) != 2 {
		t.Fatalf("expect 3 containers but got %v", len(podOut.Spec.Containers))
	}
	if podOut.Spec.Containers[0].Name != "dns-f" {
		t.Fatalf("expect dns-f injected but got %v", podOut.Spec.Containers[0].Name)
	}
	if podOut.Spec.Containers[0].Image != "dns-f-image:old" {
		t.Fatalf("expect old dns-f but got new image %v", podOut.Spec.Containers[0].Image)
	}
}

func TestUpgradeSidecarWhenSidecarSetUpgrade(t *testing.T) {
	podInput := pod1.DeepCopy()
	podInput.Annotations = map[string]string{
		sigmak8sapi.AnnotationPodSpecHash: "pod-v1-aaa",
	}
	oldPod := podInput.DeepCopy()
	oldPod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	// 老 pod 带有 dns-f sidecar
	oldPod.Spec.Containers = append([]v1.Container{{Name: "dns-f", Image: "dns-f-image:old", Env: []v1.EnvVar{
		{
			Name:  sidecarcontrol.SidecarEnvKey,
			Value: "true",
		},
	}}}, oldPod.Spec.Containers...)
	oldPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`

	newPod := oldPod.DeepCopy()
	util.GetContainer("dns-f", newPod).Image = "dns-f-image:1.0"
	// ensure pod is not the latest sidecarset version
	//oldPod.Annotations = map[string]string{sidecarcontrol.SidecarSetHashAnnotation: fmt.Sprintf(`{"dns-f":"old-hash"}`)}
	newPod.Annotations = map[string]string{
		sidecarcontrol.SidecarSetHashAnnotation: fmt.Sprintf(`{"dns-f":{"hash":"%s"}}`, sidecarcontrol.GetSidecarSetRevision(sidecarSet1)),
	}

	sidecarSetIn := sidecarSet1.DeepCopy()
	sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
	sidecarSetIn.Spec.UpdateStrategy.Type = appsv1alpha1.RollingUpdateSidecarSetStrategyType

	decoder, _ := admission.NewDecoder(scheme.Scheme)
	client := fake.NewFakeClient(sidecarSetIn)
	podOut := newPod.DeepCopy()
	podHandler := &PodCreateHandler{Decoder: decoder, Client: client}
	oldPodRaw := runtime.RawExtension{
		Raw: []byte(util.DumpJSON(oldPod)),
	}
	req := newAdmission(admissionv1.Update, runtime.RawExtension{}, oldPodRaw, "")
	err := podHandler.sidecarsetMutatingPod(context.Background(), req, podOut)
	if err != nil {
		t.Fatalf("inject sidecar into pod failed, err: %v", err)
	}

	if len(podOut.Spec.Containers) != 2 {
		t.Fatalf("expect 2 containers but got %v", len(podOut.Spec.Containers))
	}
	if podOut.Spec.Containers[0].Name != "dns-f" {
		t.Fatalf("expect dns-f injected but got %v", podOut.Spec.Containers[0].Name)
	}
	if podOut.Spec.Containers[0].Image != "dns-f-image:1.0" {
		t.Fatalf("expect inject dns-f but got old image %v", podOut.Spec.Containers[0].Image)
	}
}

func TestUpgradeSidecarWhenSidecarSetHotUpgrade(t *testing.T) {
	podInput := pod1.DeepCopy()
	podInput.Annotations = map[string]string{
		sigmak8sapi.AnnotationPodSpecHash: "pod-v1-aaa",
	}
	oldPod := podInput.DeepCopy()
	oldPod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = "pod-v2-bbb"
	// 老 pod 带有 dns-f sidecar
	oldPod.Spec.Containers = append([]v1.Container{
		{
			Name:  "dns-f-1",
			Image: "dns-f-image:old",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
				{
					Name:  sidecarcontrol.SidecarSetVersionEnvKey,
					Value: "1",
				},
			},
		},
		{
			Name:  "dns-f-2",
			Image: EmptyImageForHotUpgrade,
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
				{
					Name:  sidecarcontrol.SidecarSetVersionEnvKey,
					Value: "0",
				},
			},
		},
	}, oldPod.Spec.Containers...)
	oldPod.Annotations[sidecarcontrol.SidecarSetHashAnnotation] = `{"dns-f":{"hash":"old-hash"}}`

	newPod := pod1.DeepCopy()
	newPod.Spec.Containers = append([]v1.Container{
		{
			Name:  "dns-f-1",
			Image: "dns-f-image:old",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
				{
					Name:  sidecarcontrol.SidecarSetVersionEnvKey,
					Value: "1",
				},
			},
		},
		{
			Name:  "dns-f-2",
			Image: "dns-f-image:1.0",
			Env: []v1.EnvVar{
				{
					Name:  sidecarcontrol.SidecarEnvKey,
					Value: "true",
				},
				{
					Name:  sidecarcontrol.SidecarSetVersionEnvKey,
					Value: "2",
				},
			},
		},
	}, newPod.Spec.Containers...)
	// ensure pod is not the latest sidecarset version
	newPod.Annotations = map[string]string{
		sidecarcontrol.SidecarSetHashAnnotation: `{"dns-f":{"hash":"new-hash-sidecarset"}}`,
	}

	sidecarSetIn := sidecarSet1.DeepCopy()
	sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
	sidecarSetIn.Spec.UpdateStrategy.Type = appsv1alpha1.RollingUpdateSidecarSetStrategyType

	decoder, _ := admission.NewDecoder(scheme.Scheme)
	client := fake.NewFakeClient(sidecarSetIn)
	podOut := newPod.DeepCopy()
	podHandler := &PodCreateHandler{Decoder: decoder, Client: client}
	oldPodRaw := runtime.RawExtension{
		Raw: []byte(util.DumpJSON(oldPod)),
	}
	req := newAdmission(admissionv1.Update, runtime.RawExtension{}, oldPodRaw, "")
	err := podHandler.sidecarsetMutatingPod(context.Background(), req, podOut)
	if err != nil {
		t.Fatalf("inject sidecar into pod failed, err: %v", err)
	}

	if len(podOut.Spec.Containers) != 3 {
		t.Fatalf("expect 3 containers but got %v", len(podOut.Spec.Containers))
	}
	if podOut.Spec.Containers[0].Name != "dns-f-1" {
		t.Fatalf("expect dns-f injected but got %v", podOut.Spec.Containers[0].Name)
	}
	if podOut.Spec.Containers[0].Image != "dns-f-image:old" {
		t.Fatalf("expect dns-f image dns-f-image:old but got image %v", podOut.Spec.Containers[0].Image)
	}

	if podOut.Spec.Containers[1].Name != "dns-f-2" {
		t.Fatalf("expect dns-f injected but got %v", podOut.Spec.Containers[1].Name)
	}
	if podOut.Spec.Containers[1].Image != "dns-f-image:1.0" {
		t.Fatalf("expect dns-f image dns-f-image:1.0 but got image %v", podOut.Spec.Containers[1].Image)
	}
}

func TestPodVolumeMountsAppendASI(t *testing.T) {
	sidecarSetIn := sidecarSetWithStaragent.DeepCopy()
	// /a/b/c, /d/e/f, /staragent
	sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
	setSidecarSetASI(sidecarSetIn)
	testPodVolumeMountsAppend(t, sidecarSetIn)
}
