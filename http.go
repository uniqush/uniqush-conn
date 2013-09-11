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

	"github.com/uniqush/uniqush-conn/rpc"

	"net/http"
)

func processJsonRequest(w http.ResponseWriter, r *http.Request, req interface{}, center *msgcenter.MessageCenter, proc func(center *msgcenter.MessageCenter, req interface{}) *rpc.Result) {
	if center == nil || req == nil {
		return
	}
	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)
	err := decoder.Decode(req)

	if err != nil {
		res := &rpc.Result{
			Error: err,
		}
		encoder.Encode(res)
		return
	}

	res := proc(center, req)
	encoder.Encode(res)
	return
}

func send(center *msgcenter.MessageCenter, req interface{}) *rpc.Result {
	if r, ok := req.(*rpc.SendRequest); ok {
		return center.Send(r)
	}
	return &rpc.Result{Error: fmt.Errorf("invalid req type")}
}

func forward(center *msgcenter.MessageCenter, req interface{}) *rpc.Result {
	if r, ok := req.(*rpc.ForwardRequest); ok {
		return center.Forward(r)
	}
	return &rpc.Result{Error: fmt.Errorf("invalid req type")}
}

type HttpRequestProcessor struct {
	center *msgcenter.MessageCenter
	addr   string
}

func (self *HttpRequestProcessor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	switch r.URL.Path {
	case "/send.json":
		sendReq := &rpc.SendRequest{}
		processJsonRequest(w, r, sendReq, self.center, send)
	case "/fwd.json":
		fwdReq := &rpc.ForwardRequest{}
		processJsonRequest(w, r, fwdReq, self.center, forward)
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
	return ret
}
