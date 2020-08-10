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
	LegacyPrefix       = "sigma.ali"
	AlibabaCloudPrefix = "alibabacloud.com"

	// serial tag of pod, should be unique globally
	LabelPodSn = "sigma.ali/sn"

	// ip allocated for pod
	LabelPodIp = "sigma.ali/ip"

	// application name
	LabelAppName = "sigma.ali/app-name"

	// group of pod in cmdb like armory
	LabelInstanceGroup = "sigma.ali/instance-group"

	//device_model.model_name in cmdb like skyline
	LabelDeviceModel = AlibabaCloudPrefix + "/device-model"

	// application deploy unit, equal to app name + env in ant sigma.
	LabelDeployUnit = "sigma.ali/deploy-unit"

	// site of pod
	LabelSite = "sigma.ali/site"

	// Physical core as the topology key.
	// It is used to spread cpuset assignment across physical cores.
	TopologyKeyPhysicalCore = "sigma.ali/physical-core"

	// Logical core as the topology key.
	// It is used to unshare a single logical core in over-quota case.
	TopologyKeyLogicalCore = "sigma.ali/logical-core"

	// pod container mode, e.g. dockervm/pod/...
	LabelPodContainerModel = "sigma.ali/container-model"

	// qos of pod. label value is defined in qosclass
	LabelPodQOSClass = "sigma.ali/qos"

	// aliqos of pod. label value is defined in qosclass
	LabelPodAliQOSClass = AlibabaCloudPrefix + "/qos"

	// PriorityClass of pod SLO specified.
	LabelPriorityClass = AlibabaCloudPrefix + "/priority-class"

	//workload-type for alibabaCloud
	LabelWorkLoad = AlibabaCloudPrefix + "/workload-type"

	// if true, user can modify /etc/hosts, /etc/resolv.conf and /etc/hostname in container .
	LabelHostDNS = "ali.host.dns"

	// the type of container.
	LabelServerType = "com.alipay.acs.container.server_type"

	// pod group of pod
	LabelPodGroupName = "scheduling.sigma.ali/pod-group-name"

	// If true, the pod is a batch job.
	LabelPodIsJob = "meta.k8s.alipay.com/job"

	// If true, the pod will be scheduled by dynamic method.
	LabelPodUseDynamic = "meta.k8s.alipay.com/dynamic"

	// Quota name of pod
	LabelQuotaName = AlibabaCloudPrefix + "/quota-name"

	// cmdb state of this pod
	LabelPodRegisterNamingState = "pod.beta1.sigma.ali/naming-register-state"

	// If true, the pod is support preemption
	LabelPodPreemptible = AlibabaCloudPrefix + "/preemptible"

	// If true, means the pod want to be evicted
	LabelPodToBeEvicted = AlibabaCloudPrefix + "/to-be-evicted"

	// workload type of pod, e.g. job/streaming/service/serverless
	LabelWorkloadType = "meta.k8s.alipay.com/workload"

	// scaling type of pod, e.g. elastic-job/batch-job
	LabelScalingType = "meta.k8s.alipay.com/scaling-type"

	// LabelIgnoreCMDB indicates if the Pod need to skip the CMDB registration.
	LabelIgnoreCMDB = "meta.k8s.alipay.com/ignore-cmdb"

	// LabelIgnoreNaming indicates if the Pod need to skip the Naming registration.
	LabelIgnoreNaming = "meta.k8s.alipay.com/ignore-naming"

	// If true, the service envs won't be injected into container.
	// Deprecated: use EnableServiceLinks field instead.
	LabelDisableServiceLinks = "sigma.ali/disable-service-links"

	// LabelUpstreamComponent indicates the identity of upstream
	LabelUpstreamComponent = "sigma.ali/upstream-component"

	// If true, will not inject network envs into container.
	LabelDisableNetworkEnvInjection = "sigma.ali/disable-network-env-injection"

	// LabelResourceOwner indicates the owner of the resource
	LabelResourceOwner = "alibabacloud.com/owner"
)

//compatible to old sigma label
func GetCompatibleLabel(obj metav1.Object, key string) string {
	if strings.Contains(key, LegacyPrefix) {
		if v, ok := obj.GetLabels()[strings.Replace(key, LegacyPrefix, AlibabaCloudPrefix, 1)]; ok {
			return v
		}
	}
	return obj.GetLabels()[key]
}

func GetBackwardsCompatibleLabel(meta metav1.ObjectMeta, key string) string {
	return meta.Labels[strings.Replace(key, LegacyPrefix, AlibabaCloudPrefix, 1)]
}

func SetBackwardsCompatibleLabels(meta metav1.ObjectMeta, key string, value string) {
	if meta.Labels == nil {
		meta.Labels = make(map[string]string)
	}
	meta.Labels[strings.Replace(key, LegacyPrefix, AlibabaCloudPrefix, 1)] = value
	meta.Labels[strings.Replace(key, AlibabaCloudPrefix, LegacyPrefix, 1)] = value
}
