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
	v1 "k8s.io/api/core/v1"
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

func RemoveContainerEnvVar(container *v1.Container, envName string) {
	if container == nil {
		return
	}
	newEnvs := make([]v1.EnvVar, 0, len(container.Env))
	for i := range container.Env {
		if container.Env[i].Name == envName {
			continue
		}
		newEnvs = append(newEnvs, container.Env[i])
	}
	container.Env = newEnvs
}

func GetContainerEnvVar(container *v1.Container, key string) *v1.EnvVar {
	if container == nil {
		return nil
	}
	for i, e := range container.Env {
		if e.Name == key {
			return &container.Env[i]
		}
	}
	return nil
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

func ReplaceOwnerRef(obj metav1.Object, owner metav1.OwnerReference) {
	refs := []metav1.OwnerReference{owner}
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			continue
		}
		refs = append(refs, ref)
	}
	obj.SetOwnerReferences(refs)
}

// 合并origin.Envs 和 other.Envs，并且将结果赋值到 origin.Envs
// 如果origin.Envs 与 other.Envs 包含相同的Env.Name，则以origin为准
//
//	例如：origin.Envs = []EnvVar{
//			{
//				Name: env-1,
//				Value: origin1,
//			},
//			{
//				Name: env-2,
//				Value: origin2,
//			}
//		}
//	     other.Envs = []EnvVar{
//			{
//				Name: env-2,
//				Value: other2,
//			},
//			{
//				Name: env-3,
//				Value: other3,
//			}
//		}
//
// 最终输出结果：
//
//	     origin.Envs = []EnvVar{
//			{
//				Name: env-1,
//				Value: origin1,
//			},
//			{
//				Name: env-2,
//				Value: origin2,
//			},
//			{
//				Name: env-3,
//				Value: other3,
//			}
//		}
func MergeEnvsInContainer(origin *v1.Container, other v1.Container) {
	envExist := make(map[string]bool)
	for _, env := range origin.Env {
		envExist[env.Name] = true
	}
	for _, env := range other.Env {
		if envExist[env.Name] {
			continue
		}
		origin.Env = append(origin.Env, env)
	}

	envFromExist := make(map[string]bool)
	for _, envFrom := range origin.EnvFrom {
		if envFrom.ConfigMapRef != nil {
			envFromExist[envFrom.ConfigMapRef.Name] = true
		} else if envFrom.SecretRef != nil {
			envFromExist[envFrom.SecretRef.Name] = true
		}
	}
	for _, envFrom := range other.EnvFrom {
		var envName string
		if envFrom.ConfigMapRef != nil {
			envName = envFrom.ConfigMapRef.Name
		} else if envFrom.SecretRef != nil {
			envName = envFrom.SecretRef.Name
		}
		if envFromExist[envName] {
			continue
		}
		origin.EnvFrom = append(origin.EnvFrom, envFrom)
	}
}
