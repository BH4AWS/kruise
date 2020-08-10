package api

import (
	v1 "k8s.io/api/core/v1"
)

type AliQoSClass string

const (
	// AliQoSLSR is the latency-sensitive reserved ali qos class.
	AliQoSLSR AliQoSClass = "LSR"
	// AliQoSLS is the latency-sensitive ali qos class.
	AliQoSLS AliQoSClass = "LS"
	// AliQoSBE is the best effort qos class.
	AliQoSBE AliQoSClass = "BE"
	// AliQoSVMEnv is the special qos class for vm-like pod.
	AliQoSVMEnv AliQoSClass = "VMEnv"
	// AliQoSSYSTEM is the special qos class for critical daemonset pod, like slo-agent, alinet etc.
	// AliQoSSYSTEM pods would be managed in a new cgroup-parents(/system) restricting total resource usages
	// and assigned other characteristics like skip kubelet admissions.
	AliQoSSYSTEM AliQoSClass = "SYSTEM"
	// AliQoSNone is none.
	AliQoSNone AliQoSClass = ""
)

func GetPodAliQoSClass(pod *v1.Pod) AliQoSClass {
	if qos, ok := pod.Labels[LabelPodAliQOSClass]; ok {
		switch AliQoSClass(qos) {
		case AliQoSLSR, AliQoSLS, AliQoSBE, AliQoSVMEnv, AliQoSSYSTEM:
			return AliQoSClass(qos)
		}
	}
	// If pod has not ali qos label, default as none-qos pod.
	return AliQoSNone
}
