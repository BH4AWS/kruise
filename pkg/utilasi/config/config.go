/*
Copyright 2020 The Kruise Authors.

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

package config

import (
	"context"
	"encoding/json"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	configNamespace = "kube-system"
	configName      = "ak8s-ee-extensions-controller-config"
)

type ConfigMgr interface {
	GetConfigFromConfigMap(configType ConfigType, customConfig configInterface) (bool, error)
}

type configMgr struct {
	reader client.Reader
}

func NewConfigMgr(r client.Reader) ConfigMgr {
	return &configMgr{reader: r}
}

func (c *configMgr) GetConfigFromConfigMap(configType ConfigType, customConfig configInterface) (bool, error) {
	// set default and mutating values for config
	customConfig.setDefaultConfig()
	defer customConfig.setMutatingConfig()

	// Get configmap from k8s
	cm := &v1.ConfigMap{}
	err := c.reader.Get(context.TODO(), types.NamespacedName{Name: configName, Namespace: configNamespace}, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	val, ok := cm.Data[string(configType)]
	if !ok {
		return false, nil
	}

	// unmarshal config json
	if err := json.Unmarshal([]byte(val), customConfig); err != nil {
		return false, err
	}

	return true, nil
}
