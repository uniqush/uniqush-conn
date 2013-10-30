/*
 * Copyright 2013 Nan Deng
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

package push

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TODO: Use decorator pattern to implement an aggregate Push interface

type Push interface {
	Subscribe(service, username string, info map[string]string) error
	Unsubscribe(service, username string, info map[string]string) error
	Push(service, username, senderService, senderUsername string, info map[string]string, id string, size int) error
}

type uniqushPush struct {
	addr    string
	timeout time.Duration
}

func NewUniqushPushClient(addr string, timeout time.Duration) Push {
	ret := new(uniqushPush)
	ret.addr = addr
	ret.timeout = timeout
	return ret
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

func (self *uniqushPush) postReadLines(path string, data url.Values, nrLines int) (value string, err error) {
	if len(path) == 0 {
		return
	}

	url := fmt.Sprintf("http://%v/%v", self.addr, path)

	c := http.Client{
		Transport: &http.Transport{
			Dial: timeoutDialler(self.timeout),
		},
	}
	resp, err := c.PostForm(url, data)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if nrLines > 0 {
		respBuf := bufio.NewReader(resp.Body)
		line := make([]byte, 0, nrLines*512)
		for i := 0; i < nrLines; i++ {
			l, _, e := respBuf.ReadLine()
			if e != nil {
				err = e
				return
			}
			line = append(line, l...)
		}
		value = string(line)
	}
	return
}

func (self *uniqushPush) post(path string, data url.Values) error {
	_, err := self.postReadLines(path, data, 0)
	return err
}

func (self *uniqushPush) subscribe(service, username string, info map[string]string, sub bool) error {
	data := url.Values{}
	data.Add("service", service)
	data.Add("subscriber", username)

	for k, v := range info {
		switch k {
		case "pushservicetype":
			fallthrough
		case "regid":
			fallthrough
		case "devtoken":
			fallthrough
		case "account":
			data.Add(k, v)
		}
	}
	path := "unsubscribe"
	if sub {
		path = "subscribe"
	}
	err := self.post(path, data)
	return err
}

func (self *uniqushPush) NrDeliveryPoints(service, username string) int {
	data := url.Values{}
	data.Add("service", service)
	data.Add("subscriber", username)
	v, err := self.postReadLines("nrdp", data, 1)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0
	}
	return n
}

func (self *uniqushPush) Subscribe(service, username string, info map[string]string) error {
	return self.subscribe(service, username, info, true)
}

func (self *uniqushPush) Unsubscribe(service, username string, info map[string]string) error {
	return self.subscribe(service, username, info, false)
}

func (self *uniqushPush) Push(service, username, senderService, senderUsername string, info map[string]string, id string, size int) error {
	if len(service) == 0 {
		return fmt.Errorf("NoService")
	}
	if len(username) == 0 {
		return fmt.Errorf("NoReceiver")
	}
	data := url.Values{}
	for k, v := range info {
		if strings.HasPrefix(strings.ToLower(k), "uq.") ||
			strings.HasPrefix(strings.ToLower(k), "uniqush.") {
			// reserved prefixes.
			continue
		}
		data.Set(k, v)
	}

	// The format of the parameter string is:
	// id,size,service,username,senderService,senderUsername
	// The last two are optional if the message is sent from the server (not forwarded by another user)
	// This id part is hex number --- so that we can save space.
	param := make([]rune, 0, len(id)+len(service)+len(username)+len(senderService)+len(senderUsername)+32)

	param = append(param, []rune(id)...)
	param = append(param, rune(','))
	param = append(param, []rune(fmt.Sprintf("%x,", size))...)
	param = append(param, []rune(service)...)
	param = append(param, rune(','))
	param = append(param, []rune(username)...)
	if len(senderUsername) > 0 {
		param = append(param, rune(','))
		if len(senderService) > 0 {
			param = append(param, []rune(senderService)...)
			param = append(param, rune(','))
		} else {
			param = append(param, []rune(service)...)
			param = append(param, rune(','))
		}
		param = append(param, []rune(senderUsername)...)
	}
	data.Set("uq.", string(param))
	data.Set("service", service)
	data.Set("subscriber", username)

	err := self.post("push", data)
	return err
}
