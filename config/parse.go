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
	"github.com/kylelemons/go-gypsy/yaml"
	"github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/evthandler/webhook"
	"github.com/uniqush/uniqush-conn/msgcache"
	"io"

	"github.com/uniqush/uniqush-conn/push"
	"net"
	"strconv"
	"time"
)

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
	if node == nil {
		str = ""
		return
	}
	if scalar, ok := node.(yaml.Scalar); ok {
		str = string(scalar)
	} else {
		err = fmt.Errorf("Not a scalar")
	}
	return
}

func parseStringOrList(node yaml.Node) (str []string, err error) {
	str = make([]string, 0, 2)
	if l, ok := node.(yaml.List); ok {
		var s string
		for _, n := range l {
			s, err = parseString(n)
			if err != nil {
				return
			}
			str = append(str, s)
		}
	} else {
		var s string
		s, err = parseString(node)
		if err != nil {
			return
		}
		str = append(str, s)
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

type webhookInfo struct {
	urls         []string
	timeout      time.Duration
	defaultValue string
}

func parseWebHook(node yaml.Node) (hook *webhookInfo, err error) {
	if kv, ok := node.(yaml.Map); ok {
		hook = new(webhookInfo)
		if url, ok := kv["url"]; ok {
			hook.urls, err = parseStringOrList(url)
			if err != nil {
				err = fmt.Errorf("webhook's url should be a string or a list of string")
				return
			}
		} else {
			err = fmt.Errorf("webhook should have url")
			return
		}
		if timeout, ok := kv["timeout"]; ok {
			hook.timeout, err = parseDuration(timeout)
			if err != nil {
				err = fmt.Errorf("timeout error: %v", err)
				return
			}
		}
		if defaultValue, ok := kv["default"]; ok {
			hook.defaultValue, err = parseString(defaultValue)
			if err != nil {
				err = fmt.Errorf("webhook's default value should be a string")
				return
			}
		}
	} else {
		err = fmt.Errorf("webhook should be a map")
	}
	return
}

func setWebHook(hd webhook.WebHook, node yaml.Node, timeout time.Duration) error {
	hook, err := parseWebHook(node)
	if err != nil {
		return err
	}
	if hook.timeout < 0*time.Second {
		hook.timeout = timeout
	}
	hd.SetTimeout(hook.timeout)
	hd.SetURL(hook.urls...)
	if hook.defaultValue == "allow" {
		hd.SetDefault(200)
	} else {
		hd.SetDefault(404)
	}
	return nil
}

func parseAuthHandler(node yaml.Node, timeout time.Duration) (h evthandler.Authenticator, err error) {
	hd := new(webhook.AuthHandler)
	err = setWebHook(hd, node, timeout)
	if err != nil {
		return
	}
	h = hd
	return
}

func parseMessageHandler(node yaml.Node, timeout time.Duration) (h evthandler.MessageHandler, err error) {
	hd := new(webhook.MessageHandler)
	err = setWebHook(hd, node, timeout)
	if err != nil {
		return
	}
	h = hd
	return
}

func parseErrorHandler(node yaml.Node, timeout time.Duration) (h evthandler.ErrorHandler, err error) {
	hd := new(webhook.ErrorHandler)
	err = setWebHook(hd, node, timeout)
	if err != nil {
		return
	}
	h = hd
	return
}

func parseForwardRequestHandler(node yaml.Node, timeout time.Duration) (h evthandler.ForwardRequestHandler, err error) {
	hd := new(webhook.ForwardRequestHandler)
	err = setWebHook(hd, node, timeout)
	if err != nil {
		return
	}
	if kv, ok := node.(yaml.Map); ok {
		if ttlnode, ok := kv["max-ttl"]; ok {
			ttl, e := parseDuration(ttlnode)
			if e != nil {
				err = fmt.Errorf("max-ttl: %v", e)
				return
			}
			hd.SetMaxTTL(ttl)
		} else {
			hd.SetMaxTTL(24 * time.Hour)
		}
	}
	h = hd
	return
}

func parseLogoutHandler(node yaml.Node, timeout time.Duration) (h evthandler.LogoutHandler, err error) {
	hd := new(webhook.LogoutHandler)
	err = setWebHook(hd, node, timeout)
	if err != nil {
		return
	}
	h = hd
	return
}

func parseLoginHandler(node yaml.Node, timeout time.Duration) (h evthandler.LoginHandler, err error) {
	hd := new(webhook.LoginHandler)
	err = setWebHook(hd, node, timeout)
	if err != nil {
		return
	}
	h = hd
	return
}

func parseSubscribeHandler(node yaml.Node, timeout time.Duration) (h evthandler.SubscribeHandler, err error) {
	hd := new(webhook.SubscribeHandler)
	err = setWebHook(hd, node, timeout)
	if err != nil {
		return
	}
	h = hd
	return
}

func parseUnsubscribeHandler(node yaml.Node, timeout time.Duration) (h evthandler.UnsubscribeHandler, err error) {
	hd := new(webhook.UnsubscribeHandler)
	err = setWebHook(hd, node, timeout)
	if err != nil {
		return
	}
	h = hd
	return
}

func parseUniqushPush(node yaml.Node, timeout time.Duration) (p push.Push, err error) {
	kv, ok := node.(yaml.Map)
	if !ok {
		err = fmt.Errorf("uniqush-push information should be a map")
		return
	}
	addrN, ok := kv["addr"]
	if !ok {
		err = fmt.Errorf("cannot find addr field")
		return
	}
	addr, err := parseString(addrN)
	if !ok {
		err = fmt.Errorf("address error: %v", err)
		return
	}
	_, err = net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		err = fmt.Errorf("bad addres: %v", err)
		return
	}
	if to, ok := kv["timeout"]; ok {
		timeout, err = parseDuration(to)
		if err != nil {
			err = fmt.Errorf("bad timeout: %v", err)
			return
		}
	}
	p = push.NewUniqushPushClient(addr, timeout)
	return
}

func parseCache(node yaml.Node) (cache msgcache.Cache, err error) {
	if fields, ok := node.(yaml.Map); ok {
		engine := "redis"
		addr := ""
		password := ""
		username := ""
		database := "0"
		port := 0

		for k, v := range fields {
			switch k {
			case "engine":
				engine, err = parseString(v)
			case "host":
				addr, err = parseString(v)
			case "port":
				port, err = parseInt(v)
			case "username":
				username, err = parseString(v)
			case "password":
				password, err = parseString(v)
			case "database":
				database, err = parseString(v)
			}
			if err != nil {
				err = fmt.Errorf("[field=%v] %v", k, err)
				return
			}
		}
		cache, err = msgcache.GetCache(engine, addr, username, password, database, port)
	} else {
		err = fmt.Errorf("database info should be a map")
	}
	return
}

func parseService(service string, node yaml.Node, defaultConfig *ServiceConfig) (config *ServiceConfig, err error) {
	config = defaultConfig.clone(service, config)
	if node == nil {
		return
	}
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
		case "subscribe":
			config.SubscribeHandler, err = parseSubscribeHandler(value, timeout)
		case "unsubscribe":
			config.UnsubscribeHandler, err = parseUnsubscribeHandler(value, timeout)
		case "uniqush-push":
			fallthrough
		case "uniqush_push":
			config.PushService, err = parseUniqushPush(value, timeout)
		case "max-conns":
			fallthrough
		case "max_conns":
			config.MaxNrConns, err = parseInt(value)
		case "max-online-users":
			fallthrough
		case "max_online_users":
			config.MaxNrUsers, err = parseInt(value)
		case "max-conns-per-user":
			fallthrough
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

func checkConfig(config *Config) error {
	if config.Auth == nil {
		return fmt.Errorf("No authentication url")
	}
	return nil
}

func ParseFile(filename string) (config *Config, err error) {
	file, err := yaml.ReadFile(filename)
	if err != nil {
		return
	}
	config, err = parseConfigRootNode(file.Root)
	if err != nil {
		return
	}
	config.filename = filename
	return
}
func Parse(reader io.Reader) (config *Config, err error) {
	root, err := yaml.Parse(reader)
	if err != nil {
		return
	}
	return parseConfigRootNode(root)
}

func parseConfigRootNode(root yaml.Node) (config *Config, err error) {
	config = new(Config)
	config.filename = ""
	switch t := root.(type) {
	case yaml.Map:
		config.srvConfig = make(map[string]*ServiceConfig, len(t))
		if dc, ok := t["default"]; ok {
			config.defaultConfig, err = parseService("default", dc, nil)
		}
		if err != nil {
			config = nil
			return
		}
		for srv, node := range t {
			switch srv {
			case "auth":
				config.Auth, err = parseAuthHandler(node, 3*time.Second)
				if err != nil {
					err = fmt.Errorf("auth: %v", err)
					return
				}
				continue
			case "err":
				config.ErrorHandler, err = parseErrorHandler(node, 3*time.Second)
				if err != nil {
					err = fmt.Errorf("global error handler: %v", err)
					return
				}
				continue
			case "http-addr":
				fallthrough
			case "http_addr":
				config.HttpAddr, err = parseString(node)
				if err != nil {
					err = fmt.Errorf("Bad HTTP bind address: %v", err)
					return
				}
				continue
			case "handshake-timeout":
				fallthrough
			case "handshake_timeout":
				config.HandshakeTimeout, err = parseDuration(node)
				if err != nil {
					err = fmt.Errorf("bad handshake timeout: %v", err)
					return
				}
				continue
			case "default":
				// Don't need to parse the default service again.
				continue
			}
			var sconf *ServiceConfig
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
	if err == nil {
		err = checkConfig(config)
	}
	return
}
