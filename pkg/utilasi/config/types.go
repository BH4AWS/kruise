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
	"fmt"

	clonesetapi "github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	"github.com/openkruise/kruise/pkg/utilasi/naming"
	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	"k8s.io/kubernetes/pkg/util/slice"
)

type ConfigType string

const (
	InPlaceSetConfigType   ConfigType = "inPlaceSetConfig"
	AliThirdPartConfigType ConfigType = "aliThirdPartConfig"
	PodMonitorConfigType   ConfigType = "podMonitorConfig"

	PodMonitorRestartCntDefault int = 10
	PodMonitorWindowSizeDefault int = 3 //分钟
)

type configInterface interface {
	setDefaultConfig()
	setMutatingConfig()
}

type InPlaceSetConfig struct {
	// sync过程中发生了扩容或重建，queue.AddAfter的duration时间
	QueueAddAfterCreationInSeconds int `json:"queueAddAfterCreationInSeconds"`

	// updateExpectations中还有pod，queue.AddAfter的duration时间
	QueueAfterHasUpdateExpectationsInMinutes int `json:"queueAfterHasUpdateExpectationsInMinutes"`

	// 是否开启针对阿里集团的在扩容发布场景下做一些事情
	EnableAliProcHook bool `json:"enableAliProcHook"`

	// 重建相关配置
	RecreateConfig InPlaceSetRecreateConfig `json:"recreateConfig"`

	// 发布过程中要保留的Env keys
	EnvKeysInherited []string `json:"envKeysInherited"`

	// 默认注入的upgrade scatter
	DefaultUpgradeScatter []string `json:"defaultUpgradeScatter"`
}

func (c *InPlaceSetConfig) setDefaultConfig() {
}

func (c *InPlaceSetConfig) setMutatingConfig() {
	setIntValueIfLessThan(&c.QueueAfterHasUpdateExpectationsInMinutes, 1, 5)
	setIntValueIfLessThan(&c.QueueAddAfterCreationInSeconds, 5, 30)
	setIntValueIfLessThan(&c.RecreateConfig.OnceRecreatePodsMax, 1, 5)
	setIntValueIfLessThan(&c.RecreateConfig.ExcludeNodesKeepInSeconds, 1, 3600)
	setIntValueIfLessThan(&c.RecreateConfig.PolicyNoScheduledInSeconds, 1, 300)
	//setIntValueIfLessThan(&c.RecreateConfig.PolicyFailedScheduleIntervalInSeconds, 3, 60)
	setIntValueIfLessThan(&c.RecreateConfig.PolicyNoCreateAfterScheduledInSeconds, 1, 300)

	// 一些默认要加的key
	if !slice.ContainsString(c.EnvKeysInherited, clonesetapi.EnvDebugRestartTimestamp, nil) {
		c.EnvKeysInherited = append(c.EnvKeysInherited, clonesetapi.EnvDebugRestartTimestamp)
	}
	if !slice.ContainsString(c.EnvKeysInherited, clonesetapi.EnvAliRunInit, nil) {
		c.EnvKeysInherited = append(c.EnvKeysInherited, clonesetapi.EnvAliRunInit)
	}

	if len(c.DefaultUpgradeScatter) == 0 {
		c.DefaultUpgradeScatter = []string{fmt.Sprintf("%s=%s", sigmak8sapi.LabelPodRegisterNamingState, naming.Armory_WORKING_ONLINE)}
	}
}

func setIntValueIfLessThan(v *int, minimunValue int, defaultValue int) {
	if v == nil {
		return
	}
	if *v < minimunValue {
		*v = defaultValue
	}
}

type InPlaceSetRecreateConfig struct {
	// 扩容重试开关
	Open bool `json:"open"`
	// 一次sync中最多重建的pod数量
	OnceRecreatePodsMax int `json:"onceRecreatePodsMax"`
	// 扩容重试最多排除node数量
	ExcludeNodesMax int `json:"excludeNodesMax"`
	// 扩容重试排除node保留时间
	ExcludeNodesKeepInSeconds int `json:"excludeNodesKeepInSeconds"`

	// 扩容重试策略：调度超时时间
	PolicyNoScheduledInSeconds int `json:"policyNoScheduledInSeconds"`
	// 扩容重试策略：调度失败重试间隔
	//PolicyFailedScheduleIntervalInSeconds int `json:"policyFailedScheduleIntervalInSeconds"`
	// 扩容重试策略：sigmalet接管超时时间
	PolicyNoCreateAfterScheduledInSeconds int `json:"policyNoCreateAfterScheduledInSeconds"`
}

type AliThirdPartConfig struct {
	ArmoryConfig          ArmoryConfig          `json:"armoryConfig"`
	DauthConfig           DauthConfig           `json:"dauthConfig"`
	SkyConfig             SkyConfig             `json:"skyConfig"`
	NormandyServiceConfig NormandyServiceConfig `json:"normandyServiceConfig"`
	AlimonitorConfig      AlimonitorConfig      `json:"alimonitorConfig"`
	VipserverConfig       []VipserverMeta       `json:"vipserverConfig"`
	VipConfig             VipConfig             `json:"vipConfig"`
	KeyCenterConfig       KeyCenterConfig       `json:"keyCenterConfig"`
	AccountConfig         AccountConfig         `json:"accountConfig"`
	AtomConfig            AtomConfig            `json:"atomConfig"`
	AliyunPvtzConfig      AliyunPvtzConfig      `json:"aliyunPvtzConfig"`
	GrayPlatformConfig    GrayPlatformConfig    `json:"grayPlatformConfig"`
	DNSConfig             DNSConfig             `json:"dnsConfig"`
	HsfConfig             HsfConfig             `json:"hsfConfig"`
}

func (c *AliThirdPartConfig) setDefaultConfig() {
}

func (c *AliThirdPartConfig) setMutatingConfig() {
	if c.DauthConfig.RefreshDays <= 0 {
		c.DauthConfig.RefreshDays = 7
	}
}

type ArmoryConfig struct {
	Url         string `json:"url"`
	User        string `json:"user"`
	Key         string `json:"key"`
	Concurrency int    `json:"concurrency"`
	Disabled    bool   `json:"disabled"`
}

type DauthConfig struct {
	ServerHost  string `json:"serverHost"`
	AccessKey   string `json:"accessKey"`
	SecretKey   string `json:"secretKey"`
	Disabled    bool   `json:"disabled"`
	RefreshDays int    `json:"refreshDays"`
}

type SkyConfig struct {
	App         string `json:"app"`
	Url         string `json:"url"`
	SecondUrl   string `json:"secondUrl"`
	User        string `json:"user"`
	Key         string `json:"key"`
	Concurrency int    `json:"concurrency"`
}

type DNSConfig struct {
	App           string `json:"app"`
	Url           string `json:"url"`
	Key           string `json:"key"`
	Concurrency   int    `json:"concurrency"`
	Submitter     string `json:"submitter"`
	DomainAppName string `json:"domainAppName"`
}

type NormandyServiceConfig struct {
	Url  string `json:"url"`
	User string `json:"user"`
	Key  string `json:"key"`
}

type AlimonitorConfig struct {
	Url        string `json:"url"`
	UserName   string `json:"userName"`
	PrivateKey string `json:"privateKey"`
}

type VipserverMeta struct {
	AppStage  string `json:"appStage"`
	AppUnit   string `json:"appUnit"`
	EnvName   string `json:"envName"`
	UrlPrefix string `json:"urlPrefix"`
}

type VipConfig struct {
	AppNum  string `json:"appNum"`
	KeyName string `json:"keyName"`
	Url     string `json:"url"`
}

type KeyCenterConfig struct {
	Url string `json:"url"`
}

type PodMonitorConfig struct {
	Disable    bool `json:"disabled"`
	Window     int  `json:"window"`
	RestartCnt int  `json:"restartCnt"`
}

// 默认策略，PodMonitorWindowSizeDefault分钟内，liveness触发的，重启次数超过PodMonitorRestartCntDefault次，执行pod迁移
func (c *PodMonitorConfig) setDefaultConfig() {
	c.RestartCnt = PodMonitorRestartCntDefault
	c.Window = PodMonitorWindowSizeDefault
	c.Disable = true
}

func (c *PodMonitorConfig) setMutatingConfig() {

}

type AccountConfig struct {
	Url       string `json:"url"`
	SecretId  string `json:"secretId"`
	SecretKey string `json:"secretKey"`
}

type AtomConfig struct {
	Url       string `json:"url"`
	SecretId  string `json:"aceessID"`
	SecretKey string `json:"aceessKey"`
}

type AliyunPvtzConfig struct {
	RegionId        string `json:"regionId"`
	AccessKeyId     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	ClusterDomain   string `json:"clusterDomain,omitempty"`
}

type GrayPlatformConfig struct {
	Url      string `json:"url"`
	SysName  string `json:"sysName"`
	SignName string `json:"signName"`
}

type HsfConfig struct {
	Url string `json:"url"`
}
