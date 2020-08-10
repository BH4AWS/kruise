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
	"fmt"
	"sync"

	"github.com/openkruise/kruise/pkg/utilasi/config"
	"github.com/openkruise/kruise/pkg/utilasi/naming"
	"k8s.io/klog"
)

var service naming.Service
var lock sync.RWMutex

func SetService(input naming.Service) {
	service = input
}

func GetService(configMgr config.ConfigMgr) (naming.Service, error) {
	if service != nil {
		return service, nil
	}

	lock.Lock()
	defer lock.Unlock()
	if service != nil {
		return service, nil
	}
	newService, err := NewSkylineNamingService(configMgr)
	if err != nil {
		return nil, err
	}
	service = newService
	return service, nil
}

type SkylineNamingService struct {
	SkyManager *skylineManager // 新的skyline接口
}

// 构建一个skyline的NamingService
func NewSkylineNamingService(configMgr config.ConfigMgr) (*SkylineNamingService, error) {
	getSkyeConfig := func() (*config.SkyConfig, error) {
		c := &config.AliThirdPartConfig{}
		got, err := configMgr.GetConfigFromConfigMap(config.AliThirdPartConfigType, c)
		if err != nil {
			return nil, fmt.Errorf("failed get armory config: %v", err)
		} else if !got {
			return nil, fmt.Errorf("not found armory config")
		}

		if c.SkyConfig.Concurrency == 0 {
			c.SkyConfig.Concurrency = 50
		}
		return &c.SkyConfig, nil
	}

	skyConfig, err := getSkyeConfig()
	if err != nil {
		return nil, err
	}

	getSkyeConfigParam := func() *config.SkyConfig {
		newConf, err := getSkyeConfig()
		if err != nil {
			klog.Infof("getSkyeConfig error:%v", err)
			return skyConfig
		}
		return newConf
	}

	return &SkylineNamingService{
		SkyManager: newSkylineManager(getSkyeConfigParam),
	}, nil
}

// skyline的文档地址 https://horizon.alibaba-inc.com/meta/cp_list.htm?cat_id=11
func (skyline *SkylineNamingService) QueryAppGroup(groupName string) (*naming.AppGroupItem, error) {
	queryItem := &CommonQueryInfo{
		From:      "app_group",
		Select:    "name, owner_string, app.name, safety_out, app_id",
		Condition: fmt.Sprintf("name='%v'", groupName),
		Page:      1,
		Num:       1000,
	}
	groupInfo := &naming.AppGroupsInfo{}
	_, err := skyline.SkyManager.query(queryItem, groupInfo)
	if err != nil {
		return nil, err
	}
	if len(groupInfo.ItemList) > 1 {
		return nil, fmt.Errorf("[SkylineNamingService] find skyline app_group by name have more than one records, name:%v", groupName)
	}
	if len(groupInfo.ItemList) == 0 {
		return nil, nil
	}
	return groupInfo.ItemList[0], nil
}
