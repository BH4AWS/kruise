/*
Copyright 2021 The Kruise Authors.

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

const (
	LabelNodeType          = "alibabacloud.com/node-type"
	NodeTypeVirtualKubelet = "virtual-kubelet"

	LabelScheduleNodeName = "scheduler.assign/nodename"

	AnnotationUpdateSpec   = "alibabacloud.com/update-spec"
	AnnotationUpdateStatus = "alibabacloud.com/update-status"
)

// ContainerUpdateSpec is containers status which is after kubelet action.
type ContainerUpdateSpec struct {
	Version    string             `json:"version"`
	Containers []ContainerVersion `json:"statuses,omitempty"`
}

// ContainerUpdateStatus is containers status which is after kubelet action.
type ContainerUpdateStatus struct {
	Version    string             `json:"version"`
	Containers []ContainerVersion `json:"containers"`
}

// ContainerVersion is the operation record about the container.
type ContainerVersion struct {
	Name string `json:"name,omitempty"`
	// SpecHash is the hash string of the container spec.
	Version string `json:"version,omitempty"`
}
