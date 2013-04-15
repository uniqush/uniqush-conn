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

package configparser

import (
	"fmt"
	"github.com/kylelemons/go-gypsy/yaml"
	"github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/evthandler/webhook"
	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/msgcenter"
	"strconv"
	"time"
)

type Config struct {
	uniqushPushAddr string
	filename      string
	srvConfig     map[string]*msgcenter.ServiceConfig
	defaultConfig *msgcenter.ServiceConfig
}

func (self *Config) UniqushPushAddr() string {
	return self.uniqushPushAddr
}

func (self *Config) ReadConfig(srv string) *msgcenter.ServiceConfig {
	if ret, ok := self.srvConfig[srv]; ok {
		return ret
	}
	return self.defaultConfig
}

func parseInt(node yaml.Node) (n int, err error) {
	if scalar, ok := node.(yaml.Scalar); ok {
		str := string(scalar)
		n, err = strconv.Atoi(str)
	} else {
		err = fmt.Errorf("Not a scalar")
	}
	return
}

func parseString(node yaml.Node) (str string, err error) {
	if scalar, ok := node.(yaml.Scalar); ok {
		str = string(scalar)
	} else {
		err = fmt.Errorf("Not a scalar")
	}
	return
}

func parseDuration(node yaml.Node) (t time.Duration, err error) {
	if scalar, ok := node.(yaml.Scalar); ok {
		t, err = time.ParseDuration(string(scalar))
	} else {
		err = fmt.Errorf("timeout should be a scalar")
	}
	return
}

func parseMessageHandler(node yaml.Node, timeout time.Duration) (h evthandler.MessageHandler, err error) {
	if scalar, ok := node.(yaml.Scalar); ok {
		h := new(webhook.MessageHandler)
		h.URL = string(scalar)
		h.Timeout = timeout
	} else {
		err = fmt.Errorf("webhook should be a scalar")
	}
	return
}

func parseErrorHandler(node yaml.Node, timeout time.Duration) (h evthandler.ErrorHandler, err error) {
	if scalar, ok := node.(yaml.Scalar); ok {
		h := new(webhook.ErrorHandler)
		h.URL = string(scalar)
		h.Timeout = timeout
	} else {
		err = fmt.Errorf("webhook should be a scalar")
	}
	return
}

func parseForwardRequestHandler(node yaml.Node, timeout time.Duration) (h evthandler.ForwardRequestHandler, err error) {
	if scalar, ok := node.(yaml.Scalar); ok {
		h := new(webhook.ForwardRequestHandler)
		h.URL = string(scalar)
		h.Timeout = timeout
	} else {
		err = fmt.Errorf("webhook should be a scalar")
	}
	return
}

func parseCache(node yaml.Node) (cache msgcache.Cache, err error) {
	if fields, ok := node.(yaml.Map); ok {
		engine := "redis"
		addr := ""
		password := ""
		name := "0"

		for k, v := range fields {
			switch k {
			case "engine":
				engine, err = parseString(v)
			case "addr":
				addr, err = parseString(v)
			case "password":
				password, err = parseString(v)
			case "name":
				name, err = parseString(v)
			}
			if err != nil {
				err = fmt.Errorf("[field=%v] %v", k, err)
				return
			}
		}
		if engine != "redis" {
			err = fmt.Errorf("database %v is not supported", engine)
			return
		}
		db := 0
		db, err = strconv.Atoi(name)
		if err != nil || db < 0 {
			err = fmt.Errorf("invalid database name: %v", name)
			return
		}
		cache = msgcache.NewRedisMessageCache(addr, password, db)
	} else {
		err = fmt.Errorf("database info should be a map")
	}
	return
}

func parseLogoutHandler(node yaml.Node, timeout time.Duration) (h evthandler.LogoutHandler, err error) {
	if scalar, ok := node.(yaml.Scalar); ok {
		h := new(webhook.LogoutHandler)
		h.URL = string(scalar)
		h.Timeout = timeout
	} else {
		err = fmt.Errorf("webhook should be a scalar")
	}
	return
}

func parseLoginHandler(node yaml.Node, timeout time.Duration) (h evthandler.LoginHandler, err error) {
	if scalar, ok := node.(yaml.Scalar); ok {
		h := new(webhook.LoginHandler)
		h.URL = string(scalar)
		h.Timeout = timeout
	} else {
		err = fmt.Errorf("webhook should be a scalar")
	}
	return
}

func parseService(service string, node yaml.Node, defaultConfig *msgcenter.ServiceConfig) (config *msgcenter.ServiceConfig, err error) {
	fields, ok := node.(yaml.Map)
	if !ok {
		err = fmt.Errorf("[service=%v] Service information should be a map", service)
		return
	}
	timeout := 3 * time.Second

	if t, ok := fields["timeout"]; ok {
		timeout, err = parseDuration(t)
		if err != nil {
			err = fmt.Errorf("[service=%v][field=timeout] %v", service, err)
			return
		}
	}

	config = new(msgcenter.ServiceConfig)

	if defaultConfig != nil {
		*config = *defaultConfig
	}

	for name, value := range fields {
		switch name {
		case "msg":
			config.MessageHandler, err = parseMessageHandler(value, timeout)
		case "logout":
			config.LogoutHandler, err = parseLogoutHandler(value, timeout)
		case "login":
			config.LoginHandler, err = parseLoginHandler(value, timeout)
		case "fwd":
			config.ForwardRequestHandler, err = parseForwardRequestHandler(value, timeout)
		case "max_conns":
			config.MaxNrConns, err = parseInt(value)
		case "max_online_users":
			config.MaxNrUsers, err = parseInt(value)
		case "max_conns_per_user":
			config.MaxNrConnsPerUser, err = parseInt(value)
		case "db":
			config.MsgCache, err = parseCache(value)
		case "err":
			config.ErrorHandler, err = parseErrorHandler(value, timeout)
		}
		if err != nil {
			err = fmt.Errorf("[service=%v][field=%v] %v", service, name, err)
			config = nil
			return
		}
	}
	return
}

func Parse(filename string) (config *Config, err error) {
	file, err := yaml.ReadFile(filename)
	if err != nil {
		return
	}
	root := file.Root
	config = new(Config)
	switch t := root.(type) {
	case yaml.Map:
		config.srvConfig = make(map[string]*msgcenter.ServiceConfig, len(t))
		if dc, ok := t["default"]; ok {
			config.defaultConfig, err = parseService("default", dc, nil)
		}
		if err != nil {
			config = nil
			return
		}
		for srv, node := range t {
			switch srv {
			case "uniqush-push":
				fallthrough
			case "uniqush_push":
				config.uniqushPushAddr, err = parseString(node)
				if err != nil {
					err = fmt.Errorf("invalid uniqush-push address: %v", err)
					return
				}
				continue
			}
			var sconf *msgcenter.ServiceConfig
			sconf, err = parseService(srv, node, config.defaultConfig)
			if err != nil {
				config = nil
				return
			}
			config.srvConfig[srv] = sconf
		}
	default:
		err = fmt.Errorf("Top level should be a map")
	}
	return
}
