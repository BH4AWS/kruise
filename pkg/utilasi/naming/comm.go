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

package naming

const (
	SkylineCpuSetModeNameSpace = "cpu_set_mode"
	SkylineCpuSetModeCpuSet    = "cpu_set_mode.cpuset"
	SkylineCpuSetModeCpuShare  = "cpu_set_mode.cpushare"
)

const (
	EQ = "EQ"
	IN = "IN"
)

const (
	Skyline_CpuSetMode_NameSpace     = "cpu_set_mode"
	Skyline_CpuSetMode_ROLE_CpuSet   = "cpu_set_mode.cpuset"
	Skyline_CpuSetMode_ROLE_CpuShare = "cpu_set_mode.cpushare"
)

var (
	ArmoryRegisterStateMap = map[ArmoryAppState]int{
		Armory_READY:           1,
		Armory_WAIT_ONLINE:     1,
		Armory_WORKING_ONLINE:  1,
		Armory_WORKING_OFFLINE: 1,
		Armory_BUFFER:          1,
		Armory_UNKNOWN:         1,
		Armory_BROKEN:          1,
		Armory_LOCK:            1,
		Armory_UNUSE:           1,
	}
)

type ArmoryAppState string

const (
	NsUnknownStatus  = "unknown"
	NsCreatedStatus  = "allocated"
	NsPreparedStatus = "prepared"
	NsRunningStatus  = "running"
	NsStoppedStatus  = "stopped"
)

var (
	Armory_UNKNOWN         = ArmoryAppState("unknown")         // "未知"
	Armory_WAIT_ONLINE     = ArmoryAppState("wait_online")     // "应用等待在线"
	Armory_WORKING_ONLINE  = ArmoryAppState("working_online")  // "应用在线"
	Armory_WORKING_OFFLINE = ArmoryAppState("working_offline") // "应用离线"
	Armory_READY           = ArmoryAppState("ready")           // "准备中"),
	Armory_BUFFER          = ArmoryAppState("buffer")          // "闲置"),
	Armory_BROKEN          = ArmoryAppState("broken")          // "损坏，维修"),
	Armory_LOCK            = ArmoryAppState("lock")            // "锁定"),
	Armory_UNUSE           = ArmoryAppState("unuse")           // "停用");

)

type CmdbContainerInfoReq struct {
	Name       string
	Type       string
	CpuCore    int64
	MemorySize int64
	ServerId   int64

	PodSn string // 作为关联podSn的外检 server.sn输入
}

type CmdbPodInfoReq struct {
	//pod专用字段
	K8sClusterName string
	K8sNamespace   string
	K8sPodName     string

	InstanceGroup string //应用分组
	HostIp        string //宿主机ip
	HostSn        string //宿主机sn

	ContainerIp   string //容器IP
	ContainerSn   string //容器SN
	ContainerName string //容器名字

	ContainerHostName string //container host name
	ContainerNetwork  string //容器的网络：bridge host

	Site             string //机房
	ArmoryState      string //写到armory的状态
	IpSecurityDomain string //ip安全域

	CpuInfo   CpuInfo //cpu信息
	MemBytes  int64   //内存信息
	DiskBytes int64   //磁盘信息

	AppStage   string
	Unit       string
	DeviceMode string //设备类型：Docker k8s_pod

	SdnType       string // 容器SDN网络类型
	EniInstanceId string // 弹性网卡id

	RegionId        string
	ZoneId          string
	VpcId           string
	VSwitchId       string
	SecurityGroupId string
}

type CpuInfo struct {
	Request int64
	Limit   int64
	CoreNum int
}

type CmdbPodInfo struct {
	Site            string `json:"cabinet.logic_region"`    // 逻辑机房
	CabinetNum      string `json:"cabinet.cabinet_num"`     // 机柜编号
	CabinetPosition string `json:"cabinet_position"`        // 机柜位置
	AppName         string `json:"app.name"`                // 应用名
	NodeGroup       string `json:"app_group.name"`          // 应用分组
	AppUseType      string `json:"app_use_type"`            // 应用用途
	CpuCount        int    `json:"total_cpu_count"`         // 总核数
	DiskSize        int64  `json:"total_disk_size"`         // 磁盘大小 GB
	MemSize         int64  `json:"total_memory_size"`       // 内存大小 MB
	Abb             string `json:"room.abbreviation"`       // 物理房间位置
	SmName          string `json:"standard_model.sm_name"`  // 标准机型
	ResourceOwner   string `json:"resource_owner"`          // 数据所有者
	ParentSn        string `json:"parent_service_tag"`      // 父sn,物理机为机框sn,虚拟机为服务器sn
	ParentHostName  string `json:"parent_host_name"`        // 父设备的hostname
	Ip              string `json:"ip"`                      // 容器或者物理机ip
	Sn              string `json:"sn"`                      // 容器或者物理机sn
	HostName        string `json:"host_name"`               // 主机名
	AppState        string `json:"app_server_state"`        // 应用状态
	ContainerId     string `json:"container_id"`            // 容器ID
	AllocateMode    string `json:"allocate_mode"`           // 分配模式
	Model           string `json:"device_model.model_name"` // 设备型号 Docker容器或者其他类型容器
	SecurityDomain  string `json:"security_domain"`         // 安全域
}

type AppGroupsInfo struct {
	TotalCount int             `json:"totalCount"`
	HasMore    bool            `json:"hasMore"`
	ItemList   []*AppGroupItem `json:"itemList"`
}

type AppGroupItem struct {
	Name      string   `json:"name"`
	OwnerName []string `json:"owner_string"`
	AppName   string   `json:"app.name"`
	SafetyOut int      `json:"safety_out"`
	AppId     int      `json:"app_id"`
}

type AppGroupInfo struct {
	Name               string   `json:"name"`                         // 分组名称
	AppName            string   `json:"appName"`                      // 分组所属应用名称
	SchemaName         *string  `json:"schemaName,omitempty"`         // Aone Schema
	PESeries           []string `json:"pes,omitempty"`                // PE 工号
	EmployeeGroupNames []string `json:"employeeGroupNames,omitempty"` // 用户组名
}

type PEList []string

type AppPEList struct {
	TotalCount int                 `json:"totalCount"`
	HasMore    bool                `json:"hasMore"`
	ItemList   []map[string]PEList `json:"itemList"`
}

type ArmoryInfo struct {
	Id               int    `json:".id"`
	Ip               string `json:"dns_ip"`
	ServiceTag       string `json:"sn"`
	NodeName         string `json:"nodename"`
	NodeGroup        string `json:"nodegroup"`
	Vmparent         string `json:"vmparent"`
	ParentServiceTag string `json:"parent_service_tag"`
	State            string `json:"state"`
	Site             string `json:"site"`
	Model            string `json:"model"`
	ProductName      string `json:"product_name"`
	AppUseType       string `json:"app_use_type"`
}
