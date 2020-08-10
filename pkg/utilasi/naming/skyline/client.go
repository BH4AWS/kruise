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
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/openkruise/kruise/pkg/utilasi/config"
	"github.com/openkruise/kruise/pkg/utilasi/httpclient"
	"github.com/openkruise/kruise/pkg/utilasi/semaphore"
)

type skylineManager struct {
	sema         *semaphore.Semaphore // 用来控制并发数
	getSkyConfig func() *config.SkyConfig
}

// 新建一个skylineManager
func newSkylineManager(getSkyConfig func() *config.SkyConfig) *skylineManager {
	skyConfig := getSkyConfig()
	return &skylineManager{
		sema:         semaphore.New(skyConfig.Concurrency),
		getSkyConfig: getSkyConfig,
	}
}

// 查询，接口文档：https://yuque.antfin-inc.com/at7ocb/qbn0oy/kzr3tz
func (skyline *skylineManager) query(queryItem *CommonQueryInfo, value interface{}) (*Result, error) {
	queryParam := &QueryParam{
		Auth:  skyline.buildAuth(),
		Query: queryItem,
	}
	body, err := json.Marshal(queryParam)
	if err != nil {
		return nil, err
	}
	result, err := skyline.httpRequestToSky(queryUri, body, value)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		err = fmt.Errorf("skyline query queryItem:%+v, request uri: %v, body: %v, message: %v",
			queryItem, queryUri, string(body), result.ErrorMessage)
		return nil, err
	}
	return result, nil
}

// 简单的认证
func (skyline *skylineManager) buildAuth() *auth {
	skyConfig := skyline.getSkyConfig()
	auth := &auth{
		Account:   skyConfig.User,
		AppName:   skyConfig.App,
		Timestamp: time.Now().Unix(),
	}
	md5Cal := md5.New()
	io.WriteString(md5Cal, auth.Account)
	io.WriteString(md5Cal, skyConfig.Key)
	io.WriteString(md5Cal, fmt.Sprintf("%v", auth.Timestamp))
	auth.Signature = hex.EncodeToString(md5Cal.Sum(nil))
	return auth
}

// 绑定用户信息
func (skyline *skylineManager) buildSkyOperator(operatorType interface{}) *skyOperator {
	skyOperator := &skyOperator{
		Type:     operatorType,
		Nick:     "naming-controller",
		WorkerId: "naming-controller",
	}
	return skyOperator
}

// 通用的http-post
func (skyline *skylineManager) httpRequestToSky(uri string, body []byte, value interface{}) (*Result, error) {
	skyConfig := skyline.getSkyConfig()
	urls := []string{fmt.Sprintf(uri, skyConfig.Url)}
	if len(skyConfig.SecondUrl) > 0 {
		urls = append(urls, fmt.Sprintf(uri, skyConfig.SecondUrl))
	}

	var (
		resByte []byte
		rErr    error
	)

	for _, url := range urls {
		resByte, rErr = httpclient.HttpPostJsonWithHeaders(url, body, map[string]string{"account": skyConfig.User}, nil)
		if rErr != nil {
			continue
		}

		result := &Result{}
		result.Value = value
		rErr = json.Unmarshal(resByte, result)
		if rErr != nil {
			continue
		}
		return result, nil
	}

	return nil, rErr
}
