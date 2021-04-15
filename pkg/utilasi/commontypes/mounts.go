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

package commontypes

import "k8s.io/apimachinery/pkg/util/sets"

const (
	MntVarlog         = "/var/log"
	MntAliVmcommon    = "/home/admin/cai/alivmcommon"
	MntTms            = "/home/admin/tms"
	MntStaragent      = "/home/staragent/plugins"
	MntDiamond        = "/home/admin/snapshots/diamond"
	MntLocalData      = "/home/admin/localDatas"
	MntVmcommon       = "/home/admin/vmcommon"
	MntTopFootVm      = "/home/admin/cai/top_foot_vm"
	MntRouteTmpl      = "/etc/route.tmpl"
	MntServiceAccount = "/var/run/secrets/kubernetes.io/serviceaccount"
)

// mounts injected in webhooks,
// webhook will ensure that these mounts is only injected during create or inplace-upgrade
var InjectedMounts = sets.NewString([]string{
	MntVarlog,
	MntAliVmcommon,
	MntTms,
	MntStaragent,
	MntDiamond,
	MntLocalData,
	MntVmcommon,
	MntRouteTmpl,
	MntTopFootVm,
	MntServiceAccount,
}...)
