package api

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelPodReuseIp = "pod.beta1.sigma.ali/ip-reuse"

	// LabelPodReservedIp release pod with ipam/armory info not released.
	LabelPodReservedIp = "pod.beta1.sigma.ali/ip-reserved"

	LabelPodPool = "pod.beta1.sigma.ali/pool"
)

// AllocSpec contains specification of the desired allocation behavior of the pod.
// More info: https://lark.alipay.com/sigma.pouch/sigma3.x/sghayi
type AllocSpec struct {
	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`
	// List of containers belonging to the pod.
	// +optional
	Containers []Container `json:"containers,omitempty"`
}

// Affinity is a group of affinity scheduling rules.
type Affinity struct {
	// Describes pod anti-affinity scheduling rules extensions with added spread strategy logic.
	// +optional
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"`
	// Describes cpu anti-affinity scheduling rules between pods extensions with added spread strategy logic.
	// +optional
	CPUAntiAffinity *CPUAntiAffinity `json:"cpuAntiAffinity,omitempty"`
}

// A single application container that you want to run within a pod.
type Container struct {
	// Name of the container.
	// Must corresponds to one container in pod spec containers fields
	Name string `json:"name,omitempty"`
	// Extra attributes of resources required by this container.
	Resource ResourceRequirements `json:"resource,omitempty"`
	// Extra attributes of HostConfig required by this container.
	HostConfig HostConfigInfo `json:"hostConfig,omitempty"`
	// Reserved memory to keep alive.
	KeepAliveMemory *int64 `json:"keepAliveMemory,omitempty"`
}

// ResourceRequirements describes extra attributes of the compute resource requirements.
type ResourceRequirements struct {
	// If specified, extra cpu attributes such as cpuset modes
	// +optional
	CPU CPUSpec `json:"cpu,omitempty"`
	// If specified, extra gpu attributes such as gpu sharing mode
	// +optional
	GPU GPUSpec `json:"gpu,omitempty"`
}

// HostConfigInfo describes extra attributes of the HostConfig parameters.
type HostConfigInfo struct {
	// Path to cgroups under which the container’s cgroup is created.
	// If the path is not absolute, the path is considered to be relative to the cgroups path of the init process.
	// Cgroups are created if they do not already exist.
	CgroupParent string `json:"cgroupParent"`
	// Indicate how to set rootfs diskquota for a container.
	DiskQuotaMode RootDiskQuotaMode `json:"diskQuotaMode"`
	// Total memory limit (memory + swap); set -1 to enable unlimited swap.
	// You must use this with memory and make the swap value larger than memory
	MemorySwap int64 `json:"memorySwap,omitempty"`
	// Tune a container’s memory swappiness behavior. Accepts an integer between 0 and 100.
	MemorySwappiness int64 `json:"memorySwappiness,omitempty"`
	// Block IO weight (relative weight) accepts a weight value between 10 and 1000.
	// +optional
	BlkioWeight int64 `json:"blkioWeight,omitempty"`
	// Tune a container’s pids limit. Set -1 for unlimited.
	PidsLimit uint16 `json:"pidsLimit,omitempty"`
	// Cpu  priority of container
	CPUBvtWarpNs     int     `json:"cpuBvtWarpNs,omitempty"`
	MemoryWmarkRatio float64 `json:"memoryWmarkRatio,omitempty"`
	IntelRdtMba      string  `json:"intelRdtMba,omitempty"`
	IntelRdtGroup    string  `json:"intelRdtGroup,omitempty"`
	// Block IO weight (relative device weight) in the form of: "BlkioWeightDevice": [{"Path": "device_path", "Weight": weight}]
	BlkioWeightDevice []WeightDevice `json:"blkioWeightDevice,omitempty"`
	// Limit read rate (bytes per second) from a device in the form of: "BlkioDeviceReadBps": [{"Path": "device_path", "Rate": rate}]
	// for example: "BlkioDeviceReadBps": [{"Path": "/dev/sda", "Rate": "1024"}]"
	BlkioDeviceReadBps []ThrottleDevice `json:"blkioDeviceReadBps,omitempty"`
	// Limit write rate (bytes per second) to a device in the form of: "BlkioDeviceWriteBps": [{"Path": "device_path", "Rate": rate}]
	// for example: "BlkioDeviceWriteBps": [{"Path": "/dev/sda", "Rate": "1024"}]"
	BlkioDeviceWriteBps []ThrottleDevice `json:"blkioDeviceWriteBps,omitempty"`
	// Limit read rate (IO per second) from a device in the form of: "BlkioDeviceReadIOps": [{"Path": "device_path", "Rate": rate}]
	// for example: "BlkioDeviceReadIOps": [{"Path": "/dev/sda", "Rate": "1000"}]
	BlkioDeviceReadIOps []ThrottleDevice `json:"blkioDeviceReadIOps,omitempty"`
	// Limit write rate (IO per second) to a device in the form of: "BlkioDeviceWriteIOps": [{"Path": "device_path", "Rate": rate}]
	// for example: "BlkioDeviceWriteIOps": [{"Path": "/dev/sda", "Rate": "1000"}]
	BlkioDeviceWriteIOps []ThrottleDevice `json:"blkioDeviceWriteIOps,omitempty"`
	// List of ulimits to be set
	// for example: "ulimits: [{"Name": "nofile", "Hard": 8192, "Soft": 1024}]"
	Ulimits []Ulimit `json:"ulimits,omitempty"`
	// CPU CFS (Completely Fair Scheduler) period. Set this to overwrite agent default value
	CpuPeriod int64 `json:"cpuPeriod,omitempty"`
	// CPU CFS (Completely Fair Scheduler) quota. Default: 0 (not specified).
	CpuQuota int64 `json:"cpuQuota,omitempty"`
	// CPU shares (relative weight vs. other containers). Default: 0 (not specified).
	CpuShares int64 `json:"cpuShares,omitempty"`
	// DefaultCpuShares will be used when requests cpu is 0.
	DefaultCpuShares *int64 `json:"defaultCpuShares,omitempty"`
	// OOMScoreAdj adjusts the oom-killer score. Default: 0 (not specified).
	OomScoreAdj int64 `json:"oomScoreAdj,omitempty"`
}

type WeightDevice struct {
	Path   string
	Weight uint16
}

type ThrottleDevice struct {
	Path string
	Rate uint64
}

//  Ulimit is a human friendly version of Rlimit.
type Ulimit struct {
	// Name of ulimit.
	Name string
	// Hard limit of ulimit.
	Hard int64
	// Soft limit of ulimit.
	Soft int64
}

// DiskQuotaMode indicates how to set up container's rootfs diskquota.
// https://github.com/alibaba/pouch/blob/master/docs/features/pouch_with_diskquota.md#parameter-details
type RootDiskQuotaMode string

const (
	// DiskQuotaModeRootFsAndVolume is a DiskQuotaMode that indicates using ".*" to limit a container's rootfs.
	DiskQuotaModeRootFsAndVolume RootDiskQuotaMode = ".*"
	// DiskQuotaModeRootFsOnly is a DiskQuotaMode that indicates using "/" to limit a container's rootfs.
	DiskQuotaModeRootFsOnly RootDiskQuotaMode = "/"
)

// SpreadStrategy means how to allocate cpuset of container in the CPU topology
type SpreadStrategy string

const (
	// SpreadStrategySpread is the default strategy that favor cpuset allocation that spread
	// across physical cores
	SpreadStrategySpread SpreadStrategy = "spread"
	// SpreadStrategySameCoreFirst is the strategy that favor cpuset allocation that pack
	// in few physical cores
	SpreadStrategySameCoreFirst SpreadStrategy = "sameCoreFirst"
)

type CPUBindingStrategy string

const (
	// CPUBindStrategyDefault is a BindingStrategy that indicates kubelet binds this container according to CPUIDs
	CPUBindStrategyDefault CPUBindingStrategy = ""
	// CPUBindStrategyAllCPUs is a BindingStrategy that indicates kubelet binds this container to all cpus
	CPUBindStrategyAllCPUs CPUBindingStrategy = "BindAllCPUs"
)

// CPUSpec contains the extra attributes of CPU resource
type CPUSpec struct {
	// BindStrategy indicate kubelet how to bind cpus
	// If BindStrategy is "", that means using default binding logic as usual
	BindingStrategy CPUBindingStrategy `json:"bindingStrategy,omitempty"`
	// If specified, cpu resource of container is allocated as cpusets, and the CPUSet fields contains information
	// about CPUSet. The cpu resource request and limit value must be integer.
	// If not specified, cpu resource of container is allocated as cpu shares.
	// +optional
	CPUSet *CPUSetSpec `json:"cpuSet,omitempty"`
}

// CPUSetSpec contains extra attributes of cpuset allocation
type CPUSetSpec struct {
	// If specified, cpuset allocation strategy
	// +optional
	SpreadStrategy SpreadStrategy `json:"spreadStrategy,omitempty"`
	// the logic cpu IDs allocated to the container
	// if not specified, scheduler wil fill with allocated cpu IDs.
	// +optional
	CPUIDs []int `json:"cpuIDs,omitempty"`
}

// GPUShareMode means how multiple containers use one GPU
type GPUShareMode string

const (
	// GPUShareModeExclusive is the default mode that allow only one container is allowed to run on the GPU
	GPUShareModeExclusive GPUShareMode = "exclusive"
	// GPUShareModeExclusive is the default mode that allow only one container is allowed to run on the GPU
	GPUShareModeEShare GPUShareMode = "share"
)

// GPUSpec contains the GPU specification other than the number of GPUs
type GPUSpec struct {
	// GPUShareMode is the sharing strategy for multiple container use
	ShareMode GPUShareMode `json:"shareMode,omitempty"`
	// use cases:
	// 1.if only use requestMem, scheduler must find a gpu device have enough mem for it
	// 2.if only use requestCount, scheduler master find requestCount's unused gpu device for it
	// 3.if use requ
	RequestMem   int64 `json:"requestMem,omitempty"`
	RequestCount int64 `json:"requestCount,omitempty"`

	AllocatedGPUInfos []*AllocatedGPUInfo `json:"allocatedGPUInfos,omitempty"`
}

type AllocatedGPUInfo struct {
	UUID string `json:"uuid,omitempty"`
	// gpu card number, example(0, 1, 2)
	Minor        uint  `json:"minor,omitempty"`
	Mem          int64 `json:"mem,omitempty"`
	ComputeMilli int64 `json:"compute,omitempty"`
	DecodeMilli  int64 `json:"decode,omitempty"`
	EncodeMilli  int64 `json:"encode,omitempty"`
	IsShare      bool  `json:"isShare"`
}

// Pod anti affinity is a group of inter pod anti affinity scheduling rules.
type PodAntiAffinity struct {
	// Extensions of the RequiredDuringSchedulingIgnoredDuringExecution fields of k8s PodAntiAffinity struct,
	// extra fields is added to support more hard rules of pod spreading logic.
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution []PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// Extensions of the PreferredDuringSchedulingIgnoredDuringExecution fields of k8s PodAntiAffinity struct,
	// extra fields is added to support more soft rules of pod spreading logic.
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// CPUAntiAffinity is a group of CPU anti affinity rules between pods
type CPUAntiAffinity struct {
	// Extensions of the PreferredDuringSchedulingIgnoredDuringExecution fields of k8s PodAntiAffinity struct,
	// extra fields is added to support more soft rules of cpu spreading logic.
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []v1.WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAffinityTerm contains the extensions to the k8s PodAffinityTerm struct
type PodAffinityTerm struct {
	v1.PodAffinityTerm `json:",inline"`

	// MaxCount is the maximum "allowed" number of pods in the scope specified by the value of topologyKey,
	// which are selected by LabelSelector and namespaces，not including the pod to be scheduled. If more selected pods
	// are located in a single value of topologyKey, the allocation will be rejected by the scheduler.
	// Defaults to 0, which means no more selected pods is allowed in the scope specified by the value of topologyKey
	// +optional
	MaxCount int64 `json:"maxCount,omitempty"`
	// MaxPercent is the maximum "allowed" percentage of pods in the scope specified by the value of topologyKey,
	// which are selected by LabelSelector and namespaces, not including the pod to be scheduled.
	// If more pods are located in a single value of topologyKey, the allocation will be rejected by the scheduler
	// Defaults to 0, which means no more selected pods is allowed in the scope specified by the value of topologyKey.
	// Values should be in the range 1-100.
	// +optional
	MaxPercent int32 `json:"maxPercent,omitempty"`
}

// WeightedPodAffinityTerm contains the extensions to the k8s WeightedPodAffinityTerm struct
type WeightedPodAffinityTerm struct {
	v1.WeightedPodAffinityTerm `json:",inline"`
	// MaxCount is the maximum "allowed" number of pods in the scope specified by the value of topologyKey,
	// which are selected by LabelSelector and namespaces，not including the pod to be scheduled.
	// If more pods are located in a single value of topologyKey, the allocation will be disfavored
	// but not rejected by the scheduler
	// Defaults to 0, which means no more selected pods is allowed in the scope specified by the value of topologyKey
	// +optional
	MaxCount int64 `json:"maxCount,omitempty"`
	// MaxPercent is the maximum "allowed" percentage of pods in the scope specified by the value of topologyKey,
	// which are selected by LabelSelector and namespaces，not including the pod to be scheduled.
	// If more pods are located in a single value of topologyKey, the allocation will be disfavored but not rejected
	// by the scheduler
	// Defaults to 0, which means no more selected pods is allowed in the scope specified by the value of topologyKey.
	// Values should be in the range 1-100.
	// +optional
	MaxPercent int32 `json:"maxPercent,omitempty"`
}

// NetworkStatus contains the network configuration for the pod. It can be used by cni plugin to reuse IP configuration
// for the same pod name.
type NetworkStatus struct {
	// Backend IPAM provider name
	// +optional
	IPAMName string `json:"ipam,omitempty"`
	// Virtual lan id, e.g. 701
	// +optional
	VlanID string `json:"vlan,omitempty"`
	// The number of bits set in the subnet mask
	// +optional
	NetworkPrefixLength int32 `json:"networkPrefixLen,omitempty"`
	// The default gateway applied to the pod
	// +optional
	Gateway string `json:"gateway,omitempty"`
	// The MAC address of the pod
	// +optional
	MACAddress string `json:"macAddress,omitempty"`
	// Virtual Port ID
	// +optional
	VPortID string `json:"vPortID,omitempty"`
	// Virtual Port token
	// +optional
	VPortToken string `json:"vPortToken,omitempty"`
	// Virtual Switch ID, e.g. vsw-uf6v8mhdbxsnj0ckakqrb
	// +optional
	VSwitchID string `json:"vSwitchID,omitempty"`
	// Elastic network interface ID, e.g. eni-abcdxxxx
	// +optional
	EnID string `json:"enID,omitempty"`
	// The network mode, e.g. bridge, vlan, overlay, ecs, used by xnet, naming controller etc.
	// +optional
	NetType string `json:"netType,omitempty"`
	// Overlay network version, e.g. 1.0
	// +optional
	OverlayNetworkVer string `json:"overlayNetworkVer,omitempty"`
	// Pod ip label, e.g. stage/unit
	// +optional
	IpLabel string `json:"ipLabel,omitempty"`
	// Pod sandbox container id
	SandboxId string `json:"sandboxId"`
	// Ip, e.g. 11.172.32.180
	Ip string `json:"ip"`
	// Ip allocate time
	AllocationTimestamp *metav1.Time `json:"allocationTimestamp,omitempty"`
	// Ip release time
	ReleaseTimestamp *metav1.Time `json:"releaseTimestamp,omitempty"`
	// Ip security domain
	SecurityDomain string `json:"securityDomain,omitempty"`
	// Ipv6
	IPv6 string `json:"ipv6,omitempty"`
	// Maskv6
	Maskv6 string `json:"maskv6,omitempty"`
	// Gatewayv6
	Gatewayv6 string `json:"gatewayv6,omitempty"`
	// ENI Region
	RegionId string `json:"regionId,omitempty"`
	// ENI Zone
	ZoneId string `json:"zoneId,omitempty"`
	// ENI VpcId
	VpcId string `json:"vpcId,omitempty"`
	// ENI SecurityGroupId
	SecurityGroupId string `json:"securityGroupId,omitempty"`
	// UseUnderlayEni Pod uses underlay-eni
	UseUnderlayEni bool `json:"useUnderlayEni,omitempty"`
	// used by alinet
	CniNetType string `json:"cniNetType,omitempty"`
	// used by cni-service and scheduler
	NetworkResourceType string `json:"networkResourceType,omitempty"`

	// 指定容器在skyline 中的设备类型
	DeviceTypeInCMDB string `json:"deviceTypeInCMDB,omitempty"`

	// 网卡CR的名称
	NetworkInterfaceName string `json:"networkInterfaceName,omitempty"`

	// RDMA 的QP值
	RdmaInterfaceQueuePairNumber int `json:"rdmaInterfaceQueuePairNumber,omitempty"`
}

// DanglingPod is a kind of pod that is removed from apiserver but still running on node.
// DanglingPod's infomation is stored in node's annotation.
type DanglingPod struct {
	// Name of the DanglingPod, recorded in container's label
	Name string `json:"name"`
	// Namespace of the DanglingPod, recorded in container's label
	Namespace string `json:"namespace"`
	// UID of the DanglingPod, recorded in container's label
	UID string `json:"uid"`
	// SN is used to register peripheral system such as armory
	// +optional
	SN string `json:"sn,omitempty"`
	// The time pod is created
	// +optional
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty"`
	// Ip address of DanglingPod
	// +optional
	PodIP string `json:"podIP,omitempty"`
	// The Phase of DanglingPod
	// +optional
	Phase v1.PodPhase `json:"phase,omitempty"`
	// If SafeToRemove is true,Sigmalet will delete dangling pod
	SafeToRemove bool `json:"safeToRemove"`
}

type PodDesiredStateSpec struct {
	ID               string                      `json:"ID"`
	UpstreamIdentity string                      `json:"upstreamIdentity"`
	ContainerStates  map[string]ContainerState   `json:"containerStates,omitempty"`
	RestartSpec      *PodDesiredStateRestartSpec `json:"restartSpec,omitempty"`
}

type PodDesiredStateRestartSpec struct {
	Containers []string `json:"containers"`
	Timeout    string   `json:"timeout,omitempty"`
}

const (
	// ContainerDiskPressure means the pod container is under disk pressure.
	ContainerDiskPressure v1.PodConditionType = "ContainerDiskPressure"
)

const (
	// ContainerStartInParallel means the containes will be started in parallel
	ContainerStartInParallel = "parallel"
)
