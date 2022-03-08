package econtainer

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	kubeclient "github.com/openkruise/kruise/pkg/client"
	"github.com/openkruise/kruise/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/klog/v2"
)

type asiControl struct {
	*appsv1alpha1.EphemeralJob
}

var _ EphemeralContainerInterface = &asiControl{}

func (k *asiControl) CalculateEphemeralContainerStatus(targetPods []*v1.Pod, status *appsv1alpha1.EphemeralJobStatus) error {

	var success, failed, running, waiting int32
	for _, pod := range targetPods {
		state, err := parseEphemeralPodStatus(k.EphemeralJob, pod.Status.EphemeralContainerStatuses)
		if err != nil {
			return err
		}

		switch state {
		case v1.PodSucceeded:
			success++
		case v1.PodFailed:
			failed++
		case v1.PodRunning:
			running++
		case v1.PodPending:
			waiting++
		}
	}

	status.Succeeded = success
	status.Failed = failed
	status.Running = running
	status.Waiting = waiting

	return nil
}

func (k *asiControl) GetEphemeralContainersStatus(target *v1.Pod) []v1.ContainerStatus {
	return target.Status.EphemeralContainerStatuses
}

func (k *asiControl) GetEphemeralContainers(targetPod *v1.Pod) []v1.EphemeralContainer {
	return targetPod.Spec.EphemeralContainers
}

func (k *asiControl) CreateEphemeralContainer(targetPod *v1.Pod) error {
	oldPodJS, _ := json.Marshal(targetPod)
	newPod := targetPod.DeepCopy()
	for i := range k.Spec.Template.EphemeralContainers {
		ec := k.Spec.Template.EphemeralContainers[i].DeepCopy()
		ec.Env = append(ec.Env, v1.EnvVar{
			Name:  appsv1alpha1.EphemeralContainerEnvKey,
			Value: string(k.UID),
		})
		newPod.Spec.EphemeralContainers = append(newPod.Spec.EphemeralContainers, *ec)
	}
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

// RemoveEphemeralContainer depends on asi api-server and kubelet gc.
func (k *asiControl) RemoveEphemeralContainer(targetPod *v1.Pod) error {
	oldPodJS, _ := json.Marshal(targetPod)
	newPod := targetPod.DeepCopy()

	containersInJob, _ := getEphemeralContainersMaps(k.Spec.Template.EphemeralContainers)

	var ephemeralContainers []v1.EphemeralContainer
	for i := range newPod.Spec.EphemeralContainers {
		c := &newPod.Spec.EphemeralContainers[i]
		if _, ok := containersInJob[c.Name]; ok {
			continue
		}
		ephemeralContainers = append(ephemeralContainers, *c)
	}
	newPod.Spec.EphemeralContainers = ephemeralContainers
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

func deleteEphemeralContainers(ephemeralContainers []v1.EphemeralContainer, name string) []v1.EphemeralContainer {
	if len(ephemeralContainers) == 0 {
		return ephemeralContainers
	}

	for id := range ephemeralContainers {
		if ephemeralContainers[id].Name == name {
			return append(ephemeralContainers[:id], ephemeralContainers[id+1:]...)
		}
	}

	return ephemeralContainers
}

// UpdateEphemeralContainer is not support before kubernetes v1.23
func (k *asiControl) UpdateEphemeralContainer(target *v1.Pod) error {
	return nil
}
