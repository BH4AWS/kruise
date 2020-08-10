package api

import v1 "k8s.io/api/core/v1"

// see node api extensions docs:
// https://lark.alipay.com/sigma.pouch/sigma3.x/tsmf32
const (
	ResourceACU    v1.ResourceName = "alibabacloud.com/acu"
	ResourceFPGA   v1.ResourceName = "resource.sigma.ali/fpga"
	ResourceGPUMem v1.ResourceName = "resource.sigma.ali/gpu-mem"

	ResourceGPUCompute v1.ResourceName = AlibabaCloudPrefix + "/gpu-compute"
	ResourceGPUDecode  v1.ResourceName = AlibabaCloudPrefix + "/gpu-decode"
	ResourceGPUEncode  v1.ResourceName = AlibabaCloudPrefix + "/gpu-encode"
	ResourceGPUMemory  v1.ResourceName = AlibabaCloudPrefix + "/gpu-mem"
	ResourceGPUCount   v1.ResourceName = AlibabaCloudPrefix + "/gpu-count"

	// node IP, useful to specify scheduling target in NodeAffinty
	LabelNodeIP string = "sigma.ali/node-ip"

	// unique service tag of node
	LabelNodeSN string = "sigma.ali/node-sn"

	// reuse kubernetes host pkg/kubelet/apis
	LabelHostname string = "kubernetes.io/hostname"

	// node hostname label, used to record node hostname from armory.
	LabelNodeArmoryHostname string = "sigma.ali/armory-hostname"

	// Parent Service Tag of machine is the service tag of hosting device, e.g. parent service of a container is the node,
	// the parent service tag of a node usually means the frame of server
	LabelParentServiceTag string = "sigma.ali/parent-service-tag"

	// network topology related labels
	// see https://www.atatech.org/articles/73242?spm=a1z2e.8101737.webpage.dtitle4.4fe56a6cJnTUu9

	//e.g. H21
	LabelRack string = "sigma.ali/rack"
	//e.g. A2-3.Eu6
	LabelRoom string = "sigma.ali/room"
	//e.g. EU6-MASTER
	LabelNetLogicSite string = "sigma.ali/net-logic-site"

	//access switch, aka, top of rack, e.g. ASW-A5-1-H19.ET15
	LabelASW string = "sigma.ali/asw"
	// Points of Delivery, e.g. NA62-MASTER_NA62-LSW-MASTER-INTER-G1_0.01_0.01,
	// a group of access switches which together form the minimum network construction unit in Alibaba,
	LabelPOD string = "sigma.ali/pod"
	// Logic Points of Delivery, e.g. NA62-MASTER_NA62-LSW-MASTER-INTER-G1_0.01
	// a group of POD switches
	LabelLogicPOD string = "sigma.ali/logic-pod"
	// DSW: datacenter interchange switch, a.k.a core switch, e.g. EU6-APPDB-G1
	// Overall network topology relation is DSW > LPOD > POD > ASW
	LabelDSWCluster string = "sigma.ali/dsw-cluster"

	// security domain, nodes in different security domain cannot be reached for security reason,
	// e.g. ALI_PRODUCT, ALI_TEST, ALIYUN_PUBLIC
	LabelSecurityDomain string = "sigma.ali/security-domain"

	// reuse kubernetes failure domain, e.g. cn-shenzhen
	// a single sigma cluster can run in multiple zones, but only within the same region
	LabelRegion string = "failure-domain.beta.kubernetes.io/region"
	LabelZone   string = "failure-domain.beta.kubernetes.io/zone"

	// ip range of node, e.g. 10.183.196.0_22
	LabelPhyIPRange string = "sigma.ali/physic-ip-range"

	// boolean labels with value of true|false
	// whether related node is an ecs vm
	LabelIsECS string = "sigma.ali/is-ecs"
	// whether related node allows resource over-subscription
	LabelEnableOverQuota string = "sigma.ali/is-over-quota"
	// whether related node is directly accessible
	LabelIsPubNetServer string = "sigma.ali/is-pub-net"
	// whether related node support co-locating with batch jobs
	LabelIsMixRun string = "sigma.ali/is-mixrun"
	// whether related node support overlay network
	LabelIsOverlayNetwork string = "sigma.ali/is-overlay-network"
	// whether related node support overlay sriov
	LabelIsOverlaySriov string = "sigma.ali/is-overlay-sriov"
	// whether related node support elastic network interface
	LabelIsENI string = "sigma.ali/is-eni"
	// whether the whole resource of node will be allocated to one and only one pod
	// in host network mode.
	LabelIsHost string = "sigma.ali/is-host"

	// e.g sigma_public
	LabelResourcePool string = "sigma.ali/resource-pool"
	// e.g. 3.0, 3.5 or 4.0
	LabelNetArchVersion string = "sigma.ali/net-arch-version"
	// e.g. WM
	LabelNetCardType string = "sigma.ali/net-card-type"
	// short machine model name, e.g. A8, F53
	LabelMachineModel string = "sigma.ali/machine-model"
	// specific os identifier, such as alios7u2, alios5u7
	// note that well known label beta.kubernetes.io/os gives only broad os classification
	LabelOS string = "sigma.ali/os"

	// specific kernel identifier, such as 3.1, 4.9
	LabelKernel string = "sigma.ali/kernel"

	// cpuspce low/standard/high
	LabelCpuSpec string = "sigma.ali/cpu-spec"

	// enable cpu share
	LabelCpuShare string = "sigma.ali/is-cpu-share"

	// storage media type of ephemeral storage which usually located in docker root disk,
	// valid value includes: ssd,hdd
	LabelEphemeralDiskType string = "sigma.ali/ephemeral-disk-type"

	// ecs related labels
	// e.g. i-6grhpk0zd4gymw01te37
	LabelECSInstanceID string = "sigma.ali/ecs-instance-id"
	// e.g. deploymentSet-1
	LabelECSDeploymentSetID string = "sigma.ali/ecs-deployment-set-id"
	// e.g. ecs region id
	LabelECSRegionID string = "sigma.ali/ecs-region-id"
	// e.g. ecs env
	LabelECSENV string = "sigma.ali/ecs-env"

	// ratio of cpu over-commit, value should be equal or greater than 1.0
	LabelCPUOverQuota string = "sigma.ali/cpu-over-quota"
	// ratio of memory over-commit, value should be equal or greater than 1.0
	LabelMemOverQuota string = "sigma.ali/memory-over-quota"
	// ratio of disk space quota over-commit, value should be equal or greater than 1.0
	LabelDiskOverQuota string = "sigma.ali/disk-over-quota"
)

// LocalInfo contains the information collected by kubelet from node
type LocalInfo struct {
	// CPU basic information and status
	CPUBasicInfo CPUBasicInfo `json:"cpuBasicInfo,omitempty"`
	// CPU topology information of all available CPUs
	CPUInfos []CPUInfo `json:"cpuInfos,omitempty"`
	// Information for all local disk used for ephemeral storage
	DiskInfos []DiskInfo `json:"diskInfos,omitempty"`
}

// DiskType means storage medium of local disk for ephemeral storage
type DiskType string

const (
	// DiskTypeSSD refer to the solid state disk
	DiskTypeSSD DiskType = "ssd"
	// DiskTypeHDD refer to the hard disk driver
	DiskTypeHDD DiskType = "hdd"
)

// DiskInfo describe the local disk information
// Useful for local storage allocation
type DiskInfo struct {
	// Device file path in the filesystem, e.g. /dev/sda1
	Device string `json:"device,omitempty"`
	// Filesystem type, e.g. ext4
	FileSystemType string `json:"filesystemType,omitempty"`
	// Total capacity of the device in bytes
	Size int64 `json:"size,omitempty"`
	// The mount point in the filesystem, e.g. /home/docker
	MountPoint string `json:"mountPoint,omitempty"`
	// The type of storage medium
	DiskType DiskType `json:"diskType,omitempty"`
	// Whether the disk is the graph path of runtime, a.k.a docker root path
	IsGraphDisk bool `json:"isGraphDisk,omitempty"`
}

// CPUBasicInfo describes the cpu detail information and status
// Useful for cpu normalization to calculate ACU(Alibaba Compute Unit)
type CPUBasicInfo struct {
	CPUModel           string `json:"cpuModel,omitempty"`
	HyperThreadEnabled bool   `json:"hyperThreadEnabled,omitempty"`
	TurboEnabled       bool   `json:"turboEnabled,omitempty"`
}

// CPUTopoInfo describes the cpu topology information of a single logic cpu
// Useful for cpuset allocation, NUMA-aware scheduling
type CPUInfo struct {
	// logic CPU ID
	CPUID int32 `json:"cpu"`
	// physical CPU core ID
	CoreID int32 `json:"core"`
	// cpu socket ID
	SocketID int32 `json:"socket"`
	// numa node ID
	NodeID int32 `json:"node"`
}

// CPUSharePool describes the CPU IDs that the cpu share pool uses.
// The entire struct is json marshalled and saved in pod annotation.
// It is referenced in pod's annotation by key AnnotationNodeCPUSharePool.
type CPUSharePool struct {
	CPUIDs []int32 `json:"cpuIDs,omitempty"`
}

// Node taints
const (
	// TaintNodeLifecycle refer to the node's current state, e.g. offline
	TaintNodeLifecycle = "node.sigma.ali/lifecycle"
)
