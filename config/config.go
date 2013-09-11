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

package config

import (
	"fmt"
	"github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/msgcache"

	"github.com/uniqush/uniqush-conn/push"
	"github.com/uniqush/uniqush-conn/rpc"
	"net"
	"time"
)

type Config struct {
	HandshakeTimeout time.Duration
	HttpAddr         string
	Auth             evthandler.Authenticator
	ErrorHandler     evthandler.ErrorHandler
	filename         string
	srvConfig        map[string]*ServiceConfig
	defaultConfig    *ServiceConfig
}

func (self *Config) OnError(addr net.Addr, err error) {
	if self == nil || self.ErrorHandler == nil {
		return
	}
	go self.ErrorHandler.OnError("", "", "", addr.String(), err)
	return
}

func (self *Config) AllServices() []string {
	ret := make([]string, 0, len(self.srvConfig))
	for srv, _ := range self.srvConfig {
		ret = append(ret, srv)
	}
	return ret
}

func (self *Config) ReadConfig(srv string) *ServiceConfig {
	if ret, ok := self.srvConfig[srv]; ok {
		return ret
	}
	return self.defaultConfig
}

func (self *Config) Authenticate(srv, usr, token, addr string) (bool, error) {
	if self == nil || self.Auth == nil {
		return false, nil
	}
	return self.Auth.Authenticate(srv, usr, token, addr)
}

type ServiceConfig struct {
	ServiceName       string
	MaxNrConns        int
	MaxNrUsers        int
	MaxNrConnsPerUser int

	MsgCache msgcache.Cache

	LoginHandler          evthandler.LoginHandler
	LogoutHandler         evthandler.LogoutHandler
	MessageHandler        evthandler.MessageHandler
	ForwardRequestHandler evthandler.ForwardRequestHandler
	ErrorHandler          evthandler.ErrorHandler

	// Push related web hooks
	SubscribeHandler   evthandler.SubscribeHandler
	UnsubscribeHandler evthandler.UnsubscribeHandler

	PushService push.Push
}

func (self *ServiceConfig) clone(srv string, dst *ServiceConfig) *ServiceConfig {
	if self == nil {
		dst = new(ServiceConfig)
		return dst
	}
	if dst == nil {
		dst = new(ServiceConfig)
	}
	dst.ServiceName = srv
	dst.MaxNrConns = self.MaxNrConns
	dst.MaxNrUsers = self.MaxNrUsers
	dst.MaxNrConnsPerUser = self.MaxNrConnsPerUser

	dst.MsgCache = self.MsgCache

	dst.LoginHandler = self.LoginHandler
	dst.LogoutHandler = self.LogoutHandler
	dst.MessageHandler = self.MessageHandler
	dst.ForwardRequestHandler = self.ForwardRequestHandler
	dst.ErrorHandler = self.ErrorHandler

	// Push related web hooks
	dst.SubscribeHandler = self.SubscribeHandler
	dst.UnsubscribeHandler = self.UnsubscribeHandler

	dst.PushService = self.PushService
	return dst
}

func (self *ServiceConfig) Cache() msgcache.Cache {
	if self == nil {
		return nil
	}
	return self.MsgCache
}
func (self *ServiceConfig) Subscribe(req *rpc.SubscribeRequest) {
	if req.Subscribe {
		self.subscribe(req.Username, req.Params)
	} else {
		self.unsubscribe(req.Username, req.Params)
	}
}

func (self *ServiceConfig) subscribe(username string, info map[string]string) {
	if self == nil || self.PushService == nil {
		return
	}
	go func() {
		if self.shouldSubscribe(self.ServiceName, username, info) {
			self.PushService.Subscribe(self.ServiceName, username, info)
		}
	}()
	return
}

func (self *ServiceConfig) unsubscribe(username string, info map[string]string) {
	if self == nil || self.PushService == nil {
		return
	}
	go self.PushService.Unsubscribe(self.ServiceName, username, info)
	return
}

type connDescriptor interface {
	RemoteAddr() net.Addr
	Service() string
	Username() string
	UniqId() string
}

func (self *ServiceConfig) OnError(c connDescriptor, err error) {
	if self == nil || self.ErrorHandler == nil {
		return
	}
	go self.ErrorHandler.OnError(c.Service(), c.Username(), c.UniqId(), c.RemoteAddr().String(), err)
	return
}

func (self *ServiceConfig) OnLogin(c connDescriptor) {
	if self == nil || self.LoginHandler == nil {
		return
	}
	go self.LoginHandler.OnLogin(c.Service(), c.Username(), c.UniqId(), c.RemoteAddr().String())
	return
}

func (self *ServiceConfig) OnLogout(c connDescriptor, reason error) {
	if self == nil || self.LogoutHandler == nil {
		return
	}
	go self.LogoutHandler.OnLogout(c.Service(), c.Username(), c.UniqId(), c.RemoteAddr().String(), reason)
	return
}

func (self *ServiceConfig) OnMessage(c connDescriptor, msg *rpc.Message) {
	if self == nil || self.MessageHandler == nil {
		return
	}
	go self.MessageHandler.OnMessage(c.Service(), c.Username(), c.UniqId(), msg)
	return
}

func (self *ServiceConfig) CacheMessage(username string, mc *rpc.MessageContainer, ttl time.Duration) (string, error) {
	if self == nil || self.MsgCache == nil {
		return "", fmt.Errorf("no cache available")
	}
	return self.MsgCache.CacheMessage(self.ServiceName, username, mc, ttl)
}

func (self *ServiceConfig) Push(username, senderService, senderName string, info map[string]string, msgId string, size int) {
	if self == nil || self.PushService == nil {
		return
	}
	self.PushService.Push(self.ServiceName, username, senderService, senderName, info, msgId, size)
}

func (self *ServiceConfig) ShouldForward(fwdreq *rpc.ForwardRequest) (shouldForward, shouldPush bool, pushInfo map[string]string) {
	if self == nil || self.ForwardRequestHandler == nil {
		return false, false, nil
	}
	return self.ForwardRequestHandler.ShouldForward(fwdreq.SenderService, fwdreq.Sender, fwdreq.ReceiverService, fwdreq.Receiver, fwdreq.TTL, fwdreq.Message)
}

func (self *ServiceConfig) shouldSubscribe(srv, usr string, info map[string]string) bool {
	if self == nil || self.SubscribeHandler == nil {
		return false
	}
	return self.SubscribeHandler.ShouldSubscribe(srv, usr, info)
}
