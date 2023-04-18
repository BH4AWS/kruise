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

package labelpatcher

import (
	"context"
	"flag"
	"fmt"
	"time"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	clonesetutils "github.com/openkruise/kruise/pkg/controller/cloneset/utils"
	utildiscovery "github.com/openkruise/kruise/pkg/util/discovery"
	"github.com/openkruise/kruise/pkg/util/expectations"
	"github.com/openkruise/kruise/pkg/util/fieldindex"
	"github.com/openkruise/kruise/pkg/util/ratelimiter"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/integer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func init() {
	flag.IntVar(&concurrentReconciles, "label-patcher-workers", concurrentReconciles, "Max concurrent workers for CloneSet controller.")
}

var (
	ExpectationTimeout          = 5 * 60 * time.Second
	concurrentReconciles        = 10
	ResourceVersionExpectations = expectations.NewResourceVersionExpectation()
)

// Add creates a new CloneSet Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	if !utildiscovery.DiscoverGVK(clonesetutils.ControllerKind) {
		return nil
	}
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	reconciler := &LabelPatcherReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return reconciler
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("label-patcher-controller", mgr, controller.Options{
		Reconciler: r, MaxConcurrentReconciles: concurrentReconciles,
		RateLimiter: ratelimiter.DefaultControllerRateLimiter()})
	if err != nil {
		return err
	}

	// Watch for changes to CloneSet
	err = c.Watch(&source.Kind{Type: &appsv1alpha1.CloneSet{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
		CreateFunc: func(createEvent event.CreateEvent) bool {
			cs := createEvent.Object.(*appsv1alpha1.CloneSet)
			return cs.Labels[apiinternal.LabelRolloutId] != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCS := e.ObjectOld.(*appsv1alpha1.CloneSet)
			newCS := e.ObjectNew.(*appsv1alpha1.CloneSet)
			if newCS.Labels[apiinternal.LabelRolloutId] == "" {
				return false
			}
			if newCS.Generation > newCS.Status.ObservedGeneration {
				return false
			}
			if oldCS.Status.ObservedGeneration != newCS.Status.ObservedGeneration {
				return true
			}
			if oldCS.Labels[apiinternal.LabelRolloutId] != newCS.Labels[apiinternal.LabelRolloutId] {
				return true
			}
			if oldCS.Status.ExpectedUpdatedReplicas != newCS.Status.ExpectedUpdatedReplicas {
				return true
			}
			if oldCS.Status.UpdateRevision != newCS.Status.UpdateRevision {
				return true
			}
			return false
		},
	})
	if err != nil {
		return err
	}

	ownerHasRolloutLabel := func(pod *v1.Pod) bool {
		if pod == nil {
			return false
		}
		owner := metav1.GetControllerOfNoCopy(pod)
		if owner == nil {
			return false
		}

		cs := appsv1alpha1.CloneSet{}
		err := mgr.GetClient().Get(context.TODO(), types.NamespacedName{
			Name: owner.Name, Namespace: pod.GetNamespace(),
		}, &cs)
		if err != nil {
			return false
		}
		return cs.Labels[apiinternal.LabelRolloutId] != ""
	}

	return c.Watch(&source.Kind{Type: &v1.Pod{}}, &handler.EnqueueRequestForOwner{IsController: true, OwnerType: &appsv1alpha1.CloneSet{}}, predicate.Funcs{
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			pod := deleteEvent.Object.(*v1.Pod)
			ResourceVersionExpectations.Observe(pod)
			return false
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
		CreateFunc: func(createEvent event.CreateEvent) bool {
			pod := createEvent.Object.(*v1.Pod)
			ResourceVersionExpectations.Observe(pod)
			return ownerHasRolloutLabel(createEvent.Object.(*v1.Pod))
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldPod, oldOK := updateEvent.ObjectOld.(*v1.Pod)
			newPod, newOK := updateEvent.ObjectNew.(*v1.Pod)
			if !oldOK || !newOK || oldPod == nil || newPod == nil {
				return false
			}
			ResourceVersionExpectations.Observe(newPod)
			if clonesetutils.GetShortHash(oldPod.Labels[apps.ControllerRevisionHashLabelKey]) ==
				clonesetutils.GetShortHash(newPod.Labels[apps.ControllerRevisionHashLabelKey]) {
				return false
			}
			return ownerHasRolloutLabel(updateEvent.ObjectNew.(*v1.Pod))
		},
	})
}

var _ reconcile.Reconciler = &LabelPatcherReconciler{}

// LabelPatcherReconciler reconciles a LabelPatcher object
type LabelPatcherReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LabelPatcher object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *LabelPatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cloneSet := &appsv1alpha1.CloneSet{}
	if err := r.Get(context.TODO(), req.NamespacedName, cloneSet); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if cloneSet.Labels[apiinternal.LabelRolloutId] == "" {
		return ctrl.Result{}, nil
	}

	if cloneSet.Generation > cloneSet.Status.ObservedGeneration {
		return ctrl.Result{}, nil
	}

	pods, err := r.getOwnedPods(cloneSet)
	if err != nil || len(pods) == 0 {
		return ctrl.Result{}, err
	}

	var patched int32
	var desired []*v1.Pod
	for _, pod := range pods {
		ResourceVersionExpectations.Observe(pod)
		if isSatisfied, unsatisfiedDuration := ResourceVersionExpectations.IsSatisfied(pod); !isSatisfied {
			if unsatisfiedDuration >= ExpectationTimeout {
				klog.Warningf("LabelPatcher: expectation unsatisfied overtime for %v, wait for pod %v updating, timeout=%v", req.String(), pod.Name, unsatisfiedDuration)
				return reconcile.Result{}, nil
			}
			klog.V(4).Infof("LabelPatcher: not satisfied resourceVersion for %v, wait for pod %v updating", req.String(), pod.Name)
			return reconcile.Result{RequeueAfter: ExpectationTimeout - unsatisfiedDuration}, nil
		}

		if cloneSet.Labels[apiinternal.LabelRolloutId] == pod.Labels[apiinternal.LabelRolloutId] {
			patched++
		} else if clonesetutils.EqualToRevisionHash("", pod, cloneSet.Status.UpdateRevision) {
			desired = append(desired, pod)
		}
	}

	rest := integer.Int32Min(cloneSet.Status.ExpectedUpdatedReplicas-patched, int32(len(desired)))
	if rest <= 0 {
		// Satisfied.
		return ctrl.Result{}, nil
	}

	for i := int32(0); i < rest; i++ {
		body := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s","%s":"%s"}}}`,
			apiinternal.LabelRolloutId, cloneSet.Labels[apiinternal.LabelRolloutId],
			apiinternal.LabelRolloutBatchId, cloneSet.Labels[apiinternal.LabelRolloutBatchId])

		pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: desired[i].Name, Namespace: desired[i].Namespace}}
		if err = r.Patch(context.TODO(), pod, client.RawPatch(types.StrategicMergePatchType, []byte(body))); err != nil {
			return ctrl.Result{}, err
		}
		ResourceVersionExpectations.Expect(pod)
	}

	return ctrl.Result{}, nil
}

func (r *LabelPatcherReconciler) getOwnedPods(cs *appsv1alpha1.CloneSet) ([]*v1.Pod, error) {
	opts := &client.ListOptions{
		Namespace:     cs.Namespace,
		FieldSelector: fields.SelectorFromSet(fields.Set{fieldindex.IndexNameForOwnerRefUID: string(cs.UID)}),
	}
	active, _, err := clonesetutils.GetActiveAndInactivePods(r.Client, opts)
	return active, err
}
