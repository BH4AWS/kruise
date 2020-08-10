/*
Copyright 2019 The Kruise Authors.

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

package runtime

import (
	"fmt"

	"github.com/openkruise/kruise/pkg/daemonset/imagepuller/util"
)

const (
	// different container runtimes
	DockerContainerRuntime = "docker"
	PouchContainerRuntime  = "pouch"
	ContainerdCRIRuntime   = "containerd"
)

// NewImageRuntime create image runtime
func NewImageRuntime(runtimeType string, uri string, accountManager util.ImagePullAccountManager) (ImageRuntime, error) {
	switch runtimeType {
	case DockerContainerRuntime:
		return NewDockerImageRuntime(uri, accountManager)
	case PouchContainerRuntime:
		return NewPouchImageRuntime(uri, accountManager)
	case ContainerdCRIRuntime:
		return NewContainerdImageRuntime(uri, accountManager)
	default:
		return nil, fmt.Errorf("Unsupported runtime type %v", runtimeType)
	}
}
