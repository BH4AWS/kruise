package labelpatcher

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	oldRolloutId    = "rolling-order-old"
	newRolloutId    = "rolling-order-new"
	updateRevision  = "updateRevision"
	currentRevision = "currentRevision"
)

var (
	scheme       = runtime.NewScheme()
	cloneSetDemo = &appsv1alpha1.CloneSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.kruise.io/v1alpha1",
			Kind:       "CloneSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "demo",
			Namespace:  "default",
			Generation: 2,
			UID:        types.UID("1234567890"),
			Labels: map[string]string{
				apiinternal.LabelRolloutId:      newRolloutId,
				apiinternal.LabelRolloutBatchId: "1",
			},
		},
		Spec: appsv1alpha1.CloneSetSpec{
			Replicas: pointer.Int32(10),
		},
		Status: appsv1alpha1.CloneSetStatus{
			ExpectedUpdatedReplicas: 5,
			ObservedGeneration:      2,
			UpdateRevision:          updateRevision,
		},
	}
	podDemo = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod",
			Namespace: "default",
			Labels: map[string]string{
				apps.ControllerRevisionHashLabelKey: updateRevision,
			},
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cloneSetDemo.DeepCopy(), cloneSetDemo.GroupVersionKind())},
		},
	}
)

func init() {
	v1.AddToScheme(scheme)
	appsv1alpha1.AddToScheme(scheme)
}

func makeFakePods(replicas, beginningOrder int, rolloutId, batchId, revision string) []client.Object {
	pods := make([]client.Object, 0, replicas)
	for i := 0; i < replicas; i++ {
		pod := podDemo.DeepCopy()
		pod.UID = uuid.NewUUID()
		pod.Labels[apiinternal.LabelRolloutId] = rolloutId
		pod.Labels[apiinternal.LabelRolloutBatchId] = batchId
		pod.Labels[apps.ControllerRevisionHashLabelKey] = revision
		pod.SetName("pod-" + fmt.Sprintf("pod-%s-%d", rand.String(8), beginningOrder+i))
		pods = append(pods, pod)
	}
	return pods
}

func TestReconcile(t *testing.T) {
	cases := []struct {
		name        string
		pods        func() []client.Object
		cloneSet    func() *appsv1alpha1.CloneSet
		expectBatch map[string]int
	}{
		{
			name: "normal rolling: batch 1",
			cloneSet: func() *appsv1alpha1.CloneSet {
				clone := cloneSetDemo.DeepCopy()
				clone.Status.ExpectedUpdatedReplicas = 2
				clone.Labels[apiinternal.LabelRolloutBatchId] = "1"
				return clone
			},
			pods: func() []client.Object {
				pods := make([]client.Object, 0, 10)
				pods = append(pods, makeFakePods(2, 0, oldRolloutId, "1", updateRevision)...)
				pods = append(pods, makeFakePods(4, 2, oldRolloutId, "2", currentRevision)...)
				pods = append(pods, makeFakePods(4, 6, oldRolloutId, "3", currentRevision)...)
				return pods
			},
			expectBatch: map[string]int{
				"1": 2,
			},
		},
		{
			name: "normal rolling: batch 2",
			cloneSet: func() *appsv1alpha1.CloneSet {
				clone := cloneSetDemo.DeepCopy()
				clone.Status.ExpectedUpdatedReplicas = 6
				clone.Labels[apiinternal.LabelRolloutBatchId] = "2"
				return clone
			},
			pods: func() []client.Object {
				pods := make([]client.Object, 0, 10)
				pods = append(pods, makeFakePods(2, 0, newRolloutId, "1", updateRevision)...) // 2 patched
				pods = append(pods, makeFakePods(4, 2, oldRolloutId, "1", updateRevision)...)
				pods = append(pods, makeFakePods(4, 6, oldRolloutId, "3", currentRevision)...)
				return pods
			},
			expectBatch: map[string]int{
				"1": 2,
				"2": 4,
			},
		},
		{
			name: "rollback in batches: before batch 1",
			cloneSet: func() *appsv1alpha1.CloneSet {
				clone := cloneSetDemo.DeepCopy()
				clone.Status.ExpectedUpdatedReplicas = 7
				clone.Labels[apiinternal.LabelRolloutBatchId] = "1"
				return clone
			},
			pods: func() []client.Object {
				pods := make([]client.Object, 0, 10)
				pods = append(pods, makeFakePods(2, 0, oldRolloutId, "1", currentRevision)...) // 2 patched
				pods = append(pods, makeFakePods(4, 2, oldRolloutId, "2", currentRevision)...)
				pods = append(pods, makeFakePods(4, 6, oldRolloutId, "3", updateRevision)...)
				return pods
			},
			expectBatch: map[string]int{
				"1": 4,
			},
		},
		{
			name: "rollback in batches: after batch 1",
			cloneSet: func() *appsv1alpha1.CloneSet {
				clone := cloneSetDemo.DeepCopy()
				clone.Status.ExpectedUpdatedReplicas = 7
				clone.Labels[apiinternal.LabelRolloutBatchId] = "1"
				return clone
			},
			pods: func() []client.Object {
				pods := make([]client.Object, 0, 10)
				pods = append(pods, makeFakePods(3, 0, oldRolloutId, "3", updateRevision)...) // 2 patched
				pods = append(pods, makeFakePods(3, 3, oldRolloutId, "2", currentRevision)...)
				pods = append(pods, makeFakePods(4, 6, newRolloutId, "1", updateRevision)...)
				return pods
			},
			expectBatch: map[string]int{
				"1": 7,
			},
		},
		{
			name: "only change rollout-id: batch 1",
			cloneSet: func() *appsv1alpha1.CloneSet {
				clone := cloneSetDemo.DeepCopy()
				clone.Status.ExpectedUpdatedReplicas = 2
				clone.Labels[apiinternal.LabelRolloutBatchId] = "1"
				return clone
			},
			pods: func() []client.Object {
				pods := make([]client.Object, 0, 10)
				pods = append(pods, makeFakePods(2, 0, oldRolloutId, "1", updateRevision)...)
				pods = append(pods, makeFakePods(4, 2, oldRolloutId, "2", updateRevision)...)
				pods = append(pods, makeFakePods(4, 6, oldRolloutId, "3", updateRevision)...)
				return pods
			},
			expectBatch: map[string]int{
				"1": 2,
			},
		},
		{
			name: "only change rollout-id: batch 2",
			cloneSet: func() *appsv1alpha1.CloneSet {
				clone := cloneSetDemo.DeepCopy()
				clone.Status.ExpectedUpdatedReplicas = 6
				clone.Labels[apiinternal.LabelRolloutBatchId] = "2"
				return clone
			},
			pods: func() []client.Object {
				pods := make([]client.Object, 0, 10)
				pods = append(pods, makeFakePods(2, 0, newRolloutId, "1", updateRevision)...)
				pods = append(pods, makeFakePods(4, 2, oldRolloutId, "2", updateRevision)...)
				pods = append(pods, makeFakePods(4, 6, oldRolloutId, "3", updateRevision)...)
				return pods
			},
			expectBatch: map[string]int{
				"1": 2,
				"2": 4,
			},
		},
		{
			name: "satisfied, and nothing need to patch",
			cloneSet: func() *appsv1alpha1.CloneSet {
				clone := cloneSetDemo.DeepCopy()
				clone.Status.ExpectedUpdatedReplicas = 10
				clone.Labels[apiinternal.LabelRolloutBatchId] = "3"
				return clone
			},
			pods: func() []client.Object {
				pods := make([]client.Object, 0, 10)
				pods = append(pods, makeFakePods(2, 0, newRolloutId, "1", updateRevision)...)
				pods = append(pods, makeFakePods(4, 2, newRolloutId, "2", updateRevision)...)
				pods = append(pods, makeFakePods(4, 6, newRolloutId, "3", updateRevision)...)
				return pods
			},
			expectBatch: map[string]int{
				"1": 2,
				"2": 4,
				"3": 4,
			},
		},
		{
			name: "unsatisfied, and nothing need to patch",
			cloneSet: func() *appsv1alpha1.CloneSet {
				clone := cloneSetDemo.DeepCopy()
				clone.Status.ExpectedUpdatedReplicas = 2
				clone.Labels[apiinternal.LabelRolloutBatchId] = "1"
				return clone
			},
			pods: func() []client.Object {
				pods := make([]client.Object, 0, 10)
				pods = append(pods, makeFakePods(2, 0, oldRolloutId, "1", currentRevision)...)
				pods = append(pods, makeFakePods(4, 2, oldRolloutId, "2", currentRevision)...)
				pods = append(pods, makeFakePods(4, 6, oldRolloutId, "3", currentRevision)...)
				return pods
			},
			expectBatch: map[string]int{},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cs.cloneSet()).WithObjects(cs.pods()...).Build()
			controller := LabelPatcherReconciler{
				Client: client,
				Scheme: scheme,
			}
			if _, err := controller.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: cloneSetDemo.GetNamespace(), Name: cloneSetDemo.GetName()},
			}); err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			podLister := &v1.PodList{}
			if err := controller.List(context.TODO(), podLister); err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			var logs []string
			realBatch := map[string]int{}
			for i := range podLister.Items {
				pod := &podLister.Items[i]
				logs = append(logs, fmt.Sprintf("%s: %s, %s, %s\n", pod.Name,
					pod.Labels[apps.ControllerRevisionHashLabelKey],
					pod.Labels[apiinternal.LabelRolloutId],
					pod.Labels[apiinternal.LabelRolloutBatchId]))

				if pod.Labels[apiinternal.LabelRolloutId] != newRolloutId {
					continue
				}
				if pod.Labels[apps.ControllerRevisionHashLabelKey] != updateRevision {
					t.Fatalf("pod is not updateRevision but has new rollout-id")
				}
				realBatch[pod.Labels[apiinternal.LabelRolloutBatchId]] += 1
			}
			t.Log(logs)
			if !reflect.DeepEqual(realBatch, cs.expectBatch) {
				t.Fatalf("got unexpected result, expect %v, got %v", cs.expectBatch, realBatch)
			}
		})
	}
}
