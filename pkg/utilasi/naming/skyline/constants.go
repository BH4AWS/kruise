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
	"strings"
	"time"

	"github.com/openkruise/kruise/pkg/utilasi/naming"
)

const (
	addTagUri      = "%s/h/server/tag_add"             // 打标
	delTagUri      = "%s/h/server/tag_del"             // 去标
	batchAddUri    = "%s/item/batch_add"               // 批量新增
	vmAddUri       = "%s/openapi/device/vm/add"        // 新增
	commonAddUri   = "%s/v2/item/add"                  // 新增
	vmDeleteUri    = "%s/openapi/device/vm/delete"     // 删除
	queryUri       = "%s/item/query"                   // 查询
	updateUri      = "%s/openapi/device/server/update" // 更新
	groupAddUri    = "%s/app_group/insert"             // 添加分组
	groupUpdateUri = "%s/app_group/update"             // 更新分组
	timeOutDefault = time.Duration(60) * time.Second   // 限流器锁超时
	resourceOwner  = "sigma"
	uniformTypePod = "pod"
)

const (
	CategoryNamePodContainer = "pod_container"
)

var SelectDefault = strings.Join([]string{
	SelectCabinetNum,
	SelectSite,
	SelectCabinetPosition,
	SelectAppName,
	SelectAppGroup,
	SelectAppUseType,
	SelectCpuCount,
	SelectDiskSize,
	SelectMemorySize,
	SelectAbb,
	SelectSmName,
	SelectCreate,
	SelectResourceOwner,
	SelectModify,
	SelectParentSn,
	SelectSn,
	SelectIp,
	SelectHostName,
	SelectAppServerState,
	SelectSecurityDomain,
	SelectContainerId,
	SelectMode,
	SelectParentHostName,
	SelectK8sPodName,
	SelectK8sClusterName,
	SelectK8sNamespace,
	SelectAllocateMode}, ",")

// 参考https://yuque.antfin-inc.com/at7ocb/qbn0oy/dwuaod#9c1zin
const (
	SelectCabinetNum      = "cabinet.cabinet_num"    // 机柜编号
	SelectCabinetPosition = "cabinet_position"       // 设备在机柜中位置号
	SelectSite            = "cabinet.logic_region"   // 逻辑机房
	SelectAppName         = "app.name"               // 应用名称
	SelectAppGroup        = "app_group.name"         // 应用分组
	SelectAppUseType      = "app_use_type"           // 应用用途
	SelectCpuCount        = "total_cpu_count"        // 总核数
	SelectDiskSize        = "total_disk_size"        // 磁盘大小
	SelectMemorySize      = "total_memory_size"      // 内存大小
	SelectAbb             = "room.abbreviation"      // 物理房间缩写
	SelectSmName          = "standard_model.sm_name" // 标准机型名称
	SelectCreate          = "gmt_create"             // 创建时间
	SelectResourceOwner   = "resource_owner"         // 数据拥有者
	SelectModify          = "gmt_modified"           // 上次修改时间
	SelectParentSn        = "parent_service_tag"     // 父sn,物理机为机框sn,虚拟机为服务器sn
	SelectParentHostName  = "parent_host_name"       // 父设备hostname
	SelectSn              = "sn"                     // 容器或者宿主机sn
	SelectIp              = "ip"                     // 容器或者宿主机ip
	SelectHostName        = "host_name"              // hostname
	SelectAppServerState  = "app_server_state"       // 应用状态
	SelectSecurityDomain  = "security_domain"        // 安全域
	SelectContainerId     = "container_id"           // 容器id
	SelectAllocateMode    = "allocate_mode"          // 分配模式
	SelectDiskQuota       = "disk_quota"             // 磁盘信息
	SelectID              = "id"                     // id
	SelectSdnType         = "sdn_type"               // 容器SDN网络类型
	SelectEniInstanceId   = "eni_instance_id"        // 弹性网卡ID
	SelectMode            = "device_model.model_name"
	SelectK8sPodName      = "pod_name"
	SelectK8sNamespace    = "pod_namespace"
	SelectK8sClusterName  = "pod_cluster_name"
)

// 参考 https://yuque.antfin-inc.com/at7ocb/qbn0oy/qbdtlf#serverapiparamconstantserver
const (
	ApiParamSn       = "sn"       // sn
	ApiParamParentSn = "parentSn" // 父类sn
	ApiParamAppGroup = "appGroup" // 应用分组
	ApiParamHostName = "hostName" // 主机名

	ApiParamDeviceType       = "device_type"      // 设备类型
	ApiParamIp               = "ip"               // ip
	ApiParamIpSecurityDomain = "ipSecurityDomain" // 安全域
	ApiParamAppServerState   = "appServerState"   // 服务状态
	ApiParamTotalCpuCount    = "totalCpuCount"    // 总核数
	ApiParamTotalMemorySize  = "totalMemorySize"  // 内存大小单位M
	ApiParamTotalDiskSize    = "totalDiskSize"    // 磁盘大小单位G
	ApiParamAppUseType       = "appUseType"       // 应用状态
	ApiParamResourceOwner    = "resourceOwner"    // 资源归属
	ApiParamContainerState   = "containerState"   // 容器状态
	ApiParamContainerId      = "containerId"      // 容器ID

	ApiParamAllocateMode    = "allocateMode"   // 容器分配模式
	ApiParamDiskQuota       = "diskQuota"      // 磁盘信息
	ApiParamSdnType         = "sdn_type"       // 容器网络类型
	ApiParamSdnTunnelId     = "sdn_tunnel_id"  // 容器TunnelId
	ApiParamSdnGatewayIp    = "sdn_gateway_ip" // 容器vgw网关ip
	ApiParamEniInstanceId   = "eniInstanceId"  // server专用字段使用驼峰格式, by @炫迈
	ApiParamRegionId        = "region"
	ApiParamZoneId          = "zone"
	ApiParamVpcId           = "vpc_id"
	ApiParamVSwitchId       = "ecs_v_switch_id"
	ApiParamSecurityGroupId = "security_groups"

	ApiParamK8sPodName     = "podName"        //K8S POD名
	ApiParamK8sNamespace   = "podNamespace"   //K8S命名空间
	ApiParamK8sClusterName = "podClusterName" //K8S集群名

	ApiParamDeviceModel = "deviceModel" // 设备模式
)

// 业务错误码, 参考：https://yuque.antfin-inc.com/at7ocb/qbn0oy/wnb6wv
const (
	UK_VALUE_ALREADY_EXISTS = 30101001
)

type auth struct {
	Account   string `json:"account"`
	AppName   string `json:"appName"`
	Signature string `json:"signature"`
	Timestamp int64  `json:"timestamp"`
}

// TagAdd or Delete
type TagParam struct {
	Auth        *auth        `json:"auth"`
	SkyOperator *skyOperator `json:"operator"`
	Sn          string       `json:"sn"`
	Tag         string       `json:"tag"`
	TagValue    string       `json:"tagValue"`
}

type skyOperator struct {
	Type     interface{} `json:"type"`
	Nick     string      `json:"nick"`
	WorkerId string      `json:"workerId"`
}

// addRecordForPod
type addRecordForPodParam struct {
	Auth        *auth                    `json:"auth"`
	SkyOperator *skyOperator             `json:"operator"`
	Item        []map[string]interface{} `json:"item"`
}

// vmAdd
type VmAddParam struct {
	Auth        *auth        `json:"auth"`
	SkyOperator *skyOperator `json:"operator"`
	SkyItem     *skyVmItem   `json:"item"`
}

// AppGroup
type AppGroupParam struct {
	Auth        *auth                `json:"auth"`
	SkyOperator *skyOperator         `json:"operator"`
	Item        *naming.AppGroupInfo `json:"item"`
}

type CommonAddParam struct {
	Auth        *auth          `json:"auth"`
	SkyOperator *skyOperator   `json:"operator"`
	Item        *skyCommonItem `json:"item"`
}

type skyCommonItem struct {
	CategoryName string                 `json:"categoryName"`
	PropertyMap  map[string]interface{} `json:"propertyMap"`
}

type skyVmItem struct {
	DeviceType  string                 `json:"deviceType"`
	PropertyMap map[string]interface{} `json:"propertyMap"`
}

// query
type QueryParam struct {
	Auth  *auth            `json:"auth"`
	Query *CommonQueryInfo `json:"item"`
}

type CommonQueryInfo struct {
	From      string `json:"from"`      // 查询那个表;底层自动根据选择的字段进行join
	Select    string `json:"select"`    // 查询哪些columns;目标表可以直接取属性名,其他关联表必须以类目名作为前缀.分隔 "sn,ip,node_type,app_group.name"
	Condition string `json:"condition"` // 查询条件;暂时只支持AND;左表达式为字段名;表达式: =, !=, >, >=, <, <=, IN, LIKE;右表达式为值字符串和数字,字符串用''包起来,其他像boolean都当成字符类型
	Page      int    `json:"page"`      // 页码;第几页;从1开始
	Num       int    `json:"num"`       // 每页大小
	NeedTotal bool   `json:"needTotal"` // 是否需要总数;为true才承诺给准确总数;不需要总数默认false;有利于底层优化
}

// 所有api返回的通用结构
type Result struct {
	Success      bool        `json:"success"`
	Value        interface{} `json:"value"`
	ErrorCode    int         `json:"errorCode"`
	ErrorMessage string      `json:"errorMessage"`
}

// 标签操作的数据结构
type AddTagResult struct {
	Success      bool   `json:"success"`
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

type CmdbPodQueryResult struct {
	TotalCount int                   `json:"totalCount"`
	HasMore    bool                  `json:"hasMore"`
	ItemList   []*naming.CmdbPodInfo `json:"itemList"`
}

type CmdbCommonAddResult []int64
type CmdbAppGroupResult int64

// Update
type UpdateParam struct {
	Auth        *auth        `json:"auth"`
	SkyOperator *skyOperator `json:"operator"`
	UpdateItem  *UpdateItem  `json:"item"`
}

type UpdateItem struct {
	ContainerIP string                 `json:"-"`
	Sn          string                 `json:"sn"`
	PropertyMap map[string]interface{} `json:"propertyMap"`
}

const (
	updateNodeOperatorType = 1        // update时的Operator类型
	defaultOperatorType    = "SYSTEM" // 通用的Operator类型
)

type VmDeleteParam struct {
	Auth        *auth        `json:"auth"`
	SkyOperator *skyOperator `json:"operator"`
	SkyItem     string       `json:"item"`
}

type DiskInfo struct {
	HostPath   string `json:"host_path"`
	Type       int    `json:"type"` // 0: Volume使用本地盘(默认值); 1:Volume使用远程盘
	Name       string `json:"name"`
	DiskSize   int64  `json:"disk_size"`
	MountPoint string `json:"mount_point"`
}
