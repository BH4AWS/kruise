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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
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

	podPhaseToOrdinal = map[v1.PodPhase]int{v1.PodPending: 0, v1.PodUnknown: 1, v1.PodRunning: 2}
)

const (
	annotationSafetyOutInjection = "pod.beta1.alibabacloud.com/enable-safety-out-injection"
	labelSafetyOut               = "pod.beta1.alibabacloud.com/ali-safety-out"
	envSafetyOut                 = "ali_safety_out"
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

func (c *asiControl) ExtraStatusCalculation(status *appsv1alpha1.CloneSetStatus, pods []*v1.Pod) (map[string]string, bool, error) {
	publishSuccessReplicas := 0
	scheduledFailedCount := 0
	imageFailedCount := 0
	appFailedCount := 0
	for _, pod := range pods {
		if c.isPublishSuccess(status, pod) {
			publishSuccessReplicas++
		} else if c.isScheduledFailed(status, pod) {
			scheduledFailedCount++
		} else {
			podReason := parsePodReason(pod)
			if strings.Contains(podReason, "Image") {
				imageFailedCount++
			} else if strings.Contains(podReason, "PostStartHookError") ||
				strings.Contains(podReason, "CrashLoopBackOff") {
				appFailedCount++
			}
		}
	}

	publishSuccessReplicasStr := strconv.Itoa(publishSuccessReplicas)
	scheduledFailedCountStr := strconv.Itoa(scheduledFailedCount)
	imageFailedCountStr := strconv.Itoa(imageFailedCount)
	appFailedCountStr := strconv.Itoa(appFailedCount)

	modified := false
	if c.Annotations[apiinternal.AnnotationPublishSuccessReplicas] != publishSuccessReplicasStr ||
		c.Annotations[apiinternal.AnnotationScheduledFailCount] != scheduledFailedCountStr ||
		c.Annotations[apiinternal.AnnotationImageFailCount] != imageFailedCountStr ||
		c.Annotations[apiinternal.AnnotationAppFailCount] != appFailedCountStr {
		modified = true
	}

	extraStatus := map[string]string{}
	extraStatus[apiinternal.AnnotationPublishSuccessReplicas] = publishSuccessReplicasStr
	extraStatus[apiinternal.AnnotationScheduledFailCount] = scheduledFailedCountStr
	extraStatus[apiinternal.AnnotationImageFailCount] = imageFailedCountStr
	extraStatus[apiinternal.AnnotationAppFailCount] = appFailedCountStr
	return extraStatus, modified, nil
}

func (c *asiControl) NewVersionedPods(currentCS, updateCS *appsv1alpha1.CloneSet,
	currentRevision, updateRevision string,
	expectedCreations, expectedCurrentCreations int,
	availableIDs []string,
) ([]*v1.Pod, error) {

	var newPods []*v1.Pod
	if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType &&
		// 现在要求支持 normandy 按百分比发布，并按照 partition 来扩容新旧版本的 Pod，因此该特殊逻辑仅在设置绝对值时启用
		(c.Spec.UpdateStrategy.Partition == nil || c.Spec.UpdateStrategy.Partition.Type == intstr.Int) {
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
		newPods = c.newVersionedPods(currentCS, currentRevision, expectedCreations, &availableIDs, false)
	} else {
		newPods = c.newVersionedPods(currentCS, currentRevision, expectedCurrentCreations, &availableIDs, false)
		newPods = append(newPods, c.newVersionedPods(updateCS, updateRevision, expectedCreations-expectedCurrentCreations, &availableIDs, true)...)
	}
	return newPods, nil
}

func (c *asiControl) newVersionedPods(cs *appsv1alpha1.CloneSet, revision string, replicas int, availableIDs *[]string, isUpdateRevision bool) []*v1.Pod {
	if replicas <= 0 {
		return nil
	}

	var additionalEnvs []v1.EnvVar
	var additionalLabels map[string]string

	// TODO(jiuzhu): remove safetyout injection
	inPlaceSetConfig, err := c.getInPlaceSetConfig(gClient)
	if err != nil {
		klog.Warningf("CloneSet %s/%s failed to get InPlaceSet config for newPod: %v", c.Namespace, c.Name, err)
	} else if inPlaceSetConfig.EnableAliProcHook {
		additionalEnvs, additionalLabels = c.getAdditionalPodConfig(&cs.Spec.Template.ObjectMeta, gClient)
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
		c.injectAdditionalLabelsIntoPod(pod, additionalLabels)

		clonesetutils.UpdateStorage(cs, pod)

		klog.V(6).Infof("DEBUG %s/%s new pod %s", c.Namespace, c.Name, util.DumpJSON(pod))
		newPods = append(newPods, pod)
	}
	return newPods
}

func (c *asiControl) GetPodSpreadConstraint() []clonesetutils.PodSpreadConstraint {
	var constraints []clonesetutils.PodSpreadConstraint

	if constraintName, ok := c.Spec.Template.Labels[apiinternal.LabelPodConstraintName]; ok {
		unstructuredObj := unstructured.Unstructured{}
		unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "scheduling.alibabacloud.com",
			Version: "v1beta1",
			Kind:    "PodConstraint",
		})
		if err := gClient.Get(context.TODO(), types.NamespacedName{Namespace: c.Namespace, Name: constraintName}, &unstructuredObj); err != nil {
			klog.Errorf("Failed to get PodConstraint %s for CloneSet %s/%s: %v", constraintName, c.Namespace, c.Name, err)
			return nil
		}

		jsonBytes, _ := unstructuredObj.MarshalJSON()
		constraintObj := apiinternal.PodConstraint{}
		if err := json.Unmarshal(jsonBytes, &constraintObj); err != nil {
			klog.Errorf("Failed to unmarshal PodConstraint %s for CloneSet %s/%s: %v, json: %v", constraintName, c.Namespace, c.Name, err, string(jsonBytes))
			return nil
		}

		ruleItems := append(constraintObj.Spec.SpreadRule.Requires, constraintObj.Spec.SpreadRule.Affinities...)
		existingTopologyKey := sets.NewString()
		for _, rule := range ruleItems {
			if existingTopologyKey.Has(rule.TopologyKey) {
				continue
			}
			existingTopologyKey.Insert(rule.TopologyKey)
			constraints = append(constraints, convertConstraintRule(rule))
		}

		klog.V(3).Infof("Get spread constraints for ASI CloneSet %s/%s (%s): %v", c.Namespace, c.Name, constraintName, util.DumpJSON(constraints))
		return constraints
	}

	for _, c := range c.Spec.Template.Spec.TopologySpreadConstraints {
		constraints = append(constraints, clonesetutils.PodSpreadConstraint{TopologyKey: c.TopologyKey})
	}
	return constraints
}

func convertConstraintRule(rule apiinternal.SpreadRuleItem) clonesetutils.PodSpreadConstraint {
	c := clonesetutils.PodSpreadConstraint{TopologyKey: rule.TopologyKey}
	if rule.PodSpreadType == apiinternal.PodSpreadTypeRatio {
		for _, ratio := range rule.TopologyRatios {
			c.LimitedValues = append(c.LimitedValues, ratio.TopologyValue)
		}
	}
	return c
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
		// If only one of the pods is unassigned, the unassigned one is smaller
		if podI.Spec.NodeName != podJ.Spec.NodeName && (len(podI.Spec.NodeName) == 0 || len(podJ.Spec.NodeName) == 0) {
			return len(podI.Spec.NodeName) == 0
		}
		// PodPending < PodUnknown < PodRunning
		if podPhaseToOrdinal[podI.Status.Phase] != podPhaseToOrdinal[podJ.Status.Phase] {
			return podPhaseToOrdinal[podI.Status.Phase] < podPhaseToOrdinal[podJ.Status.Phase]
		}
		// unavailable < available
		iPodAvailable := c.IsPodUpdateReady(podI, c.Spec.MinReadySeconds)
		jPodAvailable := c.IsPodUpdateReady(podJ, c.Spec.MinReadySeconds)
		if iPodAvailable != jPodAvailable {
			return jPodAvailable
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

func (c *asiControl) checkContainersUpdateCompleted(pod *v1.Pod, inPlaceUpdateState *appspub.InPlaceUpdateState) error {
	return c.checkPodUpdateCompleted(pod)
}

func (c *asiControl) GetUpdateOptions() *inplaceupdate.UpdateOptions {
	opts := &inplaceupdate.UpdateOptions{
		CalculateSpec:                  c.customizeSpecCalculate,
		PatchSpecToPod:                 c.customizePatchPod,
		CheckPodUpdateCompleted:        c.checkPodUpdateCompleted,
		CheckContainersUpdateCompleted: c.checkContainersUpdateCompleted,
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
	if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
		trimTemplateForInPlaceOnlyStrategy(oldTemp)
	}
	updateSpec.OldTemplate = oldTemp
	updateSpec.NewTemplate = newTemp
	return updateSpec
}

func trimTemplateForInPlaceOnlyStrategy(template *v1.PodTemplateSpec) {
	template.ObjectMeta = metav1.ObjectMeta{}
	for i := range template.Spec.Containers {
		template.Spec.Containers[i].Resources = v1.ResourceRequirements{}
		utilasi.RemoveContainerEnvVar(&template.Spec.Containers[i], "SIGMA_LOG_SUFFIX")
	}
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
	if pod.Labels[apiinternal.LabelScheduleNodeName] != "" {
		utilasi.SetUpdateSpecHash(pod, fmt.Sprintf("%s_%d", spec.Revision, now.UnixNano()))
	}

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
		mergeAnnotations(pod, c.Annotations[apiinternal.AnnotationInplaceUpgradeMergeAnnotations], c.Spec.Template.Annotations)
		mergeLabels(pod, c.Annotations[apiinternal.AnnotationInplaceUpgradeMergeLabels], c.Spec.Template.Labels)

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
		additionalEnvs, additionalLabels := c.getAdditionalPodConfig(&pod.ObjectMeta, gClient)
		c.injectAdditionalEnvsIntoPod(pod, additionalEnvs, &c.Spec.Template)
		c.injectAdditionalLabelsIntoPod(pod, additionalLabels)
	}

	if publishId, ok := c.Annotations[apiinternal.AnnotationAppsPublishId]; ok {
		pod.Annotations[apiinternal.AnnotationAppsPublishId] = publishId
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

func mergeLabels(pod *v1.Pod, upgradeMergeLabels string, templateLabels map[string]string) {
	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	// 同步 sigma.ali/upgrade-merge-labels 列表中指定的 label
	// 固定同步 inplaceset.beta1.sigma.ali/upgrade-batch-order
	mergeLabelKeys := sets.NewString(apiinternal.LabelPodUpgradeBatchOrder)
	if len(upgradeMergeLabels) > 0 {
		mergeLabelKeys.Insert(strings.Split(upgradeMergeLabels, ",")...)
	}
	for _, key := range mergeLabelKeys.UnsortedList() {
		key = strings.TrimSpace(key)
		if value, ok := templateLabels[key]; ok {
			pod.Labels[key] = value
		} else {
			delete(pod.Labels, key)
		}
	}
}

func mergeAnnotations(pod *v1.Pod, upgradeMergeAnnotations string, templateAnnotations map[string]string) {
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	// 同步 sigma.ali/upgrade-merge-annotations 列表中指定的 annotation
	// 同步 pod.beta1.sigma.ali/container-extra-config annotation
	mergeAnnotationKeys := sets.NewString(sigmak8sapi.AnnotationContainerExtraConfig)
	if len(upgradeMergeAnnotations) > 0 {
		mergeAnnotationKeys.Insert(strings.Split(upgradeMergeAnnotations, ",")...)
	}
	for _, key := range mergeAnnotationKeys.UnsortedList() {
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
			var foundInTemplate bool
			for j := range template.Spec.Containers {
				if template.Spec.Containers[j].Name == c.Name {
					foundInTemplate = true
					break
				}
			}
			isCommonVM := util.GetContainerEnvValue(c, "ali_run_mode") == "common_vm"
			if !foundInTemplate || !isCommonVM {
				continue
			}
			utilasi.AddContainerEnvHeadWithOverwrite(c, envVar.Name, envVar.Value)
		}
	}
}

func (c *asiControl) injectAdditionalLabelsIntoPod(pod *v1.Pod, labels map[string]string) {
	if labels == nil {
		return
	}
	for key, value := range labels {
		pod.Labels[key] = value
	}
}

func (c *asiControl) getAdditionalPodConfig(objectMeta *metav1.ObjectMeta, reader client.Reader) ([]v1.EnvVar, map[string]string) {
	if !needAdditionalEnvs(objectMeta) && !needAdditionalLabels(objectMeta) {
		return nil, nil
	}
	nodeGroup, err := skyline.QueryNodeGroupByName(reader, objectMeta.Labels[sigmak8sapi.LabelInstanceGroup])
	if err != nil {
		klog.Warningf("CloneSet %s/%s failed to query nodegroup for %v safetyout, err: %v",
			c.Namespace, c.Name, objectMeta.Labels[sigmak8sapi.LabelInstanceGroup], err)
		return nil, nil
	} else if nodeGroup == nil {
		klog.Warningf("CloneSet %s/%s failed to query nodegroup for %v safetyout, not found",
			c.Namespace, c.Name, objectMeta.Labels[sigmak8sapi.LabelInstanceGroup])
		return nil, nil
	}
	safetyOut := "0"
	if nodeGroup.SafetyOut == 1 {
		safetyOut = "1"
	}

	var additionalEnvs = make([]v1.EnvVar, 0)
	var additionalLabels = make(map[string]string)
	if needAdditionalEnvs(objectMeta) {
		additionalEnvs = append(additionalEnvs, v1.EnvVar{Name: envSafetyOut, Value: safetyOut})
	}
	if needAdditionalLabels(objectMeta) {
		additionalLabels[labelSafetyOut] = safetyOut
	}
	return additionalEnvs, additionalLabels
}

func needAdditionalEnvs(objectMeta *metav1.ObjectMeta) bool {
	return objectMeta.Annotations[sigmak8sapi.AnnotationEnableAppRulesInjection] == "true" &&
		objectMeta.Labels[sigmak8sapi.LabelInstanceGroup] != ""
}

func needAdditionalLabels(objectMeta *metav1.ObjectMeta) bool {
	return objectMeta.Annotations[annotationSafetyOutInjection] == "true" &&
		objectMeta.Labels[sigmak8sapi.LabelInstanceGroup] != ""
}

func (c *asiControl) ValidateCloneSetUpdate(oldCS, newCS *appsv1alpha1.CloneSet) error {
	isOldInPlaceOnly := oldCS.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType
	isNewInPlaceOnly := newCS.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType
	if isOldInPlaceOnly != isNewInPlaceOnly {
		if newCS.Labels["cloneset.asi/mode"] == "asi" {
			klog.Warningf("Currently we allow proxy CloneSet to change the update type. %s/%s from %s to %s.",
				c.Namespace, c.Name, oldCS.Spec.UpdateStrategy.Type, newCS.Spec.UpdateStrategy.Type)
			return nil
		}
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

func (c *asiControl) isPublishSuccess(status *appsv1alpha1.CloneSetStatus, pod *v1.Pod) bool {
	if !clonesetutils.EqualToRevisionHash("", pod, status.UpdateRevision) {
		return false
	}

	if c.IsPodUpdateReady(pod, c.Spec.MinReadySeconds) {
		return true
	}

	if pod.Status.Phase == v1.PodPending && status.UpdateRevision == pod.Labels[apps.ControllerRevisionHashLabelKey] {
		if c.Spec.UpdateStrategy.Type == appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
			_, schedCondition := podutil.GetPodCondition(&pod.Status, v1.PodScheduled)
			if schedCondition == nil || schedCondition.Status == v1.ConditionFalse {
				return true
			}
		}
	}

	return false
}

func (c *asiControl) isScheduledFailed(status *appsv1alpha1.CloneSetStatus, pod *v1.Pod) bool {
	if pod.Status.Phase == v1.PodPending && status.UpdateRevision == pod.Labels[apps.ControllerRevisionHashLabelKey] {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodScheduled && condition.Status == v1.ConditionFalse {
				return true
			}
		}
	}
	return false
}

// parsePodReason is similar to pkg/printers/internalversion/printers.go:printPod
func parsePodReason(pod *v1.Pod) string {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}
	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]
			if isIgnoredContainer(pod, container.Name) {
				continue
			}
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
			}
		}
		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			reason = "Running"
		}
	}
	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}
	return reason
}

func (c *asiControl) IgnorePodUpdateEvent(oldPod, curPod *v1.Pod) bool {
	if utilasi.GetPodSpecHashString(curPod) == "" {
		return false
	}

	if curPod.Generation != oldPod.Generation {
		return false
	}

	if lifecycleFinalizerChanged(c.CloneSet, oldPod, curPod) {
		return false
	}

	if c.IsPodUpdatePaused(oldPod) != c.IsPodUpdatePaused(curPod) {
		return false
	}

	if c.IsPodUpdateReady(oldPod, c.Spec.MinReadySeconds) != c.IsPodUpdateReady(curPod, c.Spec.MinReadySeconds) {
		return false
	}

	containsReadinessGate := func(pod *v1.Pod) bool {
		for _, r := range pod.Spec.ReadinessGates {
			if r.ConditionType == appspub.InPlaceUpdateReady {
				return true
			}
		}
		return false
	}

	if containsReadinessGate(curPod) {
		opts := c.GetUpdateOptions()
		opts = inplaceupdate.SetOptionsDefaults(opts)
		if err := opts.CheckPodUpdateCompleted(curPod); err == nil {
			if cond := inplaceupdate.GetCondition(curPod); cond == nil || cond.Status != v1.ConditionTrue {
				return false
			}
		}
	}

	return true
}

func isIgnoredContainer(pod *v1.Pod, name string) bool {
	if name == "" || pod == nil {
		return true
	}

	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			for _, env := range c.Env {
				if env.Name == "SIGMA_IGNORE_READY" {
					return true
				}
			}
		}
	}
	return false
}
