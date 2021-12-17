package econtainer

import (
	"context"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	kubeclient "github.com/openkruise/kruise/pkg/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	kubeclient := kubeclient.GetGenericClient().KubeClient
	eContainers, err := kubeclient.CoreV1().Pods(targetPod.Namespace).GetEphemeralContainers(context.TODO(), targetPod.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if eContainers == nil {
		eContainers = &v1.EphemeralContainers{}
		eContainers.Namespace = k.Namespace
		eContainers.Name = k.Name
		eContainers.Labels = map[string]string{
			appsv1alpha1.EphemeralContainerCreateByJob: k.Name,
		}
		eContainers.EphemeralContainers = k.Spec.Template.EphemeralContainers
	}

	ephemeralContainerMaps, _ := getEphemeralContainersMaps(eContainers.EphemeralContainers)
	for _, e := range k.Spec.Template.EphemeralContainers {
		if _, ok := ephemeralContainerMaps[e.Name]; ok {
			klog.Warningf("ephemeral container %s has exist in pod %s", e.Name, targetPod.Name)
			continue
		}

		klog.Infof("ephemeral container %s add to pod %s", e.Name, targetPod.Name)
		e.Env = append(e.Env, v1.EnvVar{
			Name:  appsv1alpha1.EphemeralContainerEnvKey,
			Value: string(k.UID),
		})
		eContainers.EphemeralContainers = append(eContainers.EphemeralContainers, e)
	}

	_, err = kubeclient.CoreV1().Pods(targetPod.Namespace).UpdateEphemeralContainers(context.TODO(), targetPod.Name, eContainers, metav1.UpdateOptions{})
	return err
}

// RemoveEphemeralContainer depends on asi api-server and kubelet gc.
func (k *asiControl) RemoveEphemeralContainer(targetPod *v1.Pod) error {
	kubeclient := kubeclient.GetGenericClient().KubeClient
	eContainers, err := kubeclient.CoreV1().Pods(targetPod.Namespace).GetEphemeralContainers(context.TODO(), targetPod.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if eContainers == nil {
		return nil
	}

	ephemeralContainerMaps, _ := getEphemeralContainersMaps(eContainers.EphemeralContainers)

	for _, e := range k.Spec.Template.EphemeralContainers {
		if _, ok := ephemeralContainerMaps[e.Name]; ok {
			eContainers.EphemeralContainers = deleteEphemeralContainers(eContainers.EphemeralContainers, e.Name)
		}
	}

	_, err = kubeclient.CoreV1().Pods(targetPod.Namespace).UpdateEphemeralContainers(context.TODO(), targetPod.Name, eContainers, metav1.UpdateOptions{})
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
