package enhancedlivenessprobemapping

import (
	"context"
	"encoding/json"
	"fmt"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/util"
	utilclient "github.com/openkruise/kruise/pkg/util/client"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

const (
	concurrentReconciles                   = 10
	controllerName                         = "enhancedlivenessprobemapping-controller"
	AnnotationUsingEnhancedLiveness        = "apps.kruise.io/using-enhanced-liveness"
	AnnotationNativeLivenessContext        = "apps.kruise.io/livenessprobe-context"
	AnnotationLivenessProbeConfigConverted = "apps.kruise.io/livnessprobe-config-converted"

	AddNodeProbeConfigOpType = "addNodeProbe"
	DelNodeProbeConfigOpType = "delNodeProbe"
)

type containerLivenessProbe struct {
	Name          string   `json:"name"`
	LivenessProbe v1.Probe `json:"livenessProbe"`
}

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

var _ reconcile.Reconciler = &ReconcileEnhancedLivenessProbeMapping{}

// ReconcileContainerLaunchPriority reconciles a Pod object
type ReconcileEnhancedLivenessProbeMapping struct {
	client.Client
	recorder record.EventRecorder
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileEnhancedLivenessProbeMapping {
	return &ReconcileEnhancedLivenessProbeMapping{
		Client:   utilclient.NewClientFromManager(mgr, controllerName),
		recorder: mgr.GetEventRecorderFor(controllerName),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileEnhancedLivenessProbeMapping) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: concurrentReconciles})
	if err != nil {
		return err
	}

	// watch events of pod
	err = c.Watch(&source.Kind{Type: &v1.Pod{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			pod := e.Object.(*v1.Pod)
			return usingEnhancedLivenessProbe(pod) && getRawEnhancedLivenessProbeConfig(pod) != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			pod := e.ObjectNew.(*v1.Pod)
			return usingEnhancedLivenessProbe(pod) && getRawEnhancedLivenessProbeConfig(pod) != ""
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			pod := e.Object.(*v1.Pod)
			return usingEnhancedLivenessProbe(pod) && getRawEnhancedLivenessProbeConfig(pod) != ""
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kruise.io,resources=nodepodprobes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kruise.io,resources=nodepodprobes/status,verbs=get;update;patch

func (r *ReconcileEnhancedLivenessProbeMapping) Reconcile(_ context.Context, request reconcile.Request) (res reconcile.Result, err error) {
	start := time.Now()
	klog.V(3).Infof("Starting to process Pod %v", request.NamespacedName)
	defer func() {
		if err != nil {
			klog.Warningf("Failed to process Pod %v, elapsedTime %v, error: %v", request.NamespacedName, time.Since(start), err)
		} else {
			klog.Infof("Finish to process Pod %v, elapsedTime %v", request.NamespacedName, time.Since(start))
		}
	}()

	getPod := &v1.Pod{}
	if err = r.Get(context.TODO(), request.NamespacedName, getPod); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get pod %v: %v", request.NamespacedName, err)
	}

	if getPod.DeletionTimestamp == nil && podLivenessProbeConfigConverted(getPod) {
		return reconcile.Result{}, nil
	}

	if getPod.DeletionTimestamp == nil {
		if !controllerutil.ContainsFinalizer(getPod, "") {
			if err := util.UpdateFinalizer(r.Client, getPod, util.AddFinalizerOpType, ""); err != nil {
				return reconcile.Result{}, err
			}
		}
		if err := r.addOrRemoveNodePodProbeConfig(getPod, AddNodeProbeConfigOpType); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.patchPodAnnotationsByKeyVal(getPod, AnnotationLivenessProbeConfigConverted, "true"); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		if err := r.addOrRemoveNodePodProbeConfig(getPod, DelNodeProbeConfigOpType); err != nil {
			return reconcile.Result{}, err
		}
		if controllerutil.ContainsFinalizer(getPod, "") {
			if err := util.UpdateFinalizer(r.Client, getPod, util.RemoveFinalizerOpType, ""); err != nil {
				return reconcile.Result{}, err
			}
		}
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileEnhancedLivenessProbeMapping) addOrRemoveNodePodProbeConfig(pod *v1.Pod, op string) error {
	podRawLivenessProbeConfig := getRawEnhancedLivenessProbeConfig(pod)
	podLivenessProbeConfig := []containerLivenessProbe{}
	if err := json.Unmarshal([]byte(podRawLivenessProbeConfig), &podLivenessProbeConfig); err != nil {
		return err
	}

	podNodeName := r.GetPodNodeName(pod)
	nppClone := &appsv1alpha1.NodePodProbe{}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: podNodeName}, nppClone); err != nil {
			klog.Errorf("error getting updated npp %s from client", podNodeName)
		}
		podNewNodeProbeSpec := nppClone.Spec.DeepCopy()
		podNewProbe := []appsv1alpha1.PodProbe{}
		if op == DelNodeProbeConfigOpType {
			for index, _ := range nppClone.Spec.PodProbes {
				podProbe := podNewNodeProbeSpec.PodProbes[index]
				if podProbe.Name == pod.Name && podProbe.Namespace == pod.Namespace {
					continue
				}
				podNewProbe = append(podNewProbe, podProbe)
			}
			podNewNodeProbeSpec.PodProbes = podNewProbe
		}

		if op == AddNodeProbeConfigOpType {
			isHit := false
			for index, _ := range nppClone.Spec.PodProbes {
				podProbe := &podNewNodeProbeSpec.PodProbes[index]
				if podProbe.Name == pod.Name && podProbe.Namespace == pod.Namespace {
					isHit = true
					newPodContainersProbes := []appsv1alpha1.ContainerProbe{}
					for _, p := range podLivenessProbeConfig {
						cProbe := appsv1alpha1.ContainerProbe{}
						cProbe.Name = p.Name
						cProbe.Probe.Probe = p.LivenessProbe
						newPodContainersProbes = append(newPodContainersProbes, cProbe)
					}
					podProbe.Probes = newPodContainersProbes
				}
			}
			if !isHit {
				if len(podNewNodeProbeSpec.PodProbes) == 0 {
					podNewNodeProbeSpec.PodProbes = []appsv1alpha1.PodProbe{}
				}
				newPodProbe := appsv1alpha1.PodProbe{}
				newPodContainersProbes := []appsv1alpha1.ContainerProbe{}
				for _, p := range podLivenessProbeConfig {
					cProbe := appsv1alpha1.ContainerProbe{}
					cProbe.Name = p.Name
					cProbe.Probe.Probe = p.LivenessProbe
					newPodContainersProbes = append(newPodContainersProbes, cProbe)
				}
				newPodProbe.Name = pod.Name
				newPodProbe.Namespace = pod.Namespace
				newPodProbe.UID = string(pod.UID)
				newPodProbe.Probes = newPodContainersProbes
				podNewNodeProbeSpec.PodProbes = append(podNewNodeProbeSpec.PodProbes, newPodProbe)
			}
		}

		if reflect.DeepEqual(podNewNodeProbeSpec, nppClone.Spec) {
			return nil
		}
		nppClone.Spec = *podNewNodeProbeSpec
		return r.Client.Update(context.TODO(), nppClone)
	})
	if err != nil {
		klog.Errorf("NodePodProbe update NodePodProbe(%s) failed:%s", podNodeName, err.Error())
		return err
	}

	return nil
}

func (r *ReconcileEnhancedLivenessProbeMapping) patchPodAnnotationsByKeyVal(pod *v1.Pod, key, value string) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`, key, value))
	if err := r.Patch(context.TODO(), pod, client.RawPatch(types.StrategicMergePatchType, patch)); err != nil {
		klog.Errorf("Failed to patch pod: %v/%v with annotations: %v=%v, err: %v", pod.Namespace, pod.Name, key, value, err)
		return err
	}
	return nil
}

func (r *ReconcileEnhancedLivenessProbeMapping) GetPodNodeName(pod *v1.Pod) string {
	return pod.Spec.NodeName
}

func usingEnhancedLivenessProbe(pod *v1.Pod) bool {
	if pod.Annotations[AnnotationUsingEnhancedLiveness] == "true" {
		return true
	}
	return false
}

func getRawEnhancedLivenessProbeConfig(pod *v1.Pod) string {
	return pod.Annotations[AnnotationNativeLivenessContext]
}

func podLivenessProbeConfigConverted(pod *v1.Pod) bool {
	if pod.Annotations[AnnotationLivenessProbeConfigConverted] == "true" {
		return true
	}
	return false
}
