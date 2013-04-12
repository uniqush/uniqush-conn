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
	msgChan       chan<- *proto.Message
	fwdChan       chan<- *server.ForwardRequest
	connErrChan   chan<- *EventConnError
	errChan       chan<- error
	privkey       *rsa.PrivateKey
	srvConfReader ServiceConfigReader
}

func (self *MessageCenter) serveConn(c net.Conn) {
	conn, err := server.AuthConn(c, self.privkey, self.auth, self.authtimeout)
	if err != nil {
		self.errChan <- fmt.Errorf("[Addr=%v] %v", c.RemoteAddr(), err)
		c.Close()
	}
	srv := conn.Service()
	if len(srv) == 0 || strings.Contains(srv, ":") || strings.Contains(srv, "\n") {
		self.errChan <- fmt.Errorf("[Service=%v] bad service name", srv)
		return
	}

	self.srvCentersLock.Lock()
	center, ok := self.serviceCenterMap[srv]
	if !ok {
		config := self.srvConfReader.ReadConfig(srv)
		if config == nil {
			self.errChan <- fmt.Errorf("[Service=%v] Cannot find its config info", srv)
			self.srvCentersLock.Unlock()
			return
		}
		center = newServiceCenter(srv, config, self.msgChan, self.fwdChan, self.connErrChan)
		self.serviceCenterMap[srv] = center
	}
	self.srvCentersLock.Unlock()

	err = center.NewConn(conn)
	if err != nil {
		self.errChan <- fmt.Errorf("[Service=%v] %v", srv, err)
	}
}

func (self *MessageCenter) SendOrBox(service, username string, msg *proto.Message, extra map[string]string, timeout time.Duration) (n int, err error) {
	if len(username) == 0 || strings.Contains(username, ":") || strings.Contains(username, "\n") {
		err = fmt.Errorf("[Service=%v] bad username", username)
		return
	}
	self.srvCentersLock.Lock()
	center, ok := self.serviceCenterMap[service]
	self.srvCentersLock.Unlock()

	if !ok {
		n = 0
		return
	}
	n, err = center.SendOrBox(username, msg, extra, timeout)
	return
}

func (self *MessageCenter) SendOrQueue(service, username string, msg *proto.Message, extra map[string]string) (n int, err error) {
	if len(username) == 0 || strings.Contains(username, ":") || strings.Contains(username, "\n") {
		err = fmt.Errorf("[Service=%v] bad username", username)
		return
	}
	self.srvCentersLock.Lock()
	center, ok := self.serviceCenterMap[service]
	self.srvCentersLock.Unlock()

	if !ok {
		n = 0
		return
	}
	n, err = center.SendOrQueue(username, msg, extra)
	return
}

func (self *MessageCenter) Start() {
	for {
		conn, err := self.ln.Accept()
		if err != nil {
			self.errChan <- err
			continue
		}
		go self.serveConn(conn)
	}
}

func NewMessageCenter(ln net.Listener,
	privkey *rsa.PrivateKey,
	msgChan chan<- *proto.Message,
	fwdChan chan<- *server.ForwardRequest,
	connErrChan chan<- *EventConnError,
	errChan chan<- error,
	authtimeout time.Duration,
	auth server.Authenticator,
	srvConfReader ServiceConfigReader) *MessageCenter {

	self := new(MessageCenter)
	self.ln = ln
	self.auth = auth
	self.authtimeout = authtimeout
	self.msgChan = msgChan
	self.fwdChan = fwdChan
	self.connErrChan = connErrChan
	self.errChan = errChan
	self.privkey = privkey
	self.srvConfReader = srvConfReader

	return self
}
