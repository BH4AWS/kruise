/*
Copyright 2022.

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

package mutating

import (
	"context"
	"path"

	"github.com/openkruise/kruise/pkg/util"
	"github.com/openkruise/kruise/pkg/util/containerexitpriority"
	"github.com/openkruise/kruise/pkg/utilasi"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// mutate pod based on container exit priority
func (h *PodCreateHandler) containerExitPriorityMutatingPod(ctx context.Context, req admission.Request, pod *corev1.Pod) (skip bool, err error) {
	skip = true
	// only handler Pod Object Request
	if len(req.AdmissionRequest.SubResource) > 0 || req.AdmissionRequest.Resource.Resource != "pods" {
		return
	} else if pod.Annotations[containerexitpriority.AutoInjectContainerExitPriorityPreStopAnnotation] != "true" {
		return
	}
	// asi, in-place update inject exit priority configuration
	if req.AdmissionRequest.Operation == admissionv1.Update {
		oldPod := new(corev1.Pod)
		if err := h.Decoder.Decode(
			admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: req.AdmissionRequest.OldObject}},
			oldPod); err != nil {
			return false, err
		}
		if !utilasi.IsPodInplaceUpgrading(pod, oldPod) {
			return
		}
	}
	// don't contain exit priority containers
	containers := containerexitpriority.ListExitPriorityContainers(pod)
	if len(containers) == 0 {
		return
	}

	// inject exit priority configuration
	injectSet := sets.NewString()
	for _, container := range containers {
		injectSet.Insert(container.Name)
	}
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if !injectSet.Has(container.Name) {
			continue
		}
		// for Update scenarios
		if vm := util.GetContainerVolumeMount(container, containerexitpriority.KruiseDaemonShareRootVolume); vm == nil {
			skip = false
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:        "kruise-container-exit-info",
				MountPath:   containerexitpriority.KruiseDaemonShareRootVolume,
				SubPathExpr: path.Join("$(POD_UID)", container.Name),
			})
		}
		if env := util.GetContainerEnvVar(container, "POD_UID"); env == nil {
			skip = false
			container.Env = append(container.Env, corev1.EnvVar{
				Name: "POD_UID",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.uid",
					},
				},
			})
		}
		if container.Lifecycle != nil && container.Lifecycle.PreStop != nil && container.Lifecycle.PreStop.Exec != nil &&
			len(container.Lifecycle.PreStop.Exec.Command) > 0 && container.Lifecycle.PreStop.Exec.Command[0] == path.Join(containerexitpriority.KruiseDaemonShareRootVolume, "entrypoint") {
			continue
		}

		skip = false
		var originCmd []string
		// todo, preStop http
		if container.Lifecycle != nil && container.Lifecycle.PreStop != nil && container.Lifecycle.PreStop.Exec != nil {
			originCmd = container.Lifecycle.PreStop.Exec.Command
		} else {
			if container.Lifecycle == nil {
				container.Lifecycle = &corev1.Lifecycle{}
			}
			if container.Lifecycle.PreStop == nil {
				container.Lifecycle.PreStop = &corev1.Handler{}
			}
			if container.Lifecycle.PreStop.Exec == nil {
				container.Lifecycle.PreStop.Exec = &corev1.ExecAction{}
			}
		}
		command := []string{
			path.Join(containerexitpriority.KruiseDaemonShareRootVolume, "entrypoint"),
			"-wait-container-exit",
		}
		if len(originCmd) != 0 {
			command = append(command, []string{"-entrypoint", originCmd[0]}...)
			if len(originCmd) > 1 {
				command = append(command, "--")
				command = append(command, originCmd[1:]...)
			}
		}
		container.Lifecycle.PreStop.Exec.Command = command
	}
	if volume := util.GetPodVolume(pod, "kruise-container-exit-info"); volume == nil {
		skip = false
		hostPathType := corev1.HostPathDirectoryOrCreate
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "kruise-container-exit-info",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: path.Join(containerexitpriority.KruiseDaemonShareRootVolume, "pods"),
					Type: &hostPathType,
				},
			},
		})
	}
	klog.V(3).Infof("inject pod(%s/%s) container exit priority configuration", pod.Namespace, pod.Name)
	return
}
