package api

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerState is container state.
type ContainerState string

// container states are designed as same as those in Kubernetes code.
const (
	ContainerStateCreated   ContainerState = "created"
	ContainerStateRunning   ContainerState = "running"
	ContainerStateExited    ContainerState = "exited"
	ContainerStateUnknown   ContainerState = "unknown"
	ContainerStatePaused    ContainerState = "paused"
	ContainerStateSuspended ContainerState = "suspended"
)

// ContainerAction is the action performed on container
type ContainerAction string

const (
	// create represents just create the container not start it
	CreateContainerAction    ContainerAction = "create"
	StartContainerAction     ContainerAction = "start"
	StopContainerAction      ContainerAction = "stop"
	UpgradeContainerAction   ContainerAction = "upgrade"
	UpdateContainerAction    ContainerAction = "update"
	PauseContainerAction     ContainerAction = "pause"
	SuspendContainerAction   ContainerAction = "suspend"
	UnsuspendContainerAction ContainerAction = "unsuspend"
)

// InplaceUpdateState is inplace update state.
// https://yuque.antfin-inc.com/sys/sigma3.x/inplace-update-design-doc
const (
	InplaceUpdateStateCreated   string = "created"
	InplaceUpdateStateAccepted  string = "accepted"
	InplaceUpdateStateRejected  string = "rejected"
	InplaceUpdateStateFailed    string = "failed"
	InplaceUpdateStateSucceeded string = "succeeded"
)

// https://yuque.antfin-inc.com/sys/sigma3.x/yiu0yp
const (
	InplaceUpdateStateV2Created  string = "created"
	InplaceUpdateStateV2Rejected string = "rejected"
	InplaceUpdateStateV2Accepted string = "accepted"
)

// ContainerStateSpec is containers desired state.
type ContainerStateSpec struct {
	// It can contain all containers, but it not force. It should contain containers which you want operation.
	States map[ContainerInfo]ContainerState `json:"states"`
}

// ContainerStateStatus is containers status which is after kubelet action.
type ContainerStateStatus struct {
	Statuses map[ContainerInfo]ContainerStatus `json:"statuses"`
}

// ContainerInfo is the information about a container.
type ContainerInfo struct {
	// Name is the name of a container.
	Name string `json:"name"`
}

// ContainerStatus is the operation record about the container.
type ContainerStatus struct {
	// CreationTimestamp is the operation start time for the container.
	CreationTimestamp time.Time `json:"creationTimestamp,omitempty"`
	// FinishTimestamp is the operation stop time for the container.
	FinishTimestamp time.Time `json:"finishTimestamp,omitempty"`
	// RetryCount indicates Sigmalet retry times to finish the action.
	RetryCount int `json:"retryCount"`
	// CurrentState is container state which is after kubelet action.
	CurrentState ContainerState `json:"currentState,omitempty"`
	// LastState is container state which is before kubelet action.
	LastState ContainerState `json:"lastState,omitempty"`
	// Action is the current operation
	Action ContainerAction `json:"action,omitempty"`
	// Success is operation result.
	Success bool `json:"success"`
	// Message is a brief info about the operation result.
	Message string `json:"message,omitempty"`
	// SpecHash is the hash string of the container spec.
	SpecHash string `json:"specHash,omitempty"`
}

// MarshalText turns this instance into text.
func (c ContainerInfo) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%s", c.String())), nil
}

func (c ContainerInfo) String() string {
	// if containerInfo struct change, like add Id, you should format like this: fmt.Sprintf("%s://%s", c.Name, c.ID).
	return fmt.Sprintf("%s", c.Name)
}

// UnmarshalText turns text into the instance.
func (c *ContainerInfo) UnmarshalText(data []byte) error {
	return c.ParseString(string(data))
}

func (c *ContainerInfo) ParseString(data string) error {
	// Trim the quotes and split the name,
	parts := strings.Split(strings.Trim(data, "\""), "://")
	// if containerInfo struct change, you should change len to the num of contianerInfo struct element
	if len(parts) != 1 {
		return fmt.Errorf("invalid container name: %q", data)
	}
	// if containerInfo struct change, like add ID, you should parse like this: c.Name, c.ID = parts[0], parts[1]
	c.Name = parts[0]
	return nil
}

// RebuildContainerInfo container info from sigma 2.0 container.
type RebuildContainerInfo struct {
	// ContainerID container id.
	ContainerID string `json:"container_id,omitempty"`
	// DiskQuotaID disk quota id.
	DiskQuotaID string `json:"disk_quota_id,omitempty"`
	// AliAdminUID uid of admin in node.
	AliAdminUID string `json:"ali_admin_uid,omitempty"`
	// AnonymousVolumesMounts anonymous volume in sigma 2.0 container.
	AnonymousVolumesMounts []MountPoint `json:"anonymous_volumes_mounts,omitempty"`
}

// MountPoint volume mount point.
type MountPoint struct {
	// Name volume name.
	Name string `json:"name,omitempty"`
	// Source path of the mount on the host.
	Source string `json:"source,omitempty"`
	// Destination  path of the mount within the container.
	Destination string `json:"destination,omitempty"`
	// Driver volume driver.
	Driver string `json:"driver,omitempty"`
	// Mode of the tmpfs upon creation
	Mode string `json:"mode,omitempty"`
	//If set, the mount is read-only.
	RW bool `json:"rw,omitempty"`
}

// PostStartHook timeout key in ContainerExtraConfig
var PostStartHookTimeoutSeconds string = "PostStartHookTimeoutSeconds"

// ImagePull timeout key in ContainerExtraConfig
var ImagePullTimeoutSeconds string = "ImagePullTimeoutSeconds"

// PreStopHook timeout key in ContainerExtraConfig
var PreStopHookTimeoutSeconds string = "PreStopHookTimeoutSeconds"

// ContainerExtraConfig contains container's extra config such as timeout.
type ContainerExtraConfig struct {
	ContainerConfigs map[ContainerInfo]ContainerConfig `json:"containerConfigs"`
}

// ContainerConfig records customed config of container.
type ContainerConfig map[string]string

type K8sContainer struct {
	// Name is the name of a container.
	Name string `json:"name"`
	// Container resources
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ContainerResourceSpec struct {
	// request id
	RequestId          string         `json:"requestId"`
	OnlyResourceFilter bool           `json:"onlyResourceFilter"`
	Containers         []K8sContainer `json:"containers,omitempty"`
	// same as AllocSpec.Containers
	AllocSpecContainers []Container `json:"allocSpecContainers,omitempty"`
}

type ContainerResourceRequest struct {
	// contains creationTimestamp, labels
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ContainerResourceSpec `json:"spec"`
}
