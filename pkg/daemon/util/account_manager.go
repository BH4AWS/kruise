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

package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

type AuthInfo struct {
	Username string
	Password string
}

func (i *AuthInfo) EncodeToString() string {
	authConfig := dockertypes.AuthConfig{
		Username: i.Username,
		Password: i.Password,
	}
	encodedJSON, _ := json.Marshal(authConfig)
	return base64.URLEncoding.EncodeToString(encodedJSON)
}

type ImagePullAccountManager interface {
	GetAccountInfo(repo string) (*AuthInfo, error)
}

// NewImagePullAccountManager returns an ImagePullAccountManager, defaults to be nil
func NewImagePullAccountManager(kubeClient clientset.Interface) ImagePullAccountManager {
	return &imagePullAccountManager{
		kubeClient:    kubeClient,
		authInfoCache: make(map[string]authInfoCacheItem),
	}
}

const (
	internalRegistrySuffix = "docker.alibaba-inc.com"
	internalRegistryUser   = "aone"
	internalRegistryUsage  = "ali-registry-user-account"
)

type authInfoCacheItem struct {
	deadline time.Time
	authInfo *AuthInfo
}

func (c authInfoCacheItem) isExpired() bool {
	return time.Now().After(c.deadline)
}

type imagePullAccountManager struct {
	sync.Mutex
	kubeClient    clientset.Interface
	authInfoCache map[string]authInfoCacheItem
}

func (i *imagePullAccountManager) GetAccountInfo(repo string) (*AuthInfo, error) {
	registry := ParseRegistry(repo)
	isInternalDomain := strings.HasSuffix(registry, internalRegistrySuffix)
	if !isInternalDomain {
		return nil, nil
	}

	i.Lock()
	defer i.Unlock()
	if cache, ok := i.authInfoCache[registry]; ok {
		if !cache.isExpired() {
			return cache.authInfo, nil
		}
	}

	// 1. 从kube-system下找到aone账号（兼容alipodlifecyclehook逻辑）
	if info, err := i.getAuthFromKubeSystem(); err != nil {
		klog.Warningf("Failed to get secret in kube-system: %v", err)
	} else if info != nil {
		// renew cache in 5~10 minutes
		interval := time.Duration(rand.Int31n(6)+5) * time.Minute
		item := authInfoCacheItem{
			deadline: time.Now().Add(interval),
			authInfo: info,
		}
		i.authInfoCache[registry] = item
		return info, nil
	}

	return nil, fmt.Errorf("not found auth info for %v", repo)
}

func (i *imagePullAccountManager) getAuthFromKubeSystem() (*AuthInfo, error) {
	opts := metav1.ListOptions{
		LabelSelector:   fmt.Sprintf("usage=%s,username=%s", internalRegistryUsage, internalRegistryUser),
		ResourceVersion: "0",
	}
	secretList, err := i.kubeClient.CoreV1().Secrets("kube-system").List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	for _, s := range secretList.Items {
		if s.Type == v1.SecretTypeOpaque {
			return &AuthInfo{
				Username: string(s.Data["username"]),
				Password: string(s.Data["password"]),
			}, nil
		}
	}
	return nil, fmt.Errorf("not found aone registry secrets")
}
