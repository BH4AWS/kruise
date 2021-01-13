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
	"fmt"
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetFromCache(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	getter := NewImagePullAccountManager(fakeClient)

	fakeInfo := AuthInfo{
		Username: "hello",
		Password: "world",
	}
	realGetter := getter.(*imagePullAccountManager)
	registry := fmt.Sprintf("reg.%s", internalRegistrySuffix)
	realGetter.authInfoCache[registry] = authInfoCacheItem{deadline: time.Now().Add(time.Minute), authInfo: &fakeInfo}

	image := fmt.Sprintf("%s/me/img:latest", registry)
	if info, err := getter.GetAccountInfo(image); err != nil {
		t.Fatalf("expected get account for %s, but got %v", image, err)
	} else if !reflect.DeepEqual(info, &fakeInfo) {
		t.Fatalf("expected get account %v, but got %v", fakeInfo, info)
	}

	if info, err := getter.GetAccountInfo("test.net/me/img:latest"); err == nil && info != nil {
		t.Fatalf("expected get account error for test.net, but got %v err %v", info, err)
	}
}

func TestGetFromKubeSystem(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	manager := NewImagePullAccountManager(fakeClient)

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kube-system",
			Name:      "foo",
			Labels: map[string]string{
				"username": internalRegistryUser,
				"usage":    internalRegistryUsage,
			},
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte(internalRegistryUser),
			"password": []byte("whatever"),
		},
	}
	if _, err := fakeClient.CoreV1().Secrets("kube-system").Create(context.TODO(), &secret, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create fake secret: %v", err)
	}

	expectedInfo := &AuthInfo{
		Username: "aone",
		Password: "whatever",
	}
	image := fmt.Sprintf("reg.%s/%s/busybox", internalRegistrySuffix, internalRegistryUser)
	if err := validateGet(manager, image, expectedInfo); err != nil {
		t.Fatalf("%v", err)
	}

	// try to alter secret, it should still get the value in cache
	secret.Data["password"] = []byte("5678")
	_, _ = fakeClient.CoreV1().Secrets("kube-system").Update(context.TODO(), &secret, metav1.UpdateOptions{})

	if err := validateGet(manager, image, expectedInfo); err != nil {
		t.Fatalf("%v", err)
	}
}

func validateGet(manager ImagePullAccountManager, image string, expectedInfo *AuthInfo) error {
	info, err := manager.GetAccountInfo(image)
	if expectedInfo == nil {
		if err == nil {
			return fmt.Errorf("expected error, but got %v", info)
		}
		return nil
	}

	if !reflect.DeepEqual(expectedInfo, info) {
		return fmt.Errorf("expected info %v, but got %v, err: %v", expectedInfo, info, err)
	}
	return nil
}
