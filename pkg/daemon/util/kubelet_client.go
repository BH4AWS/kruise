/*
Copyright 2022 The Kruise Authors.

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

package util

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

const (
	kubeletAccessTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

type kubeletClient struct {
	// kubelet access token
	token string
	// https://{NODE_NAME}:Port
	apiUrlPrefix string
	client       *http.Client
}

var (
	kc *kubeletClient

	kubeletPort = flag.Int("kubelet-port", 10255, "The port of kubelet api listen(http). default is 10255")
)

func InitKubeletClient() error {
	kc = &kubeletClient{}
	token, err := ioutil.ReadFile(kubeletAccessTokenPath)
	if err != nil {
		return err
	}
	kc.token = string(token)
	kc.apiUrlPrefix = fmt.Sprintf("http://localhost:%d", *kubeletPort)
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
	}
	kc.client = &http.Client{
		Transport: tr,
		Timeout:   time.Second * 10,
	}
	klog.Infof("Init kubelet client(api:%s) success", kc.apiUrlPrefix)
	return nil
}

// GetNodePods get the pods deployed to the node.
func GetNodePods() (*corev1.PodList, error) {
	url := fmt.Sprintf("%s/pods", kc.apiUrlPrefix)
	bodyBytes, err := kc.doRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	var podList *corev1.PodList
	err = json.Unmarshal(bodyBytes, &podList)
	return podList, err
}

func (c *kubeletClient) doRequest(method, url string, bodyData []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(bodyData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := (*c.client).Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request url(%s) resp status(%s)", url, resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}
