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

package msgcenter

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/server"
	"net"
	"strings"
	"sync"
	"time"
)

var ErrNoService = errors.New("invalid service")

type ServiceConfigReader interface {
	ReadConfig(srv string) *ServiceConfig
}

type MessageCenter struct {
	srvCentersLock   sync.Mutex
	serviceCenterMap map[string]*serviceCenter

	ln            net.Listener
	auth          server.Authenticator
	authtimeout   time.Duration
	fwdChan       chan *server.ForwardRequest
	privkey       *rsa.PrivateKey
	errHandler    evthandler.ErrorHandler
	srvConfReader ServiceConfigReader
}

func (self *MessageCenter) reportError(service, username, connId, addr string, err error) {
	if self.errHandler != nil {
		go self.errHandler.OnError(service, username, connId, addr, err)
	}
}

func (self *MessageCenter) process() {
	for {
		select {
		case fwdreq := <-self.fwdChan:
			srv := fwdreq.ReceiverService
			self.srvCentersLock.Lock()
			center, ok := self.serviceCenterMap[srv]
			self.srvCentersLock.Unlock()
			if !ok {
				continue
			}
			center.ReceiveForward(fwdreq)
		}
	}
}

func (self *MessageCenter) AddService(srv string) *serviceCenter {
	self.srvCentersLock.Lock()
	defer self.srvCentersLock.Unlock()
	config := self.srvConfReader.ReadConfig(srv)
	if config == nil {
		self.reportError(srv, "", "", "", fmt.Errorf("cannot find service's config"))
		return nil
	}
	center := newServiceCenter(srv, config, self.fwdChan)
	self.serviceCenterMap[srv] = center
	return center
}

func (self *MessageCenter) serveConn(c net.Conn) {
	conn, err := server.AuthConn(c, self.privkey, self.auth, self.authtimeout)
	if err != nil {
		self.reportError("", "", "", c.RemoteAddr().String(), err)
		c.Close()
		return
	}
	srv := conn.Service()
	if len(srv) == 0 || strings.Contains(srv, ":") || strings.Contains(srv, "\n") {
		self.reportError(srv, "", "", c.RemoteAddr().String(), fmt.Errorf("bad service name"))
		return
	}

	self.srvCentersLock.Lock()
	center, ok := self.serviceCenterMap[srv]
	if !ok {
		config := self.srvConfReader.ReadConfig(srv)
		if config == nil {
			self.reportError(srv, "", "", c.RemoteAddr().String(), fmt.Errorf("cannot find service's config"))
			self.srvCentersLock.Unlock()
			return
		}
		center = newServiceCenter(srv, config, self.fwdChan)
		self.serviceCenterMap[srv] = center
	}
	self.srvCentersLock.Unlock()

	err = center.NewConn(conn)
	if err != nil {
		self.reportError(srv, conn.Username(), "", c.RemoteAddr().String(), err)
	}
}

func (self *MessageCenter) SendMessage(service, username string, msg *proto.Message, extra map[string]string, ttl time.Duration) []*Result {
	if len(username) == 0 || strings.Contains(username, ":") || strings.Contains(username, "\n") {
		res := []*Result{&Result{fmt.Errorf("[Service=%v] bad username", username), "", false}}
		return res
	}
	self.srvCentersLock.Lock()
	center, ok := self.serviceCenterMap[service]
	self.srvCentersLock.Unlock()

	if !ok {
		return nil
	}
	return center.SendMessage(username, msg, extra, ttl)
}

func (self *MessageCenter) Start() {
	go self.process()
	for {
		conn, err := self.ln.Accept()
		if err != nil {
			self.reportError("", "", "", self.ln.Addr().String(), err)
			continue
		}
		go self.serveConn(conn)
	}
}

func NewMessageCenter(ln net.Listener,
	privkey *rsa.PrivateKey,
	errHandler evthandler.ErrorHandler,
	authtimeout time.Duration,
	auth server.Authenticator,
	srvConfReader ServiceConfigReader) *MessageCenter {

	self := new(MessageCenter)
	self.ln = ln
	self.auth = auth
	self.authtimeout = authtimeout
	self.fwdChan = make(chan *server.ForwardRequest)
	self.privkey = privkey
	self.errHandler = errHandler
	self.srvConfReader = srvConfReader
	self.serviceCenterMap = make(map[string]*serviceCenter, 128)
	return self
}
