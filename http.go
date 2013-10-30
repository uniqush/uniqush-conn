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
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/uniqush/uniqush-conn/msgcenter"
	"io"
	"io/ioutil"
	"net/url"

	"strings"
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

func redirect(proc *HttpRequestProcessor, center *msgcenter.MessageCenter, req interface{}) *rpc.Result {
	if r, ok := req.(*rpc.RedirectRequest); ok {
		return center.Redirect(r)
	}
	return &rpc.Result{Error: "invalid req type"}
}

func checkUserStatus(proc *HttpRequestProcessor, center *msgcenter.MessageCenter, req interface{}) *rpc.Result {
	if r, ok := req.(*rpc.UserStatusQuery); ok {
		return center.CheckUserStatus(r)
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

	upath := strings.TrimSpace(r.URL.Path)
	switch upath {
	case rpc.SEND_MESSAGE_PATH:
		sendReq := &rpc.SendRequest{}
		self.processJsonRequest(w, r, sendReq, self.center, send)
	case rpc.FORWARD_MESSAGE_PATH:
		fwdReq := &rpc.ForwardRequest{}
		self.processJsonRequest(w, r, fwdReq, self.center, forward)
	case rpc.USER_STATUS_QUERY_PATH:
		usrStatusQuery := &rpc.UserStatusQuery{}
		self.processJsonRequest(w, r, usrStatusQuery, self.center, checkUserStatus)
	case rpc.REDIRECT_CLIENT_PATH:
		redirReq := &rpc.RedirectRequest{}
		self.processJsonRequest(w, r, redirReq, self.center, redirect)
	case "/join.json":
		instance := &rpc.UniqushConnInstance{}
		self.processJsonRequest(w, r, instance, self.center, addInstance)
	case "/id":
		fmt.Fprintf(w, "%v\r\n", self.myId)
	case "/services":
		if r.Method == "GET" {
			srvs, err := json.Marshal(self.center.AllServices())
			if err != nil {
				fmt.Fprintf(w, "[]\r\n")
				return
			}
			fmt.Fprintf(w, "%v", string(srvs))
		}
	default:
		pathb := bytes.Trim([]byte(upath), "/")
		elems := bytes.Split(pathb, []byte("/"))
		var action string
		var srv string
		if len(elems) == 2 {
			action = string(elems[0])
			srv = string(elems[1])
		}
		switch action {
		case "nr-conns":
			fmt.Fprintf(w, "%v\r\n", self.center.NrConns(srv))
		case "nr-users":
			fmt.Fprintf(w, "%v\r\n", self.center.NrUsers(srv))
		case "all-users":
			usrs, err := json.Marshal(self.center.AllUsernames(srv))
			if err != nil {
				fmt.Fprintf(w, "[]\r\n")
				return
			}
			fmt.Fprintf(w, "%v", string(usrs))
		}
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
	var d [16]byte
	io.ReadFull(rand.Reader, d[:])
	ret.myId = fmt.Sprintf("%x-%v", time.Now().Unix(), base64.URLEncoding.EncodeToString(d[:]))
	return ret
}
