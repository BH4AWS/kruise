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

package skyline

import (
	"github.com/openkruise/kruise/pkg/utilasi/config"
	"github.com/openkruise/kruise/pkg/utilasi/naming"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func QueryNodeGroupByName(reader client.Reader, groupName string) (*naming.AppGroupItem, error) {
	service, err := GetService(config.NewConfigMgr(reader))
	if err != nil {
		return nil, err
	}
	return service.QueryAppGroup(groupName)
}
