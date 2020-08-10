package api

import (
	"k8s.io/api/core/v1"
)

// SigmaQOSClass defines the supported qos classes of Pods.
type SigmaQOSClass string

const (
	// SigmaQOSDedicated is the Dedicated qos class.
	SigmaQOSDedicated SigmaQOSClass = "SigmaDedicated"

	// SigmaQOSGuaranteed is the Guaranteed qos class.
	SigmaQOSGuaranteed SigmaQOSClass = "SigmaGuaranteed"

	// SigmaQOSBurstable is the Burstable qos class.
	SigmaQOSBurstable SigmaQOSClass = "SigmaBurstable"

	// SigmaQOSBestEffort is the BestEffort qos class.
	SigmaQOSBestEffort SigmaQOSClass = "SigmaBestEffort"

	// SigmaQOSJob is the sigma job qos class.
	SigmaQOSJob SigmaQOSClass = "SigmaJob"

	// SigmaQOSCompressible is the Compressible qos class.
	SigmaQOSCompressible SigmaQOSClass = "SigmaCompressible"

	// SigmaQOSNone is the undefined qos class.
	SigmaQOSNone SigmaQOSClass = ""
)

func GetPodQOSClass(pod *v1.Pod) SigmaQOSClass {
	if v, ok := pod.Labels[LabelPodQOSClass]; ok {
		switch SigmaQOSClass(v) {
		case SigmaQOSDedicated, SigmaQOSGuaranteed, SigmaQOSBurstable, SigmaQOSBestEffort, SigmaQOSCompressible:
			return SigmaQOSClass(v)
		}
	}
	return SigmaQOSNone
}
