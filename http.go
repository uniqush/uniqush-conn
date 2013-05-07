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
	"github.com/uniqush/uniqush-conn/proto"
	"io"
	"net/http"
	"time"
)

type sendMessageRequest struct {
	Service   string            `json:"service"`
	Username  string            `json:"username"`
	Header    map[string]string `json:"header,omitempty"`
	Body      []byte            `json:"body,omitempty"`
	TTL       string            `json:"ttl,omitempty"`
	PosterKey string            `json:"key,omitempty"`
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

func (self *RequestProcessor) sendMessage(req *sendMessageRequest) (errs []error, res []*msgcenter.Result) {
	ttl := 24 * time.Hour
	if len(req.TTL) > 0 {
		var e error
		ttl, e = time.ParseDuration(req.TTL)
		if e != nil {
			errs = append(errs, e)
			return
		}
	}

	msg := new(proto.Message)
	msg.Header = make(map[string]string, len(req.Header))
	extra := make(map[string]string, len(req.Header))
	if len(req.Body) > 0 {
		msg.Body = []byte(req.Body)
	}

	for k, v := range req.Header {
		if isPrefix("notif.", k) {
			if isPrefix("notif.uniqush.", k) {
				errs = append(errs, fmt.Errorf("invalid key %v: notif.uniqush.* are reserved keys", k))
				return
			}
			extra[k] = v
		} else {
			msg.Header[k] = v
		}
	}
	if msg.IsEmpty() {
		errs = append(errs, fmt.Errorf("empty message"))
		return
	}

	if len(req.PosterKey) == 0 {
		res = self.center.SendMail(req.Service, req.Username, msg, extra, ttl)
	} else {
		res = self.center.SendPoster(req.Service, req.Username, msg, extra, req.PosterKey, ttl)
	}
	return
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
	errs, res := self.sendMessage(req)
	for _, e := range errs {
		fmt.Fprintf(w, "%v\r\n", e)
	}
	for _, r := range res {
		fmt.Fprintf(w, "%v\r\n", r)
	}
	return
}

func (self *HttpRequestProcessor) Start() error {
	http.Handle("/send.json", self)
	err := http.ListenAndServe(self.addr, nil)
	return err
}
