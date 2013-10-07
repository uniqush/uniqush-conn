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

package main

import (
	"encoding/json"
	"fmt"
	"github.com/uniqush/uniqush-conn/msgcenter"
	"io/ioutil"
	"math/rand"
	"net/url"
	"time"

	"github.com/uniqush/uniqush-conn/rpc"

	"net/http"
)

func (self *HttpRequestProcessor) processJsonRequest(w http.ResponseWriter,
	r *http.Request, req interface{},
	center *msgcenter.MessageCenter,
	proc func(p *HttpRequestProcessor, center *msgcenter.MessageCenter, req interface{}) *rpc.Result) {
	if center == nil || req == nil {
		return
	}
	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)
	err := decoder.Decode(req)

	if err != nil {
		res := &rpc.Result{
			Error: err.Error(),
		}
		encoder.Encode(res)
		return
	}

	res := proc(self, center, req)
	encoder.Encode(res)
	return
}

func send(proc *HttpRequestProcessor, center *msgcenter.MessageCenter, req interface{}) *rpc.Result {
	if r, ok := req.(*rpc.SendRequest); ok {
		return center.Send(r)
	}
	return &rpc.Result{Error: "invalid req type"}
}

func forward(proc *HttpRequestProcessor, center *msgcenter.MessageCenter, req interface{}) *rpc.Result {
	if r, ok := req.(*rpc.ForwardRequest); ok {
		return center.Forward(r)
	}
	return &rpc.Result{Error: "invalid req type"}
}

func addInstance(proc *HttpRequestProcessor, center *msgcenter.MessageCenter, req interface{}) *rpc.Result {
	if r, ok := req.(*rpc.UniqushConnInstance); ok {
		u, err := url.Parse(r.Addr)
		if err != nil {
			return &rpc.Result{Error: err.Error()}
		}

		isme, err := proc.isMyself(u.String())
		if err != nil {
			return &rpc.Result{Error: err.Error()}
		}
		if isme {
			return &rpc.Result{Error: "This is me"}
		}
		instance, err := rpc.NewUniqushConnInstance(u, r.Timeout)
		if err != nil {
			return &rpc.Result{Error: err.Error()}
		}

		center.AddPeer(instance)
		return &rpc.Result{Error: "Success"}
	}
	return &rpc.Result{Error: "invalid req type"}
}

type HttpRequestProcessor struct {
	center *msgcenter.MessageCenter
	addr   string
	myId   string
}

func (self *HttpRequestProcessor) isMyself(addr string) (bool, error) {
	resp, err := http.Get(addr + "/id")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	peerId := string(body)

	if peerId == self.myId {
		return true, nil
	}
	return false, nil
}

func (self *HttpRequestProcessor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	switch r.URL.Path {
	case rpc.SEND_MESSAGE_PATH:
		sendReq := &rpc.SendRequest{}
		self.processJsonRequest(w, r, sendReq, self.center, send)
	case rpc.FORWARD_MESSAGE_PATH:
		fwdReq := &rpc.ForwardRequest{}
		self.processJsonRequest(w, r, fwdReq, self.center, forward)
	case "/join.json":
		instance := &rpc.UniqushConnInstance{}
		self.processJsonRequest(w, r, instance, self.center, addInstance)
	case "/id":
		fmt.Fprintf(w, "%v", self.myId)
	}
}

func (self *HttpRequestProcessor) Start() error {
	http.Handle("/", self)
	err := http.ListenAndServe(self.addr, nil)
	return err
}

func NewHttpRequestProcessor(addr string, center *msgcenter.MessageCenter) *HttpRequestProcessor {
	ret := new(HttpRequestProcessor)
	ret.addr = addr
	ret.center = center
	ret.myId = fmt.Sprintf("%v-%v-%v", time.Now().UnixNano(), rand.Int63(), rand.Int63())
	return ret
}
