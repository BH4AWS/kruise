package enhancedlivenessprobe

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	utilclient "github.com/openkruise/kruise/pkg/util/client"
)

const (
	concurrentReconciles       = 10
	controllerName             = "livenessprobe-controller"
	defaultTokenBucketRate     = 1
	defaultTokenBucketCapacity = 1024

	podUIDKey     = "pod-uid"
	flagEliveness = "-eliveness" // enhanced liveness
)

var (
	globalBucket *TokenBucket
	utTest       = false
)

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

var _ reconcile.Reconciler = &ReconcileEnhancedLivenessProbe{}

// ReconcileEnhancedLivenessProbe reconciles a Pod object
type ReconcileEnhancedLivenessProbe struct {
	client.Client
	recorder record.EventRecorder
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileEnhancedLivenessProbe {
	globalBucket = NewTokenBucket(time.Duration(defaultTokenBucketRate)*time.Second, defaultTokenBucketCapacity)
	return &ReconcileEnhancedLivenessProbe{
		Client:   utilclient.NewClientFromManager(mgr, controllerName),
		recorder: mgr.GetEventRecorderFor(controllerName),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileEnhancedLivenessProbe) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: concurrentReconciles})
	if err != nil {
		return err
	}

	// watch for changes to NodePodProbe
	if err = c.Watch(&source.Kind{Type: &appsv1alpha1.NodePodProbe{}}, &enqueueRequestForNodePodProbe{}); err != nil {
		return err
	}

	// watch events of CRR
	if err = c.Watch(&source.Kind{Type: &appsv1alpha1.ContainerRecreateRequest{}}, &enqueueRequestForCRR{}); err != nil {
		return err
	}
	return nil
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list
// +kubebuilder:rbac:groups=apps.kruise.io,resources=nodepodprobes,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kruise.io,resources=nodepodprobes/status,verbs=get;list:wacth
// +kubebuilder:rbac:groups=apps.kruise.io,resources=containerrecreaterequests,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=apps.kruise.io,resources=containerrecreaterequests/status,verbs=get;list
func (r *ReconcileEnhancedLivenessProbe) Reconcile(ctx context.Context, request reconcile.Request) (res reconcile.Result, err error) {
	start := time.Now()
	klog.V(3).Infof("Starting to process nodePodProbe %v", request.NamespacedName)
	defer func() {
		if err != nil {
			klog.Warningf("Failed to process nodePodProbe %v, elapsedTime %v, error: %v", request.NamespacedName, time.Since(start), err)
		} else {
			klog.Infof("Finish to process nodePodProbe %v, elapsedTime %v", request.NamespacedName, time.Since(start))
		}
	}()

	err = r.syncNodePodProbe(ctx, request.Namespace, request.Name)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// According to the nodePodProbe, all CRR process will be triggered by this controller.
// 1\the consistent checking between the nodePodProbe and all CRRs.
// 2\the CRRs will be created or deleted according to the nodePodProbe configuration.
func (r *ReconcileEnhancedLivenessProbe) syncNodePodProbe(ctx context.Context, namespace, name string) error {
	getNodePodProbe := appsv1alpha1.NodePodProbe{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &getNodePodProbe)
	if err != nil {
		if errors.IsNotFound(err) {
			// since all CRRs related with the nodePodProbe set the owner reference, we don't need to do anything here.
			return nil
		}
		klog.Errorf("Failed to get nodePodProbe: %v/%v, err: %v", namespace, name, err)
		return err
	}
	// fetch all failed livenessProbe config from this nodePodProbe
	failedLivenessProbePods, err := r.getFailedLivenessProbeFromNodePodProbe(getNodePodProbe)
	if err != nil {
		klog.Errorf("Failed to get failed livenessProbe from node pod probe: %v/%v, err: %v", getNodePodProbe.Namespace, getNodePodProbe.Name, err)
		return err
	}
	cRRsRelatedNodePodProbe, err := r.listAllCRRsRelatedWithNodePodProbe(ctx, getNodePodProbe)
	if err != nil {
		klog.Errorf("Failed to list all CRRs related with nodePodProbe: %v/%v, err: %v", getNodePodProbe.Namespace, getNodePodProbe.Name, err)
		return err
	}

	// When the livenessProbe has been changed, the pod should be recreated.
	// All of CRRs related with the nodePodProbe are checked whether they are consistent with the nodePodProbe configuration.
	for _, item := range cRRsRelatedNodePodProbe.Items {
		podUid := item.Labels[podUIDKey]
		podName := item.Spec.PodName
		podNamespace := item.Namespace
		if !existPodProbeInNodePodProbe(getNodePodProbe, podNamespace, podName, podUid) {
			klog.Warningf("No found pod probe related with the CRR object: %v/%v in nodePodProbe: %v/%v, will delete it",
				item.Namespace, item.Name, getNodePodProbe.Namespace, getNodePodProbe.Name)
			err = r.Client.Delete(ctx, &item)
			if err != nil {
				klog.Errorf("Failed to delete CRR object: %v/%v, err: %v", item.Namespace, item.Name, err)
				return err
			}
			continue
		}
		// the CRRs related with the nodePodProbe are completed, the object should be removed.
		existingCRR, err := r.getCRRByNamespacedName(ctx, item.Namespace, item.Name)
		if err != nil {
			klog.Errorf("Failed to get CRR object by namespacedName: %v/%v, err: %v",
				item.Namespace, item.Name, err)
			return err
		}
		if existingCRR.Status.Phase == appsv1alpha1.ContainerRecreateRequestCompleted ||
			existingCRR.Status.Phase == appsv1alpha1.ContainerRecreateRequestFailed ||
			existingCRR.Status.Phase == appsv1alpha1.ContainerRecreateRequestSucceeded {
			err = r.Client.Delete(ctx, &item)
			if err != nil {
				klog.Errorf("Failed to delete CRR object: %v/%v, this CRR object is completed, err: %v", item.Namespace, item.Name, err)
				return err
			}
		}
	}

	// According to the nodePodProbe configuration, the controller will create the new CRRs.
	for _, item := range failedLivenessProbePods {
		// for nodePodProbe object, only one CRR related with pod will be created.
		existingCRRs, err := r.listCRRObjectByPodUID(ctx, getNodePodProbe.Name,
			item.Pod.Namespace, fmt.Sprintf("%v", item.Pod.UID))
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			klog.Errorf("Failed to get CRR object by label selector: %v/%v/%v, err: %v",
				item.Pod.Namespace, getNodePodProbe.Name, item.Pod.UID, err)
			return err
		}
		if existingCRRs == nil {
			return fmt.Errorf("Failed to list CRR object by label selector: %v/%v/%v, "+
				"the result is empty", item.Pod.Namespace, getNodePodProbe.Name, item.Pod.UID)
		}
		if len(existingCRRs.Items) > 1 {
			return fmt.Errorf("Expect to get one CRR object related with label selector: %v/%v/%v, but got: %v",
				item.Pod.Namespace, getNodePodProbe.Name, item.Pod.UID, len(existingCRRs.Items))
		}
		if len(existingCRRs.Items) == 0 {
			// a new CRR related with pod is generated
			crrGenerated, err := generateCRRByNodePodProbeConfig(getNodePodProbe, item)
			if err != nil {
				klog.Errorf("Failed to generate CRR template by nodePodProbe: %v/%v and pod: %v/%v", getNodePodProbe.Namespace, getNodePodProbe.Name,
					item.Pod.Namespace, item.Pod.Name)
				return err
			}
			// check the global limiting strategy
			if !utTest && !globalBucket.LimitingStrategyAllowed() {
				klog.Warningf("Forbidden to process nodePodProbe: %v/%v because of bucket limit",
					getNodePodProbe.Namespace, getNodePodProbe.Name)
				return nil
			}
			if err := r.Client.Create(ctx, &crrGenerated); err != nil {
				klog.Errorf("Failed to create CRR object: %v/%v, err: %v", crrGenerated.Namespace, crrGenerated.Name, err)
				return err
			}
			continue
		}
		getCRRObj := existingCRRs.Items[0]
		// there is a CRR related with pod nodePodProbe, continue
		// only one CRR can be created by this controller
		// when the old CRR status is completed or succeeded or Failed, the CRR will be deleted.
		if getCRRObj.Status.Phase == appsv1alpha1.ContainerRecreateRequestSucceeded ||
			getCRRObj.Status.Phase == appsv1alpha1.ContainerRecreateRequestCompleted ||
			getCRRObj.Status.Phase == appsv1alpha1.ContainerRecreateRequestFailed {
			err = r.deleteCRRByNamespacedName(ctx, getCRRObj.Namespace, getCRRObj.Name)
			if err != nil {
				klog.Errorf("Failed to delete CRR object: %v/%v, err: %v", getCRRObj.Namespace, getCRRObj.Name, err)
				return err
			}
		}
	}
	return nil
}

// struct for failed livenessProbe pod and the containers name
type podLivenessFailedContainers struct {
	Pod              *v1.Pod  `json:"pod"`
	FailedContainers []string `json:"failedContainers"`
}

// func result is bool type that indicates whether the pod info exists in the nodePodProbe
// exist: true
func existPodProbeInNodePodProbe(nodePodProbe appsv1alpha1.NodePodProbe, podNamespace, podName, podUID string) bool {
	for _, s := range nodePodProbe.Spec.PodProbes {
		if s.Name == podName && s.Namespace == podNamespace && s.UID == podUID {
			return true
		}
	}
	return false
}

// Found all failed livenessProbe config from the nodePodProbe
func (r *ReconcileEnhancedLivenessProbe) getFailedLivenessProbeFromNodePodProbe(nodePodProbe appsv1alpha1.NodePodProbe) ([]podLivenessFailedContainers, error) {
	failedPods := []podLivenessFailedContainers{}
	for _, status := range nodePodProbe.Status.PodProbeStatuses {
		podName := status.Name
		podNamespace := status.Namespace

		getPod := &v1.Pod{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: podNamespace, Name: podName}, getPod)
		if err != nil {
			klog.Errorf("Failed to get pod: %v/%v, err: %v, continue", podNamespace, podName, err)
			return failedPods, err
		}
		if getPod.DeletionTimestamp != nil {
			continue
		}
		if !usingEnhancedLivenessProbe(getPod) {
			continue
		}

		podFailedProbeContainers := []string{}
		// fetch the failed container probe status related with this pod
		for _, s := range status.ProbeStates {
			if s.State != appsv1alpha1.ProbeFailed {
				continue
			}
			if !strings.Contains(s.Name, flagEliveness) {
				continue
			}
			// found the failed probe status in nodePodProbe related with enhanced livenessprobe
			getContainerProbe, found := getPodContainerProbeFromNodePodProbe(nodePodProbe, s.Name, getPod)
			if !found {
				continue
			}
			podFailedProbeContainers = append(podFailedProbeContainers, getContainerProbe.ContainerName)
		}
		if len(podFailedProbeContainers) > 0 {
			failedPods = append(failedPods, podLivenessFailedContainers{
				Pod:              getPod,
				FailedContainers: podFailedProbeContainers,
			})
		}
	}
	return failedPods, nil
}

// According to the pod object and containerProbeName, the func will return the containerProbe configuration in the nodePodProbe
// When there is no containerProbe related with pod and containerProbeName in nodePodProbe, the second return value is false.
func getPodContainerProbeFromNodePodProbe(nodePodProbe appsv1alpha1.NodePodProbe, containerProbeName string, pod *v1.Pod) (appsv1alpha1.ContainerProbe, bool) {
	for _, c := range nodePodProbe.Spec.PodProbes {
		if c.Name == pod.Name && c.Namespace == pod.Namespace && c.UID == fmt.Sprintf("%v", pod.UID) {
			return getContainerProbeByName(c.Probes, containerProbeName)
		}
	}
	return appsv1alpha1.ContainerProbe{}, false
}

func getContainerProbeByName(containerProbes []appsv1alpha1.ContainerProbe, containerProbeName string) (appsv1alpha1.ContainerProbe, bool) {
	for _, c := range containerProbes {
		if c.Name == containerProbeName {
			return c, true
		}
	}
	return appsv1alpha1.ContainerProbe{}, false
}

func usingEnhancedLivenessProbe(pod *v1.Pod) bool {
	return pod.Annotations[alpha1.AnnotationUsingEnhancedLiveness] == "true"
}

func (r *ReconcileEnhancedLivenessProbe) listAllCRRsRelatedWithNodePodProbe(ctx context.Context, nodePodProbe appsv1alpha1.NodePodProbe) (*appsv1alpha1.ContainerRecreateRequestList, error) {
	getCRRs := &appsv1alpha1.ContainerRecreateRequestList{}
	labelSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"owner": controllerName,
			"usage": nodePodProbe.Name,
		},
	})
	if err != nil {
		return getCRRs, err
	}
	if err := r.Client.List(ctx, getCRRs, &client.ListOptions{
		LabelSelector: labelSelector,
	}); err != nil {
		klog.Errorf("Failed to list all CRRs related with nodePodProbe, err: %v", err)
		return getCRRs, err
	}
	return getCRRs, nil
}

func (r *ReconcileEnhancedLivenessProbe) listCRRObjectByPodUID(ctx context.Context, nodePodProbeName, namespace, podUID string) (*appsv1alpha1.ContainerRecreateRequestList, error) {
	getCRRs := appsv1alpha1.ContainerRecreateRequestList{}
	labelSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"owner":   controllerName,
			"usage":   nodePodProbeName,
			podUIDKey: podUID,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := r.Client.List(ctx, &getCRRs, &client.ListOptions{
		LabelSelector: labelSelector,
		Namespace:     namespace,
	}); err != nil {
		klog.Errorf("Failed to list CRRs related with nodePodProbe: %v/%v, podUID: %v, err: %v", namespace, nodePodProbeName, podUID, err)
		return nil, err
	}
	return &getCRRs, nil
}
