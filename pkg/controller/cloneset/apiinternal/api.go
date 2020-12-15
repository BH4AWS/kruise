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

package apiinternal

// Labels and Annotations for Pod
const (
	LabelPodUpgradeCanary            = "pod.beta1.sigma.ali/upgrade-canary"
	LabelPodUpgradePostpone          = "pod.beta1.sigma.ali/upgrade-postpone"
	LabelPodBatchAdoption            = "batchplan.beta1.sigma.ali/pod-state"
	AnnotationPausePodUpgrade        = "inplaceset.beta1.sigma.ali/pause-pod-upgrade"
	LabelPodUpgradeBatchOrder        = "inplaceset.beta1.sigma.ali/upgrade-batch-order"
	LabelPodUpgradePriority          = "pod.beta1.sigma.ali/upgrade-priority"
	LabelFinalStateUpgrading         = "inplaceset.beta1.sigma.ali/final-state-upgrading"
	AnnotationUpgradeSpec            = "pod.beta1.sigma.ali/upgrade-spec"
	AnnotationPodUpgradeTimeout      = "pod.beta1.sigma.ali/upgrade-timeout"
	AnnotationPodInjectNameAsSN      = "pods.sigma.alibaba-inc.com/inject-name-as-sn"
	LabelPodUpgradingState           = "pod.beta1.sigma.ali/upgrading-state"
	PodUpgradingExecuting            = "Executing"
	PodUpgradingSucceeded            = "Succeeded"
	AnnotationPodDebugContext        = "pod.beta1.sigma.ali/debug-context"
	AnnotationPublishSuccessReplicas = "cloneset.beta1.sigma.ali/publish-success-replicas"
	AnnotationAppsPublishId          = "apps.alibabacloud.com/publish-id"
)

const (
	// Specified env to force container restart
	EnvDebugRestartTimestamp = "POD_DEBUG_RESTART_TIMESTAMP"
	EnvAliRunInit            = "ali_run_init"

	// For ControllerRevision
	AnnotationRevisionFullTemplate = "cloneset.asi/full-template"
)
