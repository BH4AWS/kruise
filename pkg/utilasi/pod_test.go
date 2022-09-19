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

package utilasi

import (
	"fmt"
	"reflect"
	"testing"

	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"

	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestASIPodUpdateAPI(t *testing.T) {
	base := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}, Labels: map[string]string{}},
		Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "c1"}, {Name: "c2"}}},
	}

	cases := []struct {
		isVK         bool
		setHash      string
		expectedSpec interface{}

		mockStatusAnnotation          map[string]string
		checkConsistentWithContainers sets.String
		expectedToBeConsistent        bool
	}{
		// cases for Sigma API
		{
			isVK:         false,
			setHash:      "ABC",
			expectedSpec: "ABC",

			mockStatusAnnotation:   map[string]string{},
			expectedToBeConsistent: false,
		},
		{
			isVK:         false,
			setHash:      "ABC",
			expectedSpec: "ABC",

			mockStatusAnnotation: map[string]string{
				sigmak8sapi.AnnotationPodUpdateStatus: `{"statuses":{"c1":{"specHash":"ABC"},"c2":{"specHash":"ABC"}}}`,
			},
			expectedToBeConsistent: true,
		},
		{
			isVK:         false,
			setHash:      "ABC",
			expectedSpec: "ABC",

			mockStatusAnnotation: map[string]string{
				sigmak8sapi.AnnotationPodUpdateStatus: `{"statuses":{"c2":{"specHash":"ABC"}}}`,
			},
			expectedToBeConsistent: false,
		},
		{
			isVK:         false,
			setHash:      "ABC",
			expectedSpec: "ABC",

			mockStatusAnnotation: map[string]string{
				sigmak8sapi.AnnotationPodUpdateStatus: `{"statuses":{"c2":{"specHash":"ABC"}}}`,
			},
			checkConsistentWithContainers: sets.NewString("c1"),
			expectedToBeConsistent:        false,
		},
		{
			isVK:         false,
			setHash:      "ABC",
			expectedSpec: "ABC",

			mockStatusAnnotation: map[string]string{
				sigmak8sapi.AnnotationPodUpdateStatus: `{"statuses":{"c2":{"specHash":"ABC"}}}`,
			},
			checkConsistentWithContainers: sets.NewString("c2"),
			expectedToBeConsistent:        true,
		},

		// cases for Unified API
		{
			isVK:    true,
			setHash: "ABC",
			expectedSpec: &apiinternal.ContainerUpdateSpec{Version: "ABC", Containers: []apiinternal.ContainerVersion{
				{Name: "c1", Version: "ABC"}, {Name: "c2", Version: "ABC"},
			}},

			mockStatusAnnotation:   map[string]string{},
			expectedToBeConsistent: false,
		},
		{
			isVK:    true,
			setHash: "ABC",
			expectedSpec: &apiinternal.ContainerUpdateSpec{Version: "ABC", Containers: []apiinternal.ContainerVersion{
				{Name: "c1", Version: "ABC"}, {Name: "c2", Version: "ABC"},
			}},

			mockStatusAnnotation: map[string]string{
				apiinternal.AnnotationUpdateStatus: `{"version": "ABC", "containers":[{"name": "c1", "version": "ABC"}, {"name": "c2", "version": "ABC"}]}`,
			},
			expectedToBeConsistent: true,
		},
		{
			isVK:    true,
			setHash: "ABC",
			expectedSpec: &apiinternal.ContainerUpdateSpec{Version: "ABC", Containers: []apiinternal.ContainerVersion{
				{Name: "c1", Version: "ABC"}, {Name: "c2", Version: "ABC"},
			}},

			mockStatusAnnotation: map[string]string{
				apiinternal.AnnotationUpdateStatus: `{"version": "", "containers":[{"name": "c2", "version": "ABC"}]}`,
			},
			expectedToBeConsistent: false,
		},
		{
			isVK:    true,
			setHash: "ABC",
			expectedSpec: &apiinternal.ContainerUpdateSpec{Version: "ABC", Containers: []apiinternal.ContainerVersion{
				{Name: "c1", Version: "ABC"}, {Name: "c2", Version: "ABC"},
			}},

			mockStatusAnnotation: map[string]string{
				apiinternal.AnnotationUpdateStatus: `{"version": "", "containers":[{"name": "c2", "version": "ABC"}]}`,
			},
			checkConsistentWithContainers: sets.NewString("c1"),
			expectedToBeConsistent:        false,
		},
		{
			isVK:    true,
			setHash: "ABC",
			expectedSpec: &apiinternal.ContainerUpdateSpec{Version: "ABC", Containers: []apiinternal.ContainerVersion{
				{Name: "c1", Version: "ABC"}, {Name: "c2", Version: "ABC"},
			}},

			mockStatusAnnotation: map[string]string{
				apiinternal.AnnotationUpdateStatus: `{"version": "", "containers":[{"name": "c2", "version": "ABC"}]}`,
			},
			checkConsistentWithContainers: sets.NewString("c2"),
			expectedToBeConsistent:        true,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			pod := base.DeepCopy()
			if tc.isVK {
				pod.Labels[apiinternal.LabelNodeType] = apiinternal.NodeTypeVirtualKubelet
			}
			SetUpdateSpecHash(pod, tc.setHash)
			if !reflect.DeepEqual(GetPodSpecHashString(pod), tc.setHash) {
				t.Fatalf("GetPodSpecHashString %s is not equal to %s", GetPodSpecHashString(pod), tc.setHash)
			}
			var gotSpec interface{}
			if tc.isVK {
				gotSpec = getUnifiedUpdateSpec(pod)
			} else {
				gotSpec = getSigmaUpdateSpec(pod)
			}
			if !reflect.DeepEqual(gotSpec, tc.expectedSpec) {
				t.Fatalf("got update spec %v is not equal to %v", gotSpec, tc.expectedSpec)
			}

			for k, v := range tc.mockStatusAnnotation {
				pod.Annotations[k] = v
			}
			isConsistent := IsPodSpecHashPartConsistent(pod, tc.checkConsistentWithContainers)
			if isConsistent != tc.expectedToBeConsistent {
				t.Fatalf("got consistent %v, but expected to be %v", isConsistent, tc.expectedToBeConsistent)
			}
		})
	}
}
