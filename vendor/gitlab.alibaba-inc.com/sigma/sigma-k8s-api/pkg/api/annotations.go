/*
Copyright 2017 The Kubernetes Authors.

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

package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const (
	// When create, start or stop pod, use this annotation to represent desired state
	AnnotationContainerStateSpec = "pod.beta1.sigma.ali/container-state-spec"

	// When update or upgrade pod, store the old pod spec in this annotation
	// makes rollback controller can do rollback when operate failed
	AnnotationPodLastSpec = "pod.beta1.sigma.ali/last-spec"

	// Update result should be stored in this annotation
	AnnotationPodUpdateStatus = "pod.beta1.sigma.ali/update-status"

	// PodInplaceUpdateState is used to store inplace update state.
	// The state should be one of "created"/"accepted"/"failed".
	// https://yuque.antfin-inc.com/sys/sigma3.x/inplace-update-design-doc
	AnnotationPodInplaceUpdateState = "pod.beta1.sigma.ali/inplace-update-state"

	// PodInplaceUpdateStateV2 is used to store inplace update state(v2).
	// The state should be one of "created"/"accepted"/"rejected".
	// https://yuque.antfin-inc.com/sys/sigma3.x/yiu0yp
	AnnotationPodInplaceUpdateStateV2 = "pod.beta1.sigma.ali/inplace-update-state-v2"
	// request of inplace update
	AnnotationPodInplaceUpdateContainerResourceRequest = "pod.beta1.sigma.ali/container-resource-request"

	AnnotationLocalInfo = "node.beta1.sigma.ali/local-info"

	AnnotationPodAllocSpec = "pod.beta1.sigma.ali/alloc-spec"

	AnnotationPodRequestAllocSpec = "pod.beta1.sigma.ali/request-alloc-spec"

	AnnotationPodNetworkStats = "pod.beta1.sigma.ali/network-status"

	AnnotationPodNetworkStatsHistory = "pod.beta1.sigma.ali/network-status-history"

	// numeric number of network priority
	// http://docs.alibaba-inc.com/pages/viewpage.action?pageId=479572415
	AnnotationNetPriority = "pod.beta1.sigma.ali/net-priority"

	// AnnotationPodSpecHash is a pod spec hash string provided by user
	AnnotationPodSpecHash = "pod.beta1.sigma.ali/pod-spec-hash"

	// Deprecated: please use LabelPodRegisterNamingState
	AnnotationPodRegisterNamingState = "pod.beta1.sigma.ali/naming-register-state"

	// AnnotationAutopilot is the prefix of autopilot service in node annotation
	AnnotationAutopilot = "node.beta1.sigma.ali/autopilot"

	// AnnotationDanglingPods records the dangling pods
	// Please refer to: https://lark.alipay.com/sys/sigma3.x/iqymrh
	AnnotationDanglingPods = "node.beta1.sigma.ali/dangling-pods"

	// AnnotationRebuildContainerInfo is container info which from sigma 2.0 container
	AnnotationRebuildContainerInfo = "pod.beta1.sigma.ali/rebuild-container-info"

	// AnnotationPodHostNameTemplate is pod hostname template which used to generate hostname.
	AnnotationPodHostNameTemplate = "pod.beta1.sigma.ali/hostname-template"

	// AnnotationPodHostNameTemplateSuffix is pod hostname template suffix which is used to generate hostname template.
	AnnotationPodHostNameTemplateSuffix = "pod.beta1.sigma.ali/hostname-template-suffix"

	// AnnotationNodeCPUSharePool is annotation key of the cpu share pool of Node API
	AnnotationNodeCPUSharePool = "node.beta1.sigma.ali/cpu-sharepool"

	// AnnotationContainerExtraConfig is annotation key of container's config defined by user
	AnnotationContainerExtraConfig = "pod.beta1.sigma.ali/container-extra-config"

	// AnnotationContainerCPUQuotaUnlimit is annotation key of container that cpu.quota is set to unlimit
	AnnotationContainerCPUQuotaUnlimit = "pod.beta1.alibabacloud.com/container-cpu-quota-unlimit"

	// AnnotationContainerMemoryUnlimit is annotation key of container that memory.limit_in_bytes is set to unlimit
	AnnotationContainerMemoryUnlimit = "pod.beta1.alibabacloud.com/container-memory-unlimit"

	// AnnotationContainerCPUQuotaZoomIn will make cpu quota zoom in
	AnnotationContainerCPUQuotaZoomIn = "pod.beta1.alibabacloud.com/zoom-in-cpu-quota"

	// AnnotationPodRequestACU indicates whether pod request resource by ACU
	AnnotationPodRequestACU = "pod.alibabacloud.com/request-acu"

	// AnnotationPodPendingTimeSeconds is annotation key of pod pending, with this key,
	// sigmalet will skip pod create, the value is timeout seconds, zero represent without limit
	AnnotationPodPendingTimeSeconds = "pod.beta1.sigma.ali/pending-time-seconds"

	// AnnotationDisableCascadingDeletion indicates whether such resource disabled cascading-deletion
	AnnotationDisableCascadingDeletion = "sigma.ali/disable-cascading-deletion"

	// AnnotationEnableAppRulesInjection indicates whether to inject apprules into this resource.
	AnnotationEnableAppRulesInjection = "sigma.ali/enable-apprules-injection"

	// AnnotationContainerDiskQuotaID is container diskQuotaID
	AnnotationContainerDiskQuotaID = "sigma.ali/container-diskQuotaID"

	//AnnotationDisableOverquotaFilter indicates whether to ignore the overquota label.
	AnnotationDisableOverquotaFilter = "sigma.ali/disable-over-quota-filter"

	// AnnotationResourceDeletingConfirmed indicates if the Resources is confirmed to deleting.
	// If the Resource has this annotation, admission will REJECT deleting operation if the annotation value is NOT true
	// For now, only implemented Resources: Pod
	AnnotationResourceDeletingConfirmed = "sigma.ali/deleting-confirmed"

	// AnnotationResourceDeletingConfirmed indicates if the namespaced Resources need to inject AnnotationResourceDeletingConfirmed
	// This annotation will only be set to Namespace
	// If the Namespace has this annotation and the value is true, when the Resource created in this Namespace,
	// then Resource will be injected AnnotationResourceDeletingConfirmed and value is false
	// For now, only these Resources will be injected AnnotationResourceDeletingConfirmed: Pod
	AnnotationResourceInjectDeletingConfirmed = "sigma.ali/inject-deleting-confirmed"

	// AnnotationResourceInjectTraceID indicates if the namespaced Resources need to inject AnnotationKeyTraceID
	// The default action is injection, only if the value is "disable" the injection will be disabled
	AnnotationResourceInjectTraceID = "sigma.ali/inject-trace-id"

	// AnnotationKeyTraceID is annotation key for traceID
	AnnotationKeyTraceID = "pod.beta1.sigma.ali/trace-id"
	// AnnotationKeyTrace annotation key for trace content
	AnnotationKeyTrace = "pod.beta1.sigma.ali/trace"
	// AnnotationKeyCompressedTrace annotation key for compressed trace content(gzip+base64)
	AnnotationKeyCompressedTrace = "pod.beta1.sigma.ali/gzip-trace"

	// AnnotationPodDesiredStateSpec is pod desired state spec
	AnnotationPodDesiredStateSpec = "pod.beta1.sigma.ali/desired-state-spec"

	// AnnotationAppGroupAutoCreation is pod auto create appGroup
	AnnotationAppGroupAutoCreation = "pod.beta1.sigma.ali/appgroup-auto-creation"

	// If true, will update quota spec without admission check.
	AnnotationForceUpdateQuota = AlibabaCloudPrefix + "/force-update-quota"

	// AnnotationQuotaAccess controls accessibility of quota.
	AnnotationQuotaAccessible = AlibabaCloudPrefix + "/quota-accessible"

	// If "true", the user related info will be retained during upgrading whatever
	// the status of feature gate DisableUserInfoRetainDuringUpgrade
	AnnotationForceRetainUserInfo = "pod.beta1.sigma.ali/force-retain-user-info"

	// AnnotationInplaceUpgradeMergeAnnotations defines the user-specified annotations which will be merged
	AnnotationInplaceUpgradeMergeAnnotations = "sigma.ali/upgrade-merge-annotations"

	// AnnotationPodResourceConfig defines pod-level cgroup resource config
	AnnotationPodResourceConfig = "pod.beta1.sigma.ali/pod-resource-config"

	// AnnotationPodRawResource defines raw request resource in json format of each pod container.
	AnnotationPodRawResource = "pod.beta1.sigma.ali/raw-request-resource"

	// AnnotationNodeColocationConfig defines node colocation config.
	AnnotationNodeColocationConfig = AlibabaCloudPrefix + "/node-colocation-config"

	// AnnotationPVCloudDiskInfo defines cloud disk info and value is in json format.
	AnnotationPVCloudDiskInfo = "pv.sigma.ali/cloud-disk-info"

	// AnnotationContainerStartPolicy
	AnnotationContainerStartPolicy = "pod.sigma.ali/container-start-policy"
)

//compatible to old sigma annotation
func GetCompatibleAnnotation(obj metav1.Object, key string) string {
	if strings.Contains(key, LegacyPrefix) {
		if v, ok := obj.GetAnnotations()[strings.Replace(key, LegacyPrefix, AlibabaCloudPrefix, 1)]; ok {
			return v
		}
	}
	return obj.GetAnnotations()[key]
}

func GetBackwardsCompatibleAnnotation(meta metav1.ObjectMeta, key string) string {
	return meta.Annotations[strings.Replace(key, LegacyPrefix, AlibabaCloudPrefix, 1)]
}

func SetBackwardsCompatibleAnnotations(meta metav1.ObjectMeta, key string, value string) {
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	meta.Annotations[strings.Replace(key, LegacyPrefix, AlibabaCloudPrefix, 1)] = value
	meta.Annotations[strings.Replace(key, AlibabaCloudPrefix, LegacyPrefix, 1)] = value
}
