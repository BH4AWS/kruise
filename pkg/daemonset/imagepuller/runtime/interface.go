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
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
)

var (
	defaultImagePullingProgressLogInterval = 5 * time.Second
)

type ImageInfo struct {
	// ID of an image.
	ID string `json:"Id,omitempty"`
	// repository with digest.
	RepoDigests []string `json:"RepoDigests"`
	// repository with tag.
	RepoTags []string `json:"RepoTags"`
	// size of image's taking disk space.
	Size int64 `json:"Size,omitempty"`
}

type ImagePullStatus struct {
	Err        error
	Process    int
	DetailInfo string
	Finish     bool
}

type ImagePullStatusReader interface {
	C() <-chan ImagePullStatus
	Close()
}

type ImageRuntime interface {
	PullImage(ctx context.Context, imageName, tag string, pullSecrets []v1.Secret) (ImagePullStatusReader, error)
	ListImages(ctx context.Context) ([]ImageInfo, error)
}