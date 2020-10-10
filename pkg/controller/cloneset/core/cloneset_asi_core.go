/*
Copyright 2020 The Kruise Authors.

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

package core

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/appscode/jsonpatch"
	appspub "github.com/openkruise/kruise/apis/apps/pub"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	clonesetutils "github.com/openkruise/kruise/pkg/controller/cloneset/utils"
	"github.com/openkruise/kruise/pkg/util"
	"github.com/openkruise/kruise/pkg/util/inplaceupdate"
	"github.com/openkruise/kruise/pkg/utilasi"
	"github.com/openkruise/kruise/pkg/utilasi/config"
	"github.com/openkruise/kruise/pkg/utilasi/naming/skyline"
	"github.com/pborman/uuid"
	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	sigmakruiseapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/kruise"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/klog"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/util/slice"
	"k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	gClient client.Client

	inPlaceUpdateTemplateSpecPatchASIRexp = regexp.MustCompile("^/spec/containers/([0-9]+)/(image|command|args|env|livenessProbe|readinessProbe|lifecycle)")
)

func InitASI(c client.Client) {
	gClient = c
}

type asiControl struct {
	*appsv1alpha1.CloneSet
}

var _ Control = &asiControl{}

func (c *asiControl) IsInitializing() bool {
	return c.Annotations[sigmakruiseapi.AnnotationCloneSetProxyInitializing] == "true"
}

func (c *asiControl) SetRevisionTemplate(revisionSpec map[string]interface{}, template map[string]interface{}) {
	if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
		templateCopy := make(map[string]interface{})
		templateSpec := template["spec"].(map[string]interface{})
		templateCopy["spec"] = templateSpec
		revisionSpec["template"] = templateCopy
		templateSpec["$patch"] = "replace"
		templateContainers := templateSpec["containers"].([]interface{})
		for i := range templateContainers {
			//delete(templateContainers[i].(map[string]interface{}), "resources")
			container := templateContainers[i].(map[string]interface{})
			delete(container, "resources")
			utilasi.RemoveSpecifiedEnvFromContainer(container, "SIGMA_LOG_SUFFIX")
		}
		return
	}
	revisionSpec["template"] = template
	template["$patch"] = "replace"
}

func (c *asiControl) ApplyRevisionPatch(patched []byte) (*appsv1alpha1.CloneSet, error) {
	restoredSet := &appsv1alpha1.CloneSet{}
	if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
		restoredSet = c.DeepCopy()
	}
	if err := json.Unmarshal(patched, restoredSet); err != nil {
		return nil, err
	}
	return restoredSet, nil
}

func (c *asiControl) IsReadyToScale() bool {
	return c.Annotations[sigmakruiseapi.AnnotationCloneSetBatchAdoptionToAbandon] == "" &&
		c.Annotations[sigmakruiseapi.AnnotationCloneSetBatchAdoptionToAdopt] == ""
}

func (c *asiControl) ExtraStatusCalculation(cs *appsv1alpha1.CloneSet, pods []*v1.Pod) bool {
	return false
}

func (c *asiControl) NewVersionedPods(currentCS, updateCS *appsv1alpha1.CloneSet,
	currentRevision, updateRevision string,
	expectedCreations, expectedCurrentCreations int,
	availableIDs []string,
) ([]*v1.Pod, error) {

	var newPods []*v1.Pod
	if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
		expectedCurrentCreations = 0
	}

	if c.Spec.ScaleStrategy.PoolConfig != nil {
		adoptedReplicas, err := c.createWithPooling(expectedCreations, updateRevision, c.Spec.ScaleStrategy.PoolConfig)
		if adoptedReplicas == expectedCreations || err != nil {
			return nil, err
		}

		klog.Warningf("CloneSet %s/%s expect to create %v with pool, but only adopted %v",
			c.Namespace, c.Name, expectedCreations, adoptedReplicas)
		if c.Spec.ScaleStrategy.PoolConfig.PolicyType == appsv1alpha1.PoolPolicyPoolingOnly {
			return nil, fmt.Errorf("create %v/%v with pooling only", adoptedReplicas, expectedCreations)
		}
		expectedCreations -= adoptedReplicas
	}

	if expectedCreations <= expectedCurrentCreations {
		newPods = c.newVersionedPods(currentCS, currentRevision, expectedCreations, &availableIDs)
	} else {
		newPods = c.newVersionedPods(currentCS, currentRevision, expectedCurrentCreations, &availableIDs)
		newPods = append(newPods, c.newVersionedPods(updateCS, updateRevision, expectedCreations-expectedCurrentCreations, &availableIDs)...)
	}
	return newPods, nil
}

func (c *asiControl) newVersionedPods(cs *appsv1alpha1.CloneSet, revision string, replicas int, availableIDs *[]string) []*v1.Pod {
	if replicas <= 0 {
		return nil
	}

	var additionalEnvs []v1.EnvVar
	// TODO(jiuzhu): remove safetyout injection
	inPlaceSetConfig, err := c.getInPlaceSetConfig(gClient)
	if err != nil {
		klog.Warningf("CloneSet %s/%s failed to get InPlaceSet config for newPod: %v", c.Namespace, c.Name, err)
	} else if inPlaceSetConfig.EnableAliProcHook {
		additionalEnvs = c.getAdditionalEnvs(&cs.Spec.Template.ObjectMeta, gClient)
	}

	var newPods []*v1.Pod
	for i := 0; i < replicas; i++ {
		pod, _ := controller.GetPodFromTemplate(&cs.Spec.Template, cs, metav1.NewControllerRef(cs, clonesetutils.ControllerKind))
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Namespace = cs.Namespace
		pod.Labels[apps.ControllerRevisionHashLabelKey] = revision
		pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = revision

		if cs.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
			if cs.Annotations[sigmakruiseapi.AnnotationCloneSetUsePodGenerateName] != "true" {
				random := uuid.NewRandom().String()
				pod.Name = prefixNameWithMaxLen(cs.Namespace+"--", random, 50)
			} else {
				pod.Name = fmt.Sprintf("%s%s", pod.GenerateName, utilrand.String(5))
			}

		} else {
			id := (*availableIDs)[0]
			*availableIDs = (*availableIDs)[1:]
			pod.Name = fmt.Sprintf("%s-%s", cs.Name, id)
			pod.Labels[appsv1alpha1.CloneSetInstanceID] = id

			inplaceupdate.InjectReadinessGate(pod)
		}

		if cs.Labels[sigmakruiseapi.LabelCloneSetDisableInjectSn] != "true" {
			pod.Annotations[apiinternal.AnnotationPodInjectNameAsSN] = "true"
		}

		c.injectAdditionalEnvsIntoPod(pod, additionalEnvs, &cs.Spec.Template)

		clonesetutils.UpdateStorage(cs, pod)

		klog.V(6).Infof("DEBUG %s/%s new pod %s", c.Namespace, c.Name, util.DumpJSON(pod))
		newPods = append(newPods, pod)
	}
	return newPods
}

func (c *asiControl) GetPodsSortFunc(pods []*v1.Pod, waitUpdateIndexes []int) func(i, j int) bool {
	// not-ready < ready, unscheduled < scheduled, and pending < running
	return func(i, j int) bool {
		podI := pods[waitUpdateIndexes[i]]
		podJ := pods[waitUpdateIndexes[j]]
		iCanary := podI.Labels[apiinternal.LabelPodUpgradeCanary] == "true"
		jCanary := podJ.Labels[apiinternal.LabelPodUpgradeCanary] == "true"
		if iCanary != jCanary {
			return iCanary
		}
		iTobeAdopted := podI.Labels[apiinternal.LabelPodBatchAdoption] == "Adopt" && podI.Labels[apps.ControllerRevisionHashLabelKey] == ""
		jTobeAdopted := podJ.Labels[apiinternal.LabelPodBatchAdoption] == "Adopt" && podJ.Labels[apps.ControllerRevisionHashLabelKey] == ""
		if iTobeAdopted != jTobeAdopted {
			return iTobeAdopted
		}
		iPostpone := podI.Labels[apiinternal.LabelPodUpgradePostpone] == "true"
		jPostpone := podJ.Labels[apiinternal.LabelPodUpgradePostpone] == "true"
		if iPostpone != jPostpone {
			return jPostpone
		}
		activePods := timeOblivousActivePods{podI, podJ}
		if activePods.Less(0, 1) {
			return true
		}
		if activePods.Less(1, 0) {
			return false
		}
		iPriority := podI.Labels[apiinternal.LabelPodUpgradePriority] == "true"
		jPriority := podJ.Labels[apiinternal.LabelPodUpgradePriority] == "true"
		if iPriority != jPriority {
			return iPriority
		}
		iPodReadyCond := podutil.GetPodReadyCondition(podI.Status)
		jPodReadyCond := podutil.GetPodReadyCondition(podJ.Status)
		if iPodReadyCond != nil && jPodReadyCond != nil {
			// recent upgrade pods < older upgrade pods
			if !iPodReadyCond.LastTransitionTime.Equal(&jPodReadyCond.LastTransitionTime) {
				return afterOrZero(&iPodReadyCond.LastTransitionTime, &jPodReadyCond.LastTransitionTime)
			}
		}
		return false
	}
}

// afterOrZero checks if time t1 is after time t2; if one of them
// is zero, the zero time is seen as after non-zero time.
func afterOrZero(t1, t2 *metav1.Time) bool {
	if t1.Time.IsZero() || t2.Time.IsZero() {
		return t1.Time.IsZero()
	}
	return t1.After(t2.Time)
}

// timeOblivousActivePods type allows custom sorting of pods so a controller can pick the best ones to delete.
type timeOblivousActivePods []*v1.Pod

func (s timeOblivousActivePods) Len() int      { return len(s) }
func (s timeOblivousActivePods) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// fork from k8s.io/kubernetes/pkg/controller/controller_utils.go
// we comment rule 4, 5 and 6
func (s timeOblivousActivePods) Less(i, j int) bool {
	// 1. Unassigned < assigned
	// If only one of the pods is unassigned, the unassigned one is smaller
	if s[i].Spec.NodeName != s[j].Spec.NodeName && (len(s[i].Spec.NodeName) == 0 || len(s[j].Spec.NodeName) == 0) {
		return len(s[i].Spec.NodeName) == 0
	}
	// 2. PodPending < PodUnknown < PodRunning
	m := map[v1.PodPhase]int{v1.PodPending: 0, v1.PodUnknown: 1, v1.PodRunning: 2}
	if m[s[i].Status.Phase] != m[s[j].Status.Phase] {
		return m[s[i].Status.Phase] < m[s[j].Status.Phase]
	}
	// 3. Not ready < ready
	// If only one of the pods is not ready, the not ready one is smaller
	if podutil.IsPodReady(s[i]) != podutil.IsPodReady(s[j]) {
		return !podutil.IsPodReady(s[i])
	}
	//// TODO: take availability into account when we push minReadySeconds information from deployment into pods,
	////       see https://github.com/kubernetes/kubernetes/issues/22065
	//// 4. Been ready for empty time < less time < more time
	//// If both pods are ready, the latest ready one is smaller
	//if podutil.IsPodReady(s[i]) && podutil.IsPodReady(s[j]) && !podReadyTime(s[i]).Equal(podReadyTime(s[j])) {
	//	return afterOrZero(podReadyTime(s[i]), podReadyTime(s[j]))
	//}
	//// 5. Pods with containers with higher restart counts < lower restart counts
	//if maxContainerRestarts(s[i]) != maxContainerRestarts(s[j]) {
	//	return maxContainerRestarts(s[i]) > maxContainerRestarts(s[j])
	//}
	//// 6. Empty creation time pods < newer pods < older pods
	//if !s[i].CreationTimestamp.Equal(&s[j].CreationTimestamp) {
	//	return afterOrZero(&s[i].CreationTimestamp, &s[j].CreationTimestamp)
	//}
	return false
}

func (c *asiControl) IsPodUpdatePaused(pod *v1.Pod) bool {
	return pod.Annotations[apiinternal.AnnotationPausePodUpgrade] == "true"
}

func (c *asiControl) IsPodUpdateReady(pod *v1.Pod, minReadySeconds int32) bool {
	condition := inplaceupdate.GetCondition(pod)
	if condition != nil && condition.Status != v1.ConditionTrue {
		return false
	}

	if err := c.checkPodUpdateCompleted(pod); err != nil {
		return false
	}

	var containerStateSpec *sigmak8sapi.ContainerStateSpec
	if v, ok := pod.Annotations[sigmak8sapi.AnnotationContainerStateSpec]; ok {
		_ = json.Unmarshal([]byte(v), &containerStateSpec)
	}

	if utilasi.IsContainerStateSpecAllRunning(containerStateSpec) {
		return clonesetutils.IsRunningAndAvailable(pod, minReadySeconds)
	}

	// 如果有其他状态的container，则只看期望running的container是否ready
	isContainersReady := true
	for _, containerStatus := range pod.Status.ContainerStatuses {
		// 当容器的desired-state未设置或者为running，则容器必须是ready的
		desiredState := containerStateSpec.States[sigmak8sapi.ContainerInfo{Name: containerStatus.Name}]
		if desiredState == "" || desiredState == sigmak8sapi.ContainerStateRunning {
			if !containerStatus.Ready {
				isContainersReady = false
				break
			}
		}
	}

	return isContainersReady
}

func (c *asiControl) checkPodUpdateCompleted(pod *v1.Pod) error {
	// 检查所有容器对应的spec-hash都符合期望
	if !utilasi.IsPodSpecHashConsistent(pod) {
		return fmt.Errorf("spec hash not consistent yet")
	}

	// 检查containers状态是否符合期望
	var containerStateSpec *sigmak8sapi.ContainerStateSpec
	if v, ok := pod.Annotations[sigmak8sapi.AnnotationContainerStateSpec]; ok {
		_ = json.Unmarshal([]byte(v), &containerStateSpec)
	}

	// 如果有container期望状态是paused，则算作还在发布中
	if utilasi.ContainsInContainerStateSpec(containerStateSpec, sigmak8sapi.ContainerStatePaused) {
		return fmt.Errorf("contains paused in container-state-spec")
	}

	// 如果在debug模式，则算作在发布中
	if pod.Annotations[apiinternal.AnnotationPodDebugContext] != "" {
		return fmt.Errorf("still in debug mode, %s=%s", apiinternal.AnnotationPodDebugContext, pod.Annotations[apiinternal.AnnotationPodDebugContext])
	}

	return nil
}

func (c *asiControl) GetUpdateOptions() *inplaceupdate.UpdateOptions {
	opts := &inplaceupdate.UpdateOptions{
		CalculateSpec:           c.customizeSpecCalculate,
		PatchSpecToPod:          c.customizePatchPod,
		CheckPodUpdateCompleted: c.checkPodUpdateCompleted,
	}
	if c.Spec.UpdateStrategy.InPlaceUpdateStrategy != nil {
		opts.GracePeriodSeconds = c.Spec.UpdateStrategy.InPlaceUpdateStrategy.GracePeriodSeconds
	}
	return opts
}

func (c *asiControl) customizeSpecCalculate(oldRevision, newRevision *apps.ControllerRevision, opts *inplaceupdate.UpdateOptions) *inplaceupdate.UpdateSpec {
	opts = inplaceupdate.SetOptionsDefaults(opts)
	updateSpec := &inplaceupdate.UpdateSpec{
		Revision:     newRevision.Name,
		GraceSeconds: opts.GracePeriodSeconds,
	}

	if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
		if oldRevision == nil {
			// 只写 revision，后续升级的时候采用覆盖 containers 列表升级
			return updateSpec
		}

	} else {
		if oldRevision == nil || newRevision == nil {
			return nil
		}
		patches, err := jsonpatch.CreatePatch(oldRevision.Data.Raw, newRevision.Data.Raw)
		if err != nil {
			klog.Warningf("CloneSet %s/%s failed to create patch between %s and %s: %v",
				c.Namespace, c.Name, oldRevision.Name, newRevision.Name, err)
			return nil
		}

		for _, jsonPatchOperation := range patches {
			jsonPatchOperation.Path = strings.Replace(jsonPatchOperation.Path, "/spec/template", "", 1)

			// metadata 下的变化，全部原地升级
			if strings.HasPrefix(jsonPatchOperation.Path, "/metadata/") {
				continue
			}

			// spec.containers 下面的 image|command|args|env|livenessProbe|readinessProbe|lifecycle 可以原地升级
			if !inPlaceUpdateTemplateSpecPatchASIRexp.MatchString(jsonPatchOperation.Path) {
				return nil
			}
		}
	}

	oldTemp, err := inplaceupdate.GetTemplateFromRevision(oldRevision)
	if err != nil {
		return nil
	}
	newTemp, err := inplaceupdate.GetTemplateFromRevision(newRevision)
	if err != nil {
		return nil
	}
	updateSpec.OldTemplate = oldTemp
	updateSpec.NewTemplate = newTemp
	return updateSpec
}

func (c *asiControl) customizePatchPod(pod *v1.Pod, spec *inplaceupdate.UpdateSpec, state *appspub.InPlaceUpdateState) (*v1.Pod, error) {
	setConfig, err := c.getInPlaceSetConfig(gClient)
	if err != nil {
		klog.Warningf("CloneSet %s/%s failed to get InPlaceSet config: %v", c.Namespace, c.Name, err)
	}

	// basic
	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	if v, ok := c.Annotations[sigmakruiseapi.AnnotationCloneSetUpgradeTimeout]; ok {
		pod.Annotations[apiinternal.AnnotationPodUpgradeTimeout] = v
	} else {
		delete(pod.Annotations, apiinternal.AnnotationPodUpgradeTimeout)
	}

	now := time.Now()
	pod.Labels[apiinternal.LabelPodUpgradingState] = apiinternal.PodUpgradingExecuting
	pod.Labels[apiinternal.LabelFinalStateUpgrading] = "true"
	pod.Labels[apps.StatefulSetRevisionLabel] = spec.Revision
	pod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = fmt.Sprintf("%s_%d", spec.Revision, now.UnixNano())

	if c.Spec.Template.Spec.TerminationGracePeriodSeconds != nil {
		terminationSeconds := *c.Spec.Template.Spec.TerminationGracePeriodSeconds
		pod.Spec.TerminationGracePeriodSeconds = &terminationSeconds
	}

	upgradeSpec := podUpgradeSpec{
		UpdateTimestamp: &metav1.Time{Time: now},
		SpecHash:        spec.Revision,
		DiffUpdate:      true,
	}

	if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
		mergeVolumesIntoPod(pod, c.Spec.Template.Spec.Volumes)
		mergeAnnotations(pod, c.Annotations[sigmak8sapi.AnnotationInplaceUpgradeMergeAnnotations], c.Spec.Template.Annotations)
		mergeLabels(pod, c.Spec.Template.Labels)

		upgradeSpec.DiffUpdate = spec.OldTemplate != nil && spec.NewTemplate != nil && !isDiffUpdateDisabled()
	}

	if upgradeSpec.DiffUpdate {
		// patch containers
		var oldBytes, newBytes []byte
		if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
			oldBytes, _ = json.Marshal(v1.Pod{Spec: v1.PodSpec{Containers: spec.OldTemplate.Spec.Containers}})
			newBytes, _ = json.Marshal(v1.Pod{Spec: v1.PodSpec{Containers: spec.NewTemplate.Spec.Containers}})
		} else {
			oldBytes, _ = json.Marshal(v1.Pod{ObjectMeta: spec.OldTemplate.ObjectMeta, Spec: spec.OldTemplate.Spec})
			newBytes, _ = json.Marshal(v1.Pod{ObjectMeta: spec.NewTemplate.ObjectMeta, Spec: spec.NewTemplate.Spec})
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldBytes, newBytes, &v1.Pod{})
		if err != nil {
			return nil, err
		}
		klog.V(3).Infof("CloneSet %s/%s in-place update Pod %s diff patch: %v",
			c.Namespace, c.Name, pod.Name, string(patchBytes))

		cloneBytes, _ := json.Marshal(pod)
		modified, err := strategicpatch.StrategicMergePatch(cloneBytes, patchBytes, &v1.Pod{})
		if err != nil {
			return nil, err
		}
		pod = &v1.Pod{}
		if err = json.Unmarshal(modified, pod); err != nil {
			return nil, err
		}
	} else {
		// overwrite containers
		oldContainers := pod.Spec.Containers
		pod.Spec.Containers = c.Spec.Template.Spec.Containers
		mergeResourcesIntoPod(pod, oldContainers)
		mergeVolumeMountsIntoPod(pod, oldContainers)
		mergeSpecializedEnvs(pod, oldContainers, setConfig.EnvKeysInherited)
	}

	pod.Annotations[apiinternal.AnnotationUpgradeSpec] = util.DumpJSON(upgradeSpec)
	if setConfig != nil && setConfig.EnableAliProcHook {
		additionalEnvs := c.getAdditionalEnvs(&pod.ObjectMeta, gClient)
		c.injectAdditionalEnvsIntoPod(pod, additionalEnvs, &c.Spec.Template)
	}

	return pod, nil
}

type podUpgradeSpec struct {
	UpdateTimestamp *metav1.Time `json:"updateTimestamp,omitempty"`
	SpecHash        string       `json:"specHash,omitempty"`
	DiffUpdate      bool         `json:"diffUpdate,omitempty"`
}

func mergeResourcesIntoPod(pod *v1.Pod, oldContainers []v1.Container) {
	for i, c := range pod.Spec.Containers {
		var oldContainer *v1.Container
		for _, oc := range oldContainers {
			if c.Name == oc.Name {
				oldContainer = &oc
				break
			}
		}
		if oldContainer == nil {
			continue
		}
		pod.Spec.Containers[i].Resources = oldContainer.Resources
	}
}

func mergeVolumesIntoPod(pod *v1.Pod, volumes []v1.Volume) {
	if len(volumes) == 0 {
		return
	}
	mergedVolumes := map[string]v1.Volume{}
	for _, v := range pod.Spec.Volumes {
		mergedVolumes[v.Name] = v
	}
	for _, v := range volumes {
		mergedVolumes[v.Name] = v
	}
	newVolumes := make([]v1.Volume, 0, len(mergedVolumes))
	for _, v := range mergedVolumes {
		newVolumes = append(newVolumes, v)
	}
	pod.Spec.Volumes = newVolumes
}

func mergeSpecializedEnvs(pod *v1.Pod, oldContainers []v1.Container, inheritedKeys []string) {
	for i, c := range pod.Spec.Containers {
		var oldContainer *v1.Container
		for _, oc := range oldContainers {
			if c.Name == oc.Name {
				oldContainer = &oc
				break
			}
		}
		if oldContainer == nil {
			continue
		}
		for _, env := range oldContainer.Env {
			if slice.ContainsString(inheritedKeys, env.Name, nil) {
				utilasi.AddContainerEnvNoOverwrite(&pod.Spec.Containers[i], env.Name, env.Value)
			}
		}
	}
}

func mergeVolumeMountsIntoPod(pod *v1.Pod, oldContainers []v1.Container) {
	for i, c := range pod.Spec.Containers {
		var oldContainer *v1.Container
		for _, oc := range oldContainers {
			if c.Name == oc.Name {
				oldContainer = &oc
				break
			}
		}
		if oldContainer == nil {
			continue
		}
		var serviceAccountMount *v1.VolumeMount
		for _, vm := range oldContainer.VolumeMounts {
			if vm.MountPath == serviceaccount.DefaultAPITokenMountPath {
				serviceAccountMount = &vm
				break
			}
		}
		if serviceAccountMount == nil {
			continue
		}
		var found bool
		for _, vm := range c.VolumeMounts {
			if vm.Name == serviceAccountMount.Name {
				found = true
				break
			}
		}
		if !found {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, *serviceAccountMount)
		}
	}
}

func mergeLabels(pod *v1.Pod, templateLabels map[string]string) {
	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	if order, ok := templateLabels[apiinternal.LabelPodUpgradeBatchOrder]; ok {
		pod.Labels[apiinternal.LabelPodUpgradeBatchOrder] = order
	} else {
		delete(pod.Labels, apiinternal.LabelPodUpgradeBatchOrder)
	}
}

func mergeAnnotations(pod *v1.Pod, upgradeMergeAnnotations string, templateAnnotations map[string]string) {
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	// 同步 inplaceset sigma.ali/upgrade-merge-annotations 列表中指定的 annotation
	// 同步 pod.beta1.sigma.ali/container-extra-config annotation
	upgradeMergeAnnotationsArray := strings.Split(upgradeMergeAnnotations, ",")
	upgradeMergeAnnotationsArray = append(upgradeMergeAnnotationsArray, sigmak8sapi.AnnotationContainerExtraConfig)
	for _, key := range upgradeMergeAnnotationsArray {
		key = strings.TrimSpace(key)
		if value, ok := templateAnnotations[key]; ok {
			pod.Annotations[key] = value
		} else {
			delete(pod.Annotations, key)
		}
	}
}

func (c *asiControl) getInPlaceSetConfig(r client.Reader) (*config.InPlaceSetConfig, error) {
	inPlaceSetConfig := config.InPlaceSetConfig{}
	_, err := config.NewConfigMgr(r).GetConfigFromConfigMap(config.InPlaceSetConfigType, &inPlaceSetConfig)
	if err != nil {
		return nil, fmt.Errorf("failed get inplaceset config: %v", err)
	}
	return &inPlaceSetConfig, nil
}

func (c *asiControl) injectAdditionalEnvsIntoPod(pod *v1.Pod, envs []v1.EnvVar, template *v1.PodTemplateSpec) {
	for _, envVar := range envs {
		for i := range pod.Spec.Containers {
			c := &pod.Spec.Containers[i]
			var found bool
			for j := range template.Spec.Containers {
				if template.Spec.Containers[j].Name == c.Name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			utilasi.AddContainerEnvHeadWithOverwrite(c, envVar.Name, envVar.Value)
		}
	}
}

func (c *asiControl) getAdditionalEnvs(objectMeta *metav1.ObjectMeta, reader client.Reader) []v1.EnvVar {
	if objectMeta.Annotations[sigmak8sapi.AnnotationEnableAppRulesInjection] != "true" ||
		objectMeta.Labels[sigmak8sapi.LabelInstanceGroup] == "" {
		return nil
	}
	nodeGroup, err := skyline.QueryNodeGroupByName(reader, objectMeta.Labels[sigmak8sapi.LabelInstanceGroup])
	if err != nil {
		klog.Warningf("CloneSet %s/%s failed to query nodegroup for %v safetyout, err: %v",
			c.Namespace, c.Name, objectMeta.Labels[sigmak8sapi.LabelInstanceGroup], err)
		return nil
	} else if nodeGroup == nil {
		klog.Warningf("CloneSet %s/%s failed to query nodegroup for %v safetyout, not found",
			c.Namespace, c.Name, objectMeta.Labels[sigmak8sapi.LabelInstanceGroup])
		return nil
	}
	safetyOut := "0"
	if nodeGroup.SafetyOut == 1 {
		safetyOut = "1"
	}
	return []v1.EnvVar{{Name: "ali_safety_out", Value: safetyOut}}
}

func (c *asiControl) ValidateCloneSetUpdate(oldCS, newCS *appsv1alpha1.CloneSet) error {
	isOldInPlaceOnly := oldCS.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType
	isNewInPlaceOnly := newCS.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType
	if isOldInPlaceOnly != isNewInPlaceOnly {
		return fmt.Errorf("forbid to modified InPlaceOnly for CloneSet with asi mode")
	}

	return nil
}

func prefixNameWithMaxLen(prefix string, name string, maxLen int) string {
	if len(prefix+name) > maxLen {
		//if len(prefix) > len(prefix+name)-maxLen {
		//	prefix = prefix[0 : len(prefix)-len(prefix+name)+maxLen]
		//} else {
		//	prefix = ""
		//}
		prefix = ""
	}
	name = prefix + name
	if len(name) > maxLen {
		return name[0:maxLen]
	}
	return name
}

// TODO(jiuzhu): remove this function after stable
func isDiffUpdateDisabled() bool {
	return os.Getenv("DISABLE_CLONESET_DIFF_UPDATE") == "true"
}
