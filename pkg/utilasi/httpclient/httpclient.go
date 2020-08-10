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

package httpclient

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	utils "github.com/openkruise/kruise/pkg/utilasi"
	"github.com/pkg/errors"
)

const (
	DEFAULT_DIAL_TIMEOUT    int = 10
	DEFAULT_END2END_TIMEOUT int = 120

	RETRY_COUNT              = 2
	RETRY_INTERVAL           = 10
	RETRY_INTERVAL_INCREMENT = 10
)

func HttpPost(httpUrl string, params map[string]string) ([]byte, error) {
	return httpVisit("POST", httpUrl, params, DEFAULT_DIAL_TIMEOUT, DEFAULT_END2END_TIMEOUT, RETRY_COUNT,
		RETRY_INTERVAL, RETRY_INTERVAL_INCREMENT)
}

func HttpGet(httpUrl string, params map[string]string) ([]byte, error) {
	return httpVisit("GET", httpUrl, params, DEFAULT_DIAL_TIMEOUT, DEFAULT_END2END_TIMEOUT, RETRY_COUNT,
		RETRY_INTERVAL, RETRY_INTERVAL_INCREMENT)
}

func HttpGetWithTimeOutAndRetry(httpUrl string, params map[string]string, dialTimeOut, e2eTimeOut,
	retryCount, retryInterval, retryIntervalInc int) ([]byte, error) {
	return httpVisit("GET", httpUrl, params, dialTimeOut, e2eTimeOut, retryCount, retryInterval, retryIntervalInc)
}

func httpVisit(httpMethod string, httpUrl string, params map[string]string, dialTimeOut, e2eTimeOut,
	retryCount, retryInterval, retryIntervalInc int) ([]byte, error) {
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(dialTimeOut) * time.Second,
		}).Dial,
	}
	var client = &http.Client{
		Timeout:   time.Duration(e2eTimeOut) * time.Second,
		Transport: netTransport,
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	var data []byte
	err := utils.RetryInc(func() (err error) {
		var resp *http.Response

		if httpMethod == "GET" {
			resp, err = client.Get(fmt.Sprintf("%v?%v", httpUrl, values.Encode()))
		} else {
			resp, err = client.PostForm(httpUrl, values)
		}
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		data, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode == 404 {
			data = nil
			return nil
		} else if resp.StatusCode != 200 {
			return errors.New(fmt.Sprintf("request %v failed, Status:%v, msg:%v",
				httpUrl, resp.Status, string(data)))
		}
		return nil
	}, "httpVisit", retryCount, retryInterval, retryIntervalInc)

	return data, err
}

func HttpPostJsonWithHeaders(httpUrl string, body []byte, headers map[string]string, params map[string]string) ([]byte, error) {
	return HttpPostJsonWithHeadersWithTime(httpUrl, body, headers, params, DEFAULT_DIAL_TIMEOUT, DEFAULT_END2END_TIMEOUT)
}

func HttpPostJsonWithHeadersWithTime(httpUrl string, body []byte, headers map[string]string, params map[string]string,
	timeoutInSecond int, end2endTimeoutInSecond int) ([]byte, error) {
	var (
		err  error
		req  *http.Request
		resp *http.Response
	)

	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(timeoutInSecond) * time.Second,
		}).Dial,
	}
	var client = &http.Client{
		Timeout:   time.Duration(end2endTimeoutInSecond) * time.Second,
		Transport: netTransport,
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	var data []byte
	err = utils.RetryInc(func() (err error) {
		req, err = http.NewRequest("POST", fmt.Sprintf("%v?%v", httpUrl, values.Encode()), bytes.NewReader(body))
		if err != nil {
			return err
		}

		req.Header.Add("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Add(k, v)
		}

		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		data, err = ioutil.ReadAll(resp.Body)

		if resp.StatusCode != 200 {
			if err != nil {
				return errors.New(fmt.Sprintf("request %v with json %v and Header %v failed, StatusCode:%v, parse body error:%v",
					httpUrl, string(body), req.Header, resp.StatusCode, err.Error()))
			}
			return errors.New(fmt.Sprintf("request %v with json %v and Header %v failed, Status:%v, msg:%v",
				httpUrl, string(body), req.Header, resp.Status, string(data)))
		}
		return nil
	}, "HttpPostJsonWithHeaders", RETRY_COUNT, RETRY_INTERVAL, RETRY_INTERVAL_INCREMENT)

	return data, err
}

func HttpDelete(httpUrl string) ([]byte, error) {
	var (
		err  error
		req  *http.Request
		resp *http.Response
	)

	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(DEFAULT_DIAL_TIMEOUT) * time.Second,
		}).Dial,
	}
	var client = &http.Client{
		Timeout:   time.Duration(DEFAULT_END2END_TIMEOUT) * time.Second,
		Transport: netTransport,
	}

	req, err = http.NewRequest("DELETE", httpUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	var data []byte
	err = utils.RetryInc(func() (err error) {
		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			data, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return errors.New(fmt.Sprintf("request %v failed, StatusCode:%v, parse body error:%v",
					httpUrl, resp.StatusCode, err.Error()))
			}
			if resp.StatusCode == 404 {
				return nil
			}
			return errors.New(fmt.Sprintf("request %v failed, StatusCode:%v, msg:%v",
				httpUrl, resp.StatusCode, string(data)))
		}
		return nil
	}, "HttpDelete", RETRY_COUNT, RETRY_INTERVAL, RETRY_INTERVAL_INCREMENT)

	return data, err
}
