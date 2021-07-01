/*
Copyright 2021 The Kruise Authors.

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

package sidecarcontrol

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	"github.com/openkruise/kruise/pkg/util"
	"github.com/openkruise/kruise/pkg/utilasi"
	"github.com/openkruise/kruise/pkg/utilasi/commontypes"
	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

const (
	LabelSidecarSetMode = "sidecarset.kruise.io/mode"
	SidecarSetASI       = "asi"

	SidecarInjectOnInplaceUpdate = "sidecarset.kruise.io/inject-on-inplace-update"
	/*
	   sidecar 重要度，指本 sidecar 的健康状态是否影响业务服务, 是否影响pod ready判断
	   true: sidecar 不影响业务，调度和升级保护不需要检查 sidecar 状态
	   false: sidecar 影响业务，调度和升级保护需要检查 sidecar 状态
	*/
	EnvIgnorePodReady       = "SIGMA_IGNORE_READY"
	IgnorePodReadyValueTrue = "true"
	//proxy kruise to asi
	SidecarSetProxyLabels               = "sidecarset.kruise.io/proxy"
	SidecarSetMigrationStateAnnotations = "sidecarset.kruise.io/migration-state"
)

type asiControl struct {
	*appsv1alpha1.SidecarSet
}

func (c *asiControl) GetSidecarset() *appsv1alpha1.SidecarSet {
	return c.SidecarSet
}

func (c *asiControl) IsActiveSidecarSet() bool {
	//proxy kruise to asi
	return c.GetSidecarset().Labels[SidecarSetProxyLabels] != "true" &&
		c.GetSidecarset().Annotations[SidecarSetMigrationStateAnnotations] != "running"
}

func (c *asiControl) UpgradeSidecarContainer(sidecarContainer *appsv1alpha1.SidecarContainer, pod *v1.Pod) *v1.Container {
	var nameToUpgrade, otherContainer, beforeJson string
	var isDadiImage bool
	if IsHotUpgradeContainer(sidecarContainer) {
		nameToUpgrade, otherContainer = findContainerToHotUpgrade(sidecarContainer, pod, c)
		oldContainer := util.GetContainer(otherContainer, pod).DeepCopy()
		oldContainer.Name = nameToUpgrade
		//兼容dadi镜像场景
		isDadiImage = utilasi.IsDadiImageName(oldContainer.Image)
		beforeJson = dumpJsonContainerForCompare(isDadiImage, oldContainer)
	} else {
		nameToUpgrade = sidecarContainer.Name
		oldContainer := util.GetContainer(nameToUpgrade, pod).DeepCopy()
		//兼容dadi镜像场景
		isDadiImage = utilasi.IsDadiImageName(oldContainer.Image)
		beforeJson = dumpJsonContainerForCompare(isDadiImage, oldContainer)
	}

	// create new container
	newContainer := sidecarContainer.Container.DeepCopy()
	containerInPod := util.GetContainer(nameToUpgrade, pod)
	newContainer.Name = containerInPod.Name
	utilasi.MergeEnvsInContainer(newContainer, *containerInPod)
	util.MergeVolumeMountsInContainer(newContainer, *containerInPod)
	if IsHotUpgradeContainer(sidecarContainer) {
		// 兼容 SIDECARSET_VERSION value的场景
		setSidecarContainerVersionEnv(newContainer)
	}
	afterJson := dumpJsonContainerForCompare(isDadiImage, newContainer.DeepCopy())
	// sidecar container no changed
	if beforeJson == afterJson {
		return nil
	}
	klog.V(3).Infof("upgrade pod(%s/%s) container(%s) from(%s) -> to(%s)", pod.Namespace, pod.Name, nameToUpgrade, beforeJson, afterJson)
	return newContainer
}

func (c *asiControl) NeedToInjectVolumeMount(volumeMount v1.VolumeMount) bool {
	return !commontypes.InjectedMounts.Has(volumeMount.MountPath)
}

func (c *asiControl) NeedToInjectInUpdatedPod(pod, oldPod *v1.Pod, sidecarContainer *appsv1alpha1.SidecarContainer,
	injectedEnvs []v1.EnvVar, injectedMounts []v1.VolumeMount) (
	needInject bool, existSidecars []*appsv1alpha1.SidecarContainer, existVolumes []v1.Volume) {

	sidecarContainerName := sidecarContainer.Name
	if IsHotUpgradeContainer(sidecarContainer) {
		sidecarContainerName, _ = GetHotUpgradeContainerName(sidecarContainer.Name)
	}

	if hasInjectedSidecar(pod, sidecarContainerName) {
		klog.V(3).Infof("pod(%s/%s) have sidecar container(%s)", pod.Namespace, pod.Name, sidecarContainerName)
		ctn := util.GetContainer(sidecarContainerName, pod)
		if updated, _ := isUpgradingSidecar(oldPod, pod, c.GetSidecarset().Name, sidecarContainer); updated {
			klog.V(3).Infof("pod(%s/%s) on upgrade sidecarSet, then don't need re-inject", pod.Namespace, pod.Name)
			return false, nil, nil
		} else if !shouldUpgradeSidecarOnUpdate(ctn, injectedEnvs, injectedMounts) {
			klog.V(3).Infof("pod(%s/%s) sidecar container(%s) specification not changed, then don't need re-inject", pod.Namespace, pod.Name, sidecarContainerName)
			return false, nil, nil
		}
		// 1. 修改了main容器的volumeMounts、envs等配置，进而导致sidecar容器配置变化（配置了ShareVolumePolicy、TransferEnvs），需要重新注入
	} else if hasInjectedSidecar(oldPod, sidecarContainerName) {
		klog.V(3).Infof("old pod(%s/%s) have sidecar container(%s)", pod.Namespace, pod.Name, sidecarContainerName)
		//TODO remove this branch once all inplace migrated to cloneset
		ctn := util.GetContainer(sidecarContainerName, oldPod)
		// 2. 某些原地升级的场景会覆盖pod.spec.containers，导致sidecar container的配置丢失，此时需要重新注入
		// --该场景下，oldPod中存在sidecar容器，但是newPod中不存在
		if !shouldUpgradeSidecarOnUpdate(ctn, injectedEnvs, injectedMounts) {
			klog.V(3).Infof("old pod(%s/%s) have sidecar container(%s), and re-inject old container", pod.Namespace, pod.Name, sidecarContainerName)
			return false, getExistingSidecars(sidecarContainer, oldPod), oldPod.Spec.Volumes
		}
	} else if !shouldInjectSidecarOnUpdate(pod, oldPod, c.SidecarSet) {
		klog.V(3).Infof("pod(%s/%s) don't have sidecar container(%s), and not inject sidecar on update", pod.Namespace, pod.Name, sidecarContainerName)
		return false, nil, nil
		// 3. 对已有的pod，在升级时注入sidecar的场景（例如：alimesh铺开的场景），
		// 此种情况：oldPod、newPod均不存在sidecar容器，且annotations[sidecarset.kruise.io/inject-on-inplace-update] = true
	}
	// 确认 Pod 要执行注入这个 sidecar 容器
	return true, nil, nil
}

func (c *asiControl) UpdatePodAnnotationsInUpgrade(changedContainers []string, pod *v1.Pod) {

	sidecarSet := c.GetSidecarset()
	// 1. sidecarSet hash in pod
	updatePodSidecarSetHash(pod, sidecarSet)

	// 2. 复用 inplaceSet 的 hash 来实现 "确认 pod 已升级"
	podHash := pod.Annotations[sigmak8sapi.AnnotationPodSpecHash]
	if !strings.Contains(podHash, GetSidecarSetRevision(sidecarSet)) {
		pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] =
			fmt.Sprintf("%v-sidecarset", GetSidecarSetRevision(sidecarSet))
	} else {
		pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] =
			fmt.Sprintf("%v-sidecarset-flip", GetSidecarSetRevision(sidecarSet))
	}

	// 3. 按照 inplaceSet 的升级规范来更新 label
	pod.Labels[apiinternal.LabelFinalStateUpgrading] = "true"
	return
}

func (c *asiControl) IsPodReady(pod *v1.Pod) bool {
	sidecarSet := c.GetSidecarset()
	// check whether hot upgrade is complete
	// map[string]string: {empty container name}->{sidecarSet.spec.containers[x].upgradeStrategy.HotUpgradeEmptyImage}
	emptyContainers := map[string]string{}
	for _, sidecarContainer := range sidecarSet.Spec.Containers {
		if IsHotUpgradeContainer(&sidecarContainer) {
			_, emptyContainer := GetPodHotUpgradeContainers(sidecarContainer.Name, pod)
			emptyContainers[emptyContainer] = sidecarContainer.UpgradeStrategy.HotUpgradeEmptyImage
		}
	}
	for _, container := range pod.Spec.Containers {
		// If container is empty container, then its image must be empty image
		// 这种场景说明：pod sidecar正在热升级过程中
		if emptyImage := emptyContainers[container.Name]; emptyImage != "" && container.Image != emptyImage {
			klog.V(5).Infof("pod(%s.%s) sidecar empty container(%s) Image(%s) isn't Empty Image(%s)",
				pod.Namespace, pod.Name, container.Name, container.Image, emptyImage)
			return false
		}
	}
	// 1. pod.Status.Phase == v1.PodRunning
	// 2. pod.condition PodReady == true
	if util.IsRunningAndReady(pod) {
		return true
	}

	// container.name -> containerStatus
	statusInPod := make(map[string]*v1.ContainerStatus)
	for i := range pod.Status.ContainerStatuses {
		cStatus := &pod.Status.ContainerStatuses[i]
		statusInPod[cStatus.Name] = cStatus
	}

	// 区分出本 sidecarSet 的 sidecar containers
	sidecarContainers, _ := getSidecarsInPod(c.GetSidecarset(), pod)
	// 如果不存在重要 sidecar，那么检查本 sidecar containers ready 即可
	// 检查升级的sidecar容器是否ready
	if !hasReadyAffectingContainer(sidecarContainers) {
		return isContainersReady(sidecarContainers, statusInPod)
	}

	// 如果存在重要 sidecar，则需要检查Pod所有的container是否ready
	containerStateSpec := sigmak8sapi.ContainerStateSpec{}
	if v, ok := pod.Annotations[sigmak8sapi.AnnotationContainerStateSpec]; ok {
		_ = json.Unmarshal([]byte(v), &containerStateSpec)
	}
	// 只看期望running的container是否ready
	for _, containerStatus := range statusInPod {
		// 当容器的desired-state未设置或者为running，则容器必须是ready的
		desiredState := containerStateSpec.States[sigmak8sapi.ContainerInfo{Name: containerStatus.Name}]
		if desiredState == "" || desiredState == sigmak8sapi.ContainerStateRunning {
			if !containerStatus.Ready {
				return false
			}
		}
	}
	return true
}

func (c *asiControl) IsPodStateConsistent(pod *v1.Pod, sidecarContainers sets.String) bool {
	if !utilasi.IsPodSpecHashPartConsistent(pod, sidecarContainers) {
		return false
	}

	// 检查containers状态是否符合期望
	containerStateSpec := sigmak8sapi.ContainerStateSpec{}
	if v, ok := pod.Annotations[sigmak8sapi.AnnotationContainerStateSpec]; ok {
		_ = json.Unmarshal([]byte(v), &containerStateSpec)
	}

	// 1. 当containers为空，不执行该逻辑，检查所有的containers
	// 2. 当containers不为空，只检查列表内的containers
	if sidecarContainers.Len() > 0 {
		for container, _ := range containerStateSpec.States {
			if !sidecarContainers.Has(container.Name) {
				delete(containerStateSpec.States, container)
			}
		}
	}

	// 如果有container期望状态是paused，则算作还在发布中
	if utilasi.ContainsInContainerStateSpec(&containerStateSpec, sigmak8sapi.ContainerStatePaused) {
		klog.V(3).Infof("pod(%s/%s) container state contains state(%s), then inconsistent", pod.Namespace, pod.Name, sigmak8sapi.ContainerStatePaused)
		return false
	}

	// 如果在debug模式，则算作在发布中
	if pod.Annotations[apiinternal.AnnotationPodDebugContext] != "" {
		klog.V(3).Infof("pod(%s/%s) annotations in context(%s), then inconsistent", pod.Namespace, pod.Name, apiinternal.AnnotationPodDebugContext)
		return false
	}
	return true
}

func (c *asiControl) IsSidecarSetUpgradable(pod *v1.Pod) bool {
	// cStatus: container.name -> containerStatus.Ready
	cStatus := map[string]bool{}
	for _, status := range pod.Status.ContainerStatuses {
		cStatus[status.Name] = status.Ready
	}
	updateStatus := utilasi.GetPodUpdatedSpecHashes(pod)
	podSpecHash := utilasi.GetPodSpecHash(pod)
	sidecarContainerList := GetSidecarContainersInPod(c.GetSidecarset())
	for _, sidecar := range sidecarContainerList.List() {
		// when containerStatus.Ready == true and container non-consistent,
		// indicates that sidecar container is in the process of being upgraded
		// wait for the last upgrade to complete before performing this upgrade
		if cStatus[sidecar] && updateStatus[sidecar] != "" && updateStatus[sidecar] != podSpecHash {
			return false
		}
	}

	return true
}

func (c *asiControl) IsPodAvailabilityChanged(pod, oldPod *v1.Pod) bool {
	//只有当pod spec改变时才会引起触发re-inject sidecar容器，即pod.annotations[pod.beta1.sigma.ali/pod-spec-hash]有变化
	return utilasi.IsPodInplaceUpgrading(pod, oldPod)
}

func isContainersReady(containers []*v1.Container, status map[string]*v1.ContainerStatus) bool {
	for _, c := range containers {
		containerStatus, ok := status[c.Name]
		if !ok || !containerStatus.Ready {
			return false
		}
	}
	return true
}

func isContainerReadyAffecting(container *v1.Container) bool {
	env := util.GetContainerEnvValue(container, EnvIgnorePodReady)
	// 不存在 degree 的是业务容器，也需要按 affected 处理
	return env != IgnorePodReadyValueTrue
}

func hasReadyAffectingContainer(containers []*v1.Container) bool {
	for _, c := range containers {
		if isContainerReadyAffecting(c) {
			return true
		}
	}
	return false
}

func getSidecarsInPod(s *appsv1alpha1.SidecarSet, pod *v1.Pod) (sidecarContainers []*v1.Container, otherContainers []*v1.Container) {
	sidecars := GetSidecarContainersInPod(s)
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if sidecars.Has(container.Name) {
			sidecarContainers = append(sidecarContainers, container)
		} else {
			otherContainers = append(otherContainers, container)
		}
	}
	return
}

func getExistingSidecars(sidecarContainer *appsv1alpha1.SidecarContainer, pod *v1.Pod) (sidecarContainers []*appsv1alpha1.SidecarContainer) {
	sidecarContainerName, sidecarContainerName2 := sidecarContainer.Name, ""
	if IsHotUpgradeContainer(sidecarContainer) {
		sidecarContainerName, sidecarContainerName2 = GetHotUpgradeContainerName(sidecarContainer.Name)
	}
	sidecarContainer.DeepCopy()
	sidecarContainer.Container = *util.GetContainer(sidecarContainerName, pod)
	sidecarContainers = append(sidecarContainers, sidecarContainer)
	if IsHotUpgradeContainer(sidecarContainer) {
		ctn2 := util.GetContainer(sidecarContainerName2, pod)
		// make a shallow copy of original sidecar
		sidecarContainer2 := sidecarContainer.DeepCopy()
		sidecarContainer2.Container = *ctn2
		sidecarContainers = append(sidecarContainers, sidecarContainer2)
	}
	return sidecarContainers
}

func hasInjectedSidecar(pod *v1.Pod, sidecarContainerName string) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == sidecarContainerName {
			return true
		}
	}
	return false
}

func shouldUpgradeSidecarOnUpdate(container *v1.Container, injectedEnvs []v1.EnvVar, injectedMounts []v1.VolumeMount) bool {
	for _, env := range injectedEnvs {
		if val := util.GetContainerEnvVar(container, env.Name); val == nil {
			// new container env should be injected
			klog.Infof("sidecar container %v should be upgraded for new env %v ",
				container.Name, env.Name)
			return true
		} else if val.Value != env.Value || !reflect.DeepEqual(val.ValueFrom, env.ValueFrom) {
			//container env should be updated
			klog.Infof("sidecar container %v should be upgraded for changed env, old: %v, new: %v",
				container.Name, val, env)
			return true
		}
	}

	for _, mount := range injectedMounts {
		if mnt := util.GetContainerVolumeMount(container, mount.MountPath); mnt == nil {
			// new container mount should be injected
			klog.Infof("sidecar container %v should be upgraded for new mount %v ",
				container.Name, mount.MountPath)
			return true
		} else if mnt.Name != mount.Name || mnt.SubPath != mount.SubPath || mnt.SubPathExpr != mount.SubPathExpr {
			//container volumeMounts should be updated
			klog.Infof("sidecar container %v should be upgraded for changed volumeMounts, old: %v, new: %v",
				container.Name, mnt, mount)
			return true
		}
	}

	return false
}

func shouldInjectSidecarOnUpdate(pod *v1.Pod, oldPod *v1.Pod, sidecarSet *appsv1alpha1.SidecarSet) bool {
	injectOnInplaceUpdate := sidecarSet.Annotations[SidecarInjectOnInplaceUpdate] == "true"
	if injectOnInplaceUpdate && utilasi.IsPodInplaceUpgrading(pod, oldPod) {
		return true
	}

	return false
}

func isUpgradingSidecar(oldPod *v1.Pod, pod *v1.Pod, sidecarSetName string, sidecarContainer *appsv1alpha1.SidecarContainer) (bool, error) {
	sidecar := sidecarContainer.Name
	hashKey := SidecarSetHashAnnotation
	if pod.Annotations[hashKey] == "" || oldPod.Annotations[hashKey] == "" {
		return false, nil
	}

	sidecarSetHashOld := GetPodSidecarSetRevision(sidecarSetName, oldPod)
	sidecarSetHashNew := GetPodSidecarSetRevision(sidecarSetName, pod)

	if sidecarSetHashOld != sidecarSetHashNew {
		return true, nil
	}

	if sidecarContainer.UpgradeStrategy.UpgradeType != appsv1alpha1.SidecarContainerHotUpgrade {
		return false, nil
	}

	// sidecar container has hot upgrade, check whether hot upgrade is in process
	name1, name2 := GetHotUpgradeContainerName(sidecar)

	if util.GetContainer(name1, pod).Image == util.GetContainer(name1, oldPod).Image &&
		util.GetContainer(name2, pod).Image == util.GetContainer(name2, oldPod).Image {
		return false, nil
	}

	return true, nil
}

// use Sidecarset.ResourceVersion to mark sidecar container version in env(SIDECARSET_VERSION)
// env(SIDECARSET_VERSION) ValueFrom pod.metadata.annotations['sidecarset.kruise.io/{container.name}.version']
func setSidecarContainerVersionEnv(container *v1.Container) {
	envs := make([]v1.EnvVar, 0)
	for _, env := range container.Env {
		if env.Name == SidecarSetVersionEnvKey || env.Name == SidecarSetVersionAltEnvKey {
			continue
		}
		envs = append(envs, env)
	}
	// inject SIDECARSET_VERSION
	envs = append(envs, v1.EnvVar{
		Name: SidecarSetVersionEnvKey,
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: fmt.Sprintf("metadata.annotations['%s']", GetPodSidecarSetVersionAnnotation(container.Name)),
			},
		},
	})
	// inject SIDECARSET_VERSION_ALT
	envs = append(envs, v1.EnvVar{
		Name: SidecarSetVersionAltEnvKey,
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: fmt.Sprintf("metadata.annotations['%s']", GetPodSidecarSetVersionAltAnnotation(container.Name)),
			},
		},
	})
	container.Env = envs
}

func dumpJsonContainerForCompare(isDadiImage bool, container *v1.Container) string {
	// 如果是dadi镜像，因为dadi镜像是不可逆转的，所以需要将tag之前的地址都去掉，只比较tag即可
	// 目前主要用在sidecarset场景下，其它场景如果需要使用的话，请着重考虑
	if isDadiImage {
		_, tag, _, err := util.ParseImage(container.Image)
		if err != nil {
			klog.Errorf("parse container(%s) image(%s) failed: %s", container.Name, container.Image, err.Error())
		} else {
			container.Image = tag
		}
		// 最新的dadi格式是在tag的最后面增加_dadi字符串,
		// 这种场景下不需要其它的容器再转换，只将此image的后缀"_dadi"去掉即可
	} else if strings.HasSuffix(container.Image, utilasi.RunTypeSuffix) {
		strings.TrimRight(container.Image, utilasi.RunTypeSuffix)
	}

	// 场景：存量的container中env SidecarSetVersion不是downward api的方式，在更新时，转换为downward api
	// 兼容上面两种情况，所以过滤掉SidecarSetVersion、SidecarSetVersionAlt
	envs := make([]v1.EnvVar, 0)
	for _, env := range container.Env {
		if env.Name == SidecarSetVersionEnvKey || env.Name == SidecarSetVersionAltEnvKey {
			continue
		}
		envs = append(envs, env)
	}
	container.Env = envs

	return util.DumpJSON(container)
}
