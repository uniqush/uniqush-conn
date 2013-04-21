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
	"net/http"
	"encoding/json"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/msgcenter"
	"time"
	"fmt"
	"io"
)

type sendMessageRequest struct {
	Service string `json:"service"`
	Username string `json:"username"`
	Header map[string]string `json:"header,omitempty"`
	Body []byte `json:"body,omitempty"`
	TTL time.Duration `json:"ttl,omitempty"`
	PosterKey string `json:"key,omitempty"`
}

func parseJson(input io.Reader) (req *sendMessageRequest, err error) {
	req = new(sendMessageRequest)
	decoder := json.NewDecoder(input)
	err = decoder.Decode(req)
	if err != nil {
		req = nil
	}
	return
}

type RequestProcessor struct {
	center *msgcenter.MessageCenter
}

func isPrefix(prefix, str string) bool {
	if len(str) > len(prefix) {
		if str[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (self *RequestProcessor) sendMessage(req *sendMessageRequest) []error {
	if req.TTL < 1 * time.Second {
		req.TTL = 24 * time.Hour
	}

	msg := new(proto.Message)
	msg.Header = make(map[string]string, len(req.Header))
	extra := make(map[string]string, len(req.Header))
	if len(req.Body) > 0 {
		msg.Body = []byte(req.Body)
	}

	var errs []error

	for k, v := range req.Header {
		if isPrefix("notif.", k) {
			if isPrefix("notif.uniqush.", k) {
				errs = append(errs, fmt.Errorf("invalid key %v: notif.uniqush.* are reserved keys", k))
				return errs
			}
			extra[k] = v
		} else {
			msg.Header[k] = v
		}
	}
	if msg.IsEmpty() {
		errs = append(errs, fmt.Errorf("empty message"))
		return errs
	}

	if len(req.PosterKey) == 0 {
		_, errs = self.center.SendMail(req.Service, req.Username, msg, extra, req.TTL)
	} else {
		_, errs = self.center.SendPoster(req.Service, req.Username, msg, extra, req.PosterKey, req.TTL)
	}
	return errs
}

type HttpRequestProcessor struct {
	RequestProcessor
	addr string
}

func NewHttpRequestProcessor(addr string, center *msgcenter.MessageCenter) *HttpRequestProcessor {
	ret := new(HttpRequestProcessor)
	ret.addr = addr
	ret.center = center
	return ret
}

func (self *HttpRequestProcessor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	req, err := parseJson(r.Body)
	if err != nil {
		fmt.Fprintf(w, "Invalid input: %v\r\n", err)
		return
	}
	errs := self.sendMessage(req)
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(w, "%v\r\n", err)
		}
		return
	}
	fmt.Fprintf(w, "Success\r\n")
	return
}

func (self *HttpRequestProcessor) Start() error {
	err := http.ListenAndServe(self.addr, self)
	return err
}

