/*
Copyright 2019 The Kruise Authors.

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

package imagepuller

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	imageruntime "github.com/openkruise/kruise/pkg/daemonset/imagepuller/runtime"
)

type FakePullStatusReader struct {
	internalC chan imageruntime.ImagePullStatus
}

func (r *FakePullStatusReader) C() <-chan imageruntime.ImagePullStatus {
	return r.internalC
}

func (r *FakePullStatusReader) Close() {
	close(r.internalC)
}

type FakeImageRuntime struct {
	sync.Mutex

	readers map[string]*FakePullStatusReader
}

func (r *FakeImageRuntime) PullImage(ctx context.Context, imageName, tag string, pullSecrets []v1.Secret) (imageruntime.ImagePullStatusReader, error) {
	r.Lock()
	defer r.Unlock()

	fakeReader := &FakePullStatusReader{
		internalC: make(chan imageruntime.ImagePullStatus, 10),
	}
	if r.readers == nil {
		r.readers = make(map[string]*FakePullStatusReader)
	}
	fullName := fmt.Sprintf("%v:%v", imageName, tag)
	r.readers[fullName] = fakeReader
	return fakeReader, nil
}

func (r *FakeImageRuntime) ListImages(ctx context.Context) ([]imageruntime.ImageInfo, error) {
	return nil, nil
}

func (r *FakeImageRuntime) waitPull(image string, timeout time.Duration) (*FakePullStatusReader, error) {
	start := time.Now()
	for {
		r.Lock()
		reader, ok := r.readers[image]
		r.Unlock()
		if ok {
			return reader, nil
		}
		if time.Since(start) > timeout {
			return nil, fmt.Errorf("timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

type FakeSecretManager struct {
}

func (s *FakeSecretManager) GetSecrets(secrets []appsv1alpha1.ReferenceObject) ([]v1.Secret, error) {
	return nil, nil
}

func TestPullerStartAndStopWorkers(t *testing.T) {
	fakeRuntime := &FakeImageRuntime{}
	fakeSecretManager := &FakeSecretManager{}

	puller, _ := newRealPuller(fakeRuntime, fakeSecretManager, nil)

	testCases := []struct {
		nodeSpec *appsv1alpha1.NodeImageSpec
		expected map[string]int
	}{
		{
			nodeSpec: &appsv1alpha1.NodeImageSpec{
				Images: map[string]appsv1alpha1.ImageSpec{
					"busybox": {
						Tags: []appsv1alpha1.ImageTagSpec{
							{
								Tag: "v1",
							},
						},
					},
				},
			},
			expected: map[string]int{"busybox": 1},
		},
		{
			nodeSpec: &appsv1alpha1.NodeImageSpec{
				Images: map[string]appsv1alpha1.ImageSpec{
					"busybox": {
						Tags: []appsv1alpha1.ImageTagSpec{
							{
								Tag: "v1",
							},
							{
								Tag: "v2",
							},
						},
					},
				},
			},
			expected: map[string]int{"busybox": 2},
		},
		{
			nodeSpec: &appsv1alpha1.NodeImageSpec{
				Images: map[string]appsv1alpha1.ImageSpec{
					"busybox": {
						Tags: []appsv1alpha1.ImageTagSpec{
							{
								Tag: "v1",
							},
							{
								Tag: "v2",
							},
						},
					},
					"centos": {
						Tags: []appsv1alpha1.ImageTagSpec{
							{
								Tag: "v1",
							},
						},
					},
				},
			},
			expected: map[string]int{"busybox": 2, "centos": 1},
		},
		{
			nodeSpec: &appsv1alpha1.NodeImageSpec{
				Images: map[string]appsv1alpha1.ImageSpec{
					"centos": {
						Tags: []appsv1alpha1.ImageTagSpec{
							{
								Tag: "v1",
							},
							{
								Tag: "v2",
							},
						},
					},
				},
			},
			expected: map[string]int{"centos": 2},
		},
	}

	for _, cs := range testCases {
		if err := puller.Sync(cs.nodeSpec, nil); err != nil {
			t.Fatalf("unepected sync err %v", err)
		}
		for image := range cs.expected {
			worker, ok := puller.workerPools[image]
			if !ok || len(worker.(*realWorkerPool).pullWorkers) != cs.expected[image] {
				t.Fatalf("failed to start worker for image %v, expected %v actual %v", image, cs.expected[image], len(worker.(*realWorkerPool).pullWorkers))
			}
		}
	}
}

func waitGetTagStatus(p puller, imageName, tag string, expected appsv1alpha1.ImagePullPhase, timeout time.Duration) (*appsv1alpha1.ImageTagStatus, error) {
	start := time.Now()
	var lastStatus *appsv1alpha1.ImageTagStatus
	for {
		status := p.GetStatus(imageName)
		if status != nil {
			for _, tagStatus := range status.Tags {
				if tagStatus.Tag == tag {
					if tagStatus.Phase == expected {
						return &tagStatus, nil
					}
					lastStatus = &tagStatus
				}
			}
		}
		if time.Since(start) > timeout {
			return lastStatus, fmt.Errorf("timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestPullerMergePullStatus(t *testing.T) {
	fakeRuntime := &FakeImageRuntime{}
	fakeSecretManager := &FakeSecretManager{}

	puller, _ := newRealPuller(fakeRuntime, fakeSecretManager, nil)

	backoffLimit := int32(1)
	nodeSpec := &appsv1alpha1.NodeImageSpec{
		Images: map[string]appsv1alpha1.ImageSpec{
			"busybox": {
				Tags: []appsv1alpha1.ImageTagSpec{
					{
						Tag: "v1",
					},
					{
						Tag: "v2",
						PullPolicy: &appsv1alpha1.ImageTagPullPolicy{
							BackoffLimit: &backoffLimit,
						},
					},
				},
			},
		},
	}
	if err := puller.Sync(nodeSpec, nil); err != nil {
		t.Fatalf("unepected sync err %v", err)
	}

	// tag v1
	reader, err := fakeRuntime.waitPull("busybox:v1", time.Second)
	if err != nil {
		t.Errorf("failed to start worker")
	}
	// pull succeed
	reader.internalC <- imageruntime.ImagePullStatus{Process: 100, Finish: true}

	pullStatus, _ := waitGetTagStatus(puller, "busybox", "v1", appsv1alpha1.ImagePhaseSucceeded, time.Second)
	if pullStatus == nil {
		t.Fatalf("failed to get pull status")
	}
	if pullStatus.CompletionTime == nil {
		t.Fatalf("unexpected image status %#v", pullStatus)
	}

	// tag v2
	reader, err = fakeRuntime.waitPull("busybox:v2", time.Second)
	if err != nil {
		t.Errorf("failed to start worker")
	}
	// pulling
	reader.internalC <- imageruntime.ImagePullStatus{Process: 10}
	pullStatus, _ = waitGetTagStatus(puller, "busybox", "v2", appsv1alpha1.ImagePhasePulling, time.Second)
	if pullStatus.CompletionTime != nil {
		t.Fatalf("unexpected image status %#v", pullStatus)
	}
	// failed
	reader.internalC <- imageruntime.ImagePullStatus{Process: 10, Err: fmt.Errorf("pull failed"), Finish: true}

	pullStatus, _ = waitGetTagStatus(puller, "busybox", "v2", appsv1alpha1.ImagePhaseFailed, 2*time.Second)
	if pullStatus.CompletionTime == nil || pullStatus.Progress != 10 {
		t.Errorf("unexpected image status %#v", pullStatus)
	}
}
