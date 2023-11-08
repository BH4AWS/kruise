package econtainer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	kubeclient "github.com/openkruise/kruise/pkg/client"
	"github.com/openkruise/kruise/pkg/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

const (
	// reference doc: https://aliyuque.antfin.com/asidocs/xg2eea/gbbhkafkutrrpzmm
	DeletingEphemeralContainersAnnoKey = "alibabacloud.com/deleting-ephemeral-containers"
	ManagingEphemeralContainersAnnoKey = "alibabacloud.com/managing-ephemeral-containers"
)

var defaultRetryDuration = 2 * time.Second

type asiControl struct {
	*appsv1alpha1.EphemeralJob
	kControl *k8sControl
	recorder record.EventRecorder
}

var _ EphemeralContainerInterface = &asiControl{}

func NewAsiControl(job *appsv1alpha1.EphemeralJob) EphemeralContainerInterface {
	asi := &asiControl{EphemeralJob: job}
	asi.kControl = &k8sControl{job}
	return asi
}

func (k *asiControl) CalculateEphemeralContainerStatus(targetPods []*v1.Pod, status *appsv1alpha1.EphemeralJobStatus) error {
	return k.kControl.CalculateEphemeralContainerStatus(targetPods, status)
}

func (k *asiControl) GetEphemeralContainersStatus(target *v1.Pod) []v1.ContainerStatus {
	return k.kControl.GetEphemeralContainersStatus(target)
}

func (k *asiControl) GetEphemeralContainers(targetPod *v1.Pod) []v1.EphemeralContainer {
	return k.kControl.GetEphemeralContainers(targetPod)
}

func (k *asiControl) CreateEphemeralContainer(targetPod *v1.Pod) error {
	return k.kControl.CreateEphemeralContainer(targetPod)
}

func (k *asiControl) ContainsEphemeralContainer(target *v1.Pod) (bool, bool) {
	deletingSet := sets.NewString(strings.Split(target.Annotations[DeletingEphemeralContainersAnnoKey], ",")...)
	for _, ec := range k.Spec.Template.EphemeralContainers {
		if deletingSet.Has(ec.Name) {
			return true, true
		}
	}
	return k.kControl.ContainsEphemeralContainer(target)
}

func (k *asiControl) RemoveEphemeralContainer(pod *v1.Pod) (*time.Duration, error) {
	needToRemove := getEphemeralContainerNames(k.Spec.Template.EphemeralContainers)
	managingSet := sets.NewString(strings.Split(pod.Annotations[ManagingEphemeralContainersAnnoKey], ",")...)

	// any ephemeral container still running
	running := func() bool {
		for _, status := range pod.Status.EphemeralContainerStatuses {
			if !needToRemove.Has(status.Name) {
				continue
			}
			if status.State.Terminated == nil {
				return true
			}
		}
		return false
	}()
	if running {
		klog.Warningf("E-containers in pod %s are still running when deleting ephemeralJob %s", klog.KObj(pod), klog.KObj(k.EphemeralJob))
		return &defaultRetryDuration, nil
	}

	// any ephemeral container still is managed by kubelet
	runtimeRemoved := func() bool {
		for ec := range needToRemove {
			if managingSet.Has(ec) {
				return false
			}
		}
		return true
	}()

	// all ephemeral containers have been removed from spec and status of pod
	serverRemoved := func() bool {
		for _, status := range pod.Status.EphemeralContainerStatuses {
			if needToRemove.Has(status.Name) {
				return false
			}
		}
		for _, ec := range pod.Spec.EphemeralContainers {
			if needToRemove.Has(ec.Name) {
				return false
			}
		}
		return true
	}()

	switch {
	case !runtimeRemoved && !serverRemoved:
		klog.V(4).Infof("Notifying kubelet to remove e-containers in pod %s for ephemeralJob %s", klog.KObj(pod), klog.KObj(k.EphemeralJob))
		return &defaultRetryDuration, k.patchDeletingAnnotation(pod, needToRemove, "Add")

	case runtimeRemoved && !serverRemoved:
		klog.V(4).Infof("E-containers have been removed in pod %s by kubelet for ephemeralJob %s", klog.KObj(pod), klog.KObj(k.EphemeralJob))
		return &defaultRetryDuration, k.removeEphemeralContainer(pod, needToRemove)

	case runtimeRemoved && serverRemoved:
		klog.V(4).Infof("E-containers have been removed in pod %s by api-server for ephemeralJob %s", klog.KObj(pod), klog.KObj(k.EphemeralJob))
		return nil, k.patchDeletingAnnotation(pod, needToRemove, "Del")

	case !runtimeRemoved && serverRemoved:
		err := fmt.Errorf("e-containers have been removed by api-server but not kubelet in pod %s for ephemeralJob %s", klog.KObj(pod), klog.KObj(k.EphemeralJob))
		return nil, err
	}
	return nil, nil
}

// removeEphemeralContainer depends on asi api-server and kubelet gc. this will work in both 1.20 and 1.22.
func (k *asiControl) removeEphemeralContainer(targetPod *v1.Pod, needToRemove sets.String) error {
	err := k.doRemoveEphemeralContainer(targetPod, needToRemove)
	if err != nil {
		// The apiserver will return a 404 when the EphemeralContainers feature is disabled because the `/ephemeralcontainers` subresource
		// is missing. Unlike the 404 returned by a missing pod, the status details will be empty.
		if serr, ok := err.(*errors.StatusError); ok && serr.Status().Reason == metav1.StatusReasonNotFound && serr.ErrStatus.Details.Name == "" {
			klog.Errorf("ephemeral containers are disabled for this cluster (error from server: %q).", err)
			return nil
		}

		// The Kind used for the /ephemeralcontainers subresource changed in 1.22. When presented with an unexpected
		// Kind the api server will respond with a not-registered error. When this happens we can optimistically try
		// using the old API.
		if runtime.IsNotRegisteredError(err) {
			klog.V(1).Infof("Falling back to legacy ephemeral container API because server returned error: %v", err)
			return k.doRemoveEphemeralContainerLegacy(targetPod, needToRemove)
		}
	}
	return err
}

// removeEphemeralContainerLegacy depends on asi api-server and kubelet gc. for 1.20.
func (k *asiControl) doRemoveEphemeralContainerLegacy(targetPod *v1.Pod, needToRemove sets.String) error {
	kubeClient := kubeclient.GetGenericClient().KubeClient
	newPod, err := kubeClient.CoreV1().Pods(targetPod.Namespace).Get(context.TODO(), targetPod.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	reserved := deleteEphemeralContainers(newPod.Spec.EphemeralContainers, needToRemove)
	// We no longer have the v1.EphemeralContainers Kind since it was removed in 1.22, but
	// we can present a JSON 6902 patch that the api server will apply.
	patch, err := json.Marshal([]map[string]interface{}{{
		"op":    "replace",
		"path":  "/ephemeralContainers",
		"value": reserved,
	}})
	if err != nil {
		klog.Errorf("error creating JSON 6902 patch for old /ephemeralcontainers API: %s", err)
		return nil
	}

	_, err = kubeClient.CoreV1().Pods(targetPod.Namespace).
		Patch(context.TODO(), targetPod.Name, types.JSONPatchType, patch, metav1.PatchOptions{}, "ephemeralcontainers")
	return err
}

// removeEphemeralContainerLegacy depends on asi api-server and kubelet gc. for 1.22.
func (k *asiControl) doRemoveEphemeralContainer(targetPod *v1.Pod, needToRemove sets.String) error {
	oldPodJS, _ := json.Marshal(targetPod)
	newPod := targetPod.DeepCopy()
	newPod.Spec.EphemeralContainers = deleteEphemeralContainers(newPod.Spec.EphemeralContainers, needToRemove)
	newPodJS, _ := json.Marshal(newPod)

	patch, err := strategicpatch.CreateTwoWayMergePatch(oldPodJS, newPodJS, &v1.Pod{})
	if err != nil {
		return fmt.Errorf("error creating patch to add ephemeral containers: %v", err)
	}

	klog.Infof("EphemeralJob %s/%s tries to patch containers to pod %s: %v", k.Namespace, k.Name, targetPod.Name, util.DumpJSON(patch))

	kubeClient := kubeclient.GetGenericClient().KubeClient
	_, err = kubeClient.CoreV1().Pods(targetPod.Namespace).
		Patch(context.TODO(), targetPod.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{}, "ephemeralcontainers")
	return err
}

func (k *asiControl) patchDeletingAnnotation(pod *v1.Pod, needToRemove sets.String, op string) error {
	var patchBody string
	switch op {
	case "Add":
		modified := addDeletingAnnotation(pod, needToRemove)
		if modified == pod.Annotations[DeletingEphemeralContainersAnnoKey] {
			return nil
		}
		patchBody = fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`,
			DeletingEphemeralContainersAnnoKey, modified)
	case "Del":
		modified := delDeletingAnnotation(pod, needToRemove)
		if modified == pod.Annotations[DeletingEphemeralContainersAnnoKey] {
			return nil
		}
		patchBody = fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`,
			DeletingEphemeralContainersAnnoKey, modified)
	default:
		panic("unknown 'op' parameter in patchDeletingAnnotation func")
	}

	kubeClient := kubeclient.GetGenericClient().KubeClient
	_, err := kubeClient.CoreV1().Pods(pod.Namespace).
		Patch(context.TODO(), pod.Name, types.StrategicMergePatchType, []byte(patchBody), metav1.PatchOptions{})
	return err
}

func addDeletingAnnotation(pod *v1.Pod, needToRemove sets.String) string {
	deletingSet := sets.NewString(strings.Split(pod.Annotations[DeletingEphemeralContainersAnnoKey], ",")...).Delete("")
	return strings.Join(deletingSet.Union(needToRemove).List(), ",")
}

func delDeletingAnnotation(pod *v1.Pod, needToRemove sets.String) string {
	deletingSet := sets.NewString(strings.Split(pod.Annotations[DeletingEphemeralContainersAnnoKey], ",")...).Delete("")
	return strings.Join(deletingSet.Difference(needToRemove).List(), ",")
}

func deleteEphemeralContainers(ephemeralContainers []v1.EphemeralContainer, needToRemove sets.String) []v1.EphemeralContainer {
	if len(ephemeralContainers) == 0 {
		return ephemeralContainers
	}

	var filteredEContainers []v1.EphemeralContainer
	for id := range ephemeralContainers {
		if needToRemove.Has(ephemeralContainers[id].Name) {
			continue
		}
		filteredEContainers = append(filteredEContainers, ephemeralContainers[id])
	}

	return filteredEContainers
}

// UpdateEphemeralContainer is not support before kubernetes v1.23
func (k *asiControl) UpdateEphemeralContainer(target *v1.Pod) error {
	return nil
}

func getEphemeralContainerNames(eContainers []v1.EphemeralContainer) sets.String {
	names := sets.NewString()
	for _, e := range eContainers {
		names.Insert(e.Name)
	}
	return names
}
