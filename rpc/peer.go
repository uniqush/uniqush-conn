/*
 * Copyright 2012 Nan Deng
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	SEND_MESSAGE_PATH    = "/send.json"
	FORWARD_MESSAGE_PATH = "/fwd.json"
)

type UniqushConnPeer interface {
	Send(req *SendRequest) *Result
	Forward(req *ForwardRequest) *Result
}

type UniqushConnInstance struct {
	addr    string
	timeout time.Duration
}

func NewUniqushConnInstance(u *url.URL, timeout time.Duration) (instance *UniqushConnInstance, err error) {
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%v is not supported", u.Scheme)
	}
	instance = new(UniqushConnInstance)
	instance.timeout = timeout
	instance.addr = u.String()
	return
}

func timeoutDialler(ns time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		c, err := net.Dial(netw, addr)
		if err != nil {
			return nil, err
		}
		if ns.Seconds() > 0.0 {
			c.SetDeadline(time.Now().Add(ns))
		}
		return c, nil
	}
}

func (self *UniqushConnInstance) post(url string, data interface{}, out interface{}) int {
	if len(url) == 0 || url == "none" {
		return 400
	}
	jdata, err := json.Marshal(data)
	if err != nil {
		return 400
	}
	c := http.Client{
		Transport: &http.Transport{
			Dial: timeoutDialler(self.timeout),
		},
	}
	resp, err := c.Post(url, "application/json", bytes.NewReader(jdata))
	if err != nil {
		return 400
	}
	defer resp.Body.Close()

	if out != nil {
		e := json.NewDecoder(resp.Body)
		err = e.Decode(out)
		if err != nil {
			return 400
		}
	}
	return resp.StatusCode
}

func (self *UniqushConnInstance) requestThenResult(path string, req interface{}) *Result {
	result := new(Result)
	status := self.post(self.addr+path, req, result)
	if status != 200 {
		return nil
	}
	return result
}

func (self *UniqushConnInstance) Send(req *SendRequest) *Result {
	return self.requestThenResult(SEND_MESSAGE_PATH, req)
}

func (self *UniqushConnInstance) Forward(req *ForwardRequest) *Result {
	return self.requestThenResult(FORWARD_MESSAGE_PATH, req)
}
