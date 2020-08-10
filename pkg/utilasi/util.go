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

package utilasi

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPodNames(pods []*v1.Pod) []string {
	var ret []string
	for _, p := range pods {
		ret = append(ret, p.Name)
	}
	return ret
}

func RemoveSpecifiedEnvFromContainer(container map[string]interface{}, envName string) {
	if envsObj, ok := container["env"]; ok {
		var newEnvs []interface{}
		envs := envsObj.([]interface{})
		for j := range envs {
			if envs[j].(map[string]interface{})["name"] == envName {
				continue
			}
			newEnvs = append(newEnvs, envs[j])
		}
		container["env"] = newEnvs
	}
}

func AddContainerEnvWithOverwrite(container *v1.Container, key, value string) {
	if container == nil {
		return
	}
	for i, e := range container.Env {
		if e.Name == key {
			container.Env[i].Value = value
			container.Env[i].ValueFrom = nil
			return
		}
	}
	container.Env = append(container.Env, v1.EnvVar{
		Name:  key,
		Value: value,
	})
}

func AddContainerEnvHeadWithOverwrite(container *v1.Container, key, value string) {
	if container == nil {
		return
	}
	if len(container.Env) == 0 {
		container.Env = []v1.EnvVar{{Name: key, Value: value}}
		return
	}
	for i, e := range container.Env {
		if e.Name == key {
			container.Env[i].Value = value
			container.Env[i].ValueFrom = nil
			return
		}
	}
	container.Env = append([]v1.EnvVar{{Name: key, Value: value}}, container.Env...)
}

func AddContainerEnvNoOverwrite(container *v1.Container, key, value string) {
	if container == nil {
		return
	}
	for _, e := range container.Env {
		if e.Name == key {
			return
		}
	}
	container.Env = append(container.Env, v1.EnvVar{
		Name:  key,
		Value: value,
	})
}

func ReplaceOwnerRef(pod *v1.Pod, owner metav1.OwnerReference) {
	refs := []metav1.OwnerReference{owner}
	for _, ref := range pod.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			continue
		}
		refs = append(refs, ref)
	}
	pod.OwnerReferences = refs
}
