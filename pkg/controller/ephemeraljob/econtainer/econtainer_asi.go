package econtainer

import (
	"context"
	"encoding/json"
	"fmt"

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
	"k8s.io/klog/v2"
)

type asiControl struct {
	*appsv1alpha1.EphemeralJob
	kControl *k8sControl
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

// create ephemeral containers in k8s 1.20
//func (k *asiControl) CreateEphemeralContainer(targetPod *v1.Pod) error {
//	kubeclient := kubeclient.GetGenericClient().KubeClient
//	eContainers, err := kubeclient.CoreV1().Pods(targetPod.Namespace).GetEphemeralContainers(context.TODO(), targetPod.Name, metav1.GetOptions{})
//	if err != nil {
//		return err
//	}
//	if eContainers == nil {
//		eContainers = &v1.EphemeralContainers{}
//		eContainers.Namespace = k.Namespace
//		eContainers.Name = k.Name
//		eContainers.EphemeralContainers = k.Spec.Template.EphemeralContainers
//	}
//	ephemeralContainerMaps, _ := getEphemeralContainersMaps(eContainers.EphemeralContainers)
//	for _, e := range k.Spec.Template.EphemeralContainers {
//		if _, ok := ephemeralContainerMaps[e.Name]; ok {
//			klog.Warningf("ephemeral container %s has exist in pod %s", e.Name, targetPod.Name)
//			continue
//		}
//		klog.Infof("ephemeral container %s add to pod %s", e.Name, targetPod.Name)
//		e.Env = append(e.Env, v1.EnvVar{
//			Name:  appsv1alpha1.EphemeralContainerEnvKey,
//			Value: string(k.UID),
//		})
//		eContainers.EphemeralContainers = append(eContainers.EphemeralContainers, e)
//	}
//	_, err = kubeclient.CoreV1().Pods(targetPod.Namespace).UpdateEphemeralContainers(context.TODO(), targetPod.Name, eContainers, metav1.UpdateOptions{})
//	return err
//}

func (k *asiControl) CreateEphemeralContainer(targetPod *v1.Pod) error {
	return k.kControl.CreateEphemeralContainer(targetPod)
}

// RemoveEphemeralContainer depends on asi api-server and kubelet gc.
//func (k *asiControl) RemoveEphemeralContainer(targetPod *v1.Pod) error {
//	kubeclient := kubeclient.GetGenericClient().KubeClient
//	eContainers, err := kubeclient.CoreV1().Pods(targetPod.Namespace).GetEphemeralContainers(context.TODO(), targetPod.Name, metav1.GetOptions{})
//	if err != nil {
//		return err
//	}
//	if eContainers == nil {
//		return nil
//	}
//	ephemeralContainerMaps, _ := getEphemeralContainersMaps(eContainers.EphemeralContainers)
//	for _, e := range k.Spec.Template.EphemeralContainers {
//		if _, ok := ephemeralContainerMaps[e.Name]; ok {
//			eContainers.EphemeralContainers = deleteEphemeralContainers(eContainers.EphemeralContainers, e.Name)
//		}
//	}
//	_, err = kubeclient.CoreV1().Pods(targetPod.Namespace).UpdateEphemeralContainers(context.TODO(), targetPod.Name, eContainers, metav1.UpdateOptions{})
//	return err
//}

// RemoveEphemeralContainer depends on asi api-server and kubelet gc. this will work in both 1.20 and 1.22.
func (k *asiControl) RemoveEphemeralContainer(targetPod *v1.Pod) error {
	needToRemove := getEphemeralContainerNames(k.Spec.Template.EphemeralContainers)
	err := k.removeEphemeralContainer(targetPod, needToRemove)
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
			return k.removeEphemeralContainerLegacy(targetPod, needToRemove)
		}
	}
	return err
}

// removeEphemeralContainerLegacy depends on asi api-server and kubelet gc. for 1.20.
func (k *asiControl) removeEphemeralContainerLegacy(targetPod *v1.Pod, needToRemove sets.String) error {
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
func (k *asiControl) removeEphemeralContainer(targetPod *v1.Pod, needToRemove sets.String) error {
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
