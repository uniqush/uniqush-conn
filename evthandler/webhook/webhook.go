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

package webhook

import (
	"bytes"
	"encoding/json"
	"github.com/uniqush/uniqush-conn/rpc"
	"net"
	"net/http"
	"time"
)

type WebHook interface {
	SetURL(urls ...string)
	SetTimeout(timeout time.Duration)
	SetDefault(d int)
}

type webHook struct {
	URLs    []string
	Timeout time.Duration
	Default int
}

func (self *webHook) SetURL(urls ...string) {
	self.URLs = urls
}

func (self *webHook) SetTimeout(timeout time.Duration) {
	self.Timeout = timeout
}

func (self *webHook) SetDefault(d int) {
	self.Default = d
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

func (self *webHook) post(data interface{}, out interface{}, requireOut bool) int {
	ret := self.Default
	for _, url := range self.URLs {
		status := self.postSingle(url, data, out, requireOut)
		if status == 200 {
			if out != nil {
				return status
			}
			ret = status
		}
	}
	return ret
}

func (self *webHook) postSingle(url string, data interface{}, out interface{}, requireOut bool) int {
	if len(url) == 0 || url == "none" {
		return self.Default
	}
	jdata, err := json.Marshal(data)
	if err != nil {
		return self.Default
	}
	c := http.Client{
		Transport: &http.Transport{
			Dial: timeoutDialler(self.Timeout),
		},
	}
	resp, err := c.Post(url, "application/json", bytes.NewReader(jdata))
	if err != nil {
		return self.Default
	}
	defer resp.Body.Close()

	if out != nil {
		e := json.NewDecoder(resp.Body)
		err = e.Decode(out)
		if err != nil && requireOut {
			return self.Default
		}
	}
	return resp.StatusCode
}

type loginEvent struct {
	Service  string `json:"service"`
	Username string `json:"username"`
	ConnID   string `json:"connId"`
	Addr     string `json:"addr"`
}

type LoginHandler struct {
	webHook
}

func (self *LoginHandler) OnLogin(service, username, connId, addr string) {
	self.post(&loginEvent{service, username, connId, addr}, nil, false)
}

type logoutEvent struct {
	Service  string `json:"service"`
	Username string `json:"username"`
	ConnID   string `json:"connId"`
	Addr     string `json:"addr"`
	Reason   string `json:"reason,omitempty"`
}

type LogoutHandler struct {
	webHook
}

func (self *LogoutHandler) OnLogout(service, username, connId, addr string, reason error) {
	evt := &logoutEvent{
		Service:  service,
		Username: username,
		ConnID:   connId,
		Addr:     addr,
	}

	if reason != nil {
		evt.Reason = reason.Error()
	}
	self.post(evt, nil, false)
}

type messageEvent struct {
	ConnID   string       `json:"connId"`
	Msg      *rpc.Message `json:"msg"`
	Service  string       `json:"service"`
	Username string       `json:"username"`
}

type MessageHandler struct {
	webHook
}

func (self *MessageHandler) OnMessage(service, username, connId string, msg *rpc.Message) {
	evt := &messageEvent{
		Service:  service,
		Username: username,
		ConnID:   connId,
		Msg:      msg,
	}
	self.post(evt, nil, false)
}

type errorEvent struct {
	Service  string `json:"service"`
	Username string `json:"username"`
	ConnID   string `json:"connId"`
	Addr     string `json:"addr"`
	Reason   string `json:"reason"`
}

type ErrorHandler struct {
	webHook
}

func (self *ErrorHandler) OnError(service, username, connId, addr string, reason error) {
	self.post(&errorEvent{service, username, connId, addr, reason.Error()}, nil, false)
}

type ForwardRequestHandler struct {
	webHook
	maxTTL time.Duration
}

type forwardEvent struct {
	SenderService   string        `json:"sender-service"`
	Sender          string        `json:"sender"`
	ReceiverService string        `json:"receiver-service"`
	Receivers       []string      `json:"receivers"`
	Message         *rpc.Message  `json:"msg"`
	TTL             time.Duration `json:"ttl"`
}

type forwardDecision struct {
	ShouldForward bool              `json:"should-forward"`
	ShouldPush    bool              `json:"should-push"`
	PushInfo      map[string]string `json:"push-info"`
}

func (self *ForwardRequestHandler) ShouldForward(senderService, sender, receiverService string, receivers []string,
	ttl time.Duration, msg *rpc.Message) (shouldForward, shouldPush bool, pushInfo map[string]string) {
	fwd := &forwardEvent{
		Sender:          sender,
		SenderService:   senderService,
		Receivers:       receivers,
		ReceiverService: receiverService,
		TTL:             ttl,
		Message:         msg,
	}

	res := &forwardDecision{
		ShouldForward: true,
		ShouldPush:    true,
		PushInfo:      make(map[string]string, 10),
	}

	status := self.post(fwd, res, false)
	if status != 200 {
		res.ShouldForward = false
		res.ShouldPush = false
	}
	return res.ShouldForward, res.ShouldPush, res.PushInfo
}

func (self *ForwardRequestHandler) SetMaxTTL(ttl time.Duration) {
	self.maxTTL = ttl
}

func (self *ForwardRequestHandler) MaxTTL() time.Duration {
	return self.maxTTL
}

type authEvent struct {
	Service  string `json:"service"`
	Username string `json:"username"`
	ConnId   string `json:"conn-id"`
	Token    string `json:"token"`
	Addr     string `json:"addr"`
}

type AuthHandler struct {
	webHook
}

func (self *AuthHandler) Authenticate(srv, usr, connId, token, addr string) (pass bool, redir []string, err error) {
	evt := new(authEvent)
	evt.Service = srv
	evt.Username = usr
	evt.ConnId = connId
	evt.Token = token
	evt.Addr = addr
	pass = self.post(evt, redir, false) == 200
	return
}

type pushRelatedEvent struct {
	Service  string            `json:"service"`
	Username string            `json:"username"`
	Info     map[string]string `json:"info"`
}

type SubscribeHandler struct {
	webHook
}

func (self *SubscribeHandler) ShouldSubscribe(service, username string, info map[string]string) bool {
	evt := new(pushRelatedEvent)
	evt.Service = service
	evt.Username = username
	evt.Info = info
	return self.post(evt, nil, false) == 200
}

type UnsubscribeHandler struct {
	webHook
}

func (self *UnsubscribeHandler) OnUnsubscribe(service, username string, info map[string]string) {
	evt := &pushRelatedEvent{}
	evt.Service = service
	evt.Username = username
	evt.Info = info
	self.post(evt, nil, false)
	return
}
