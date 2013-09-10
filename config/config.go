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
	"github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/push"
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

type ServiceConfig struct {
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
	PushHandler        evthandler.PushHandler

	PushService push.Push
}
