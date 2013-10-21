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
	"fmt"
	"github.com/uniqush/uniqush-conn/config"
	"github.com/uniqush/uniqush-conn/proto/server"
	"github.com/uniqush/uniqush-conn/rpc"
	"net"
	"sync"
)

type MessageCenter struct {
	ln      net.Listener
	privkey *rsa.PrivateKey
	config  *config.Config

	srvCentersLock   sync.Mutex
	serviceCenterMap map[string]*serviceCenter
	fwdChan          chan *rpc.ForwardRequest
	peers            *rpc.MultiPeer
}

func (self *MessageCenter) processForwardRequest() {
	for req := range self.fwdChan {
		if req == nil {
			return
		}
		if len(req.ReceiverService) == 0 {
			continue
		}
		center := self.getServiceCenter(req.ReceiverService)
		if center != nil {
			go center.Forward(req)
		}
	}
}

func NewMessageCenter(ln net.Listener, privkey *rsa.PrivateKey, conf *config.Config) *MessageCenter {
	ret := new(MessageCenter)
	ret.ln = ln
	ret.privkey = privkey
	ret.config = conf

	ret.peers = rpc.NewMultiPeer()
	ret.fwdChan = make(chan *rpc.ForwardRequest)
	ret.serviceCenterMap = make(map[string]*serviceCenter, 10)

	go ret.processForwardRequest()
	return ret
}

func (self *MessageCenter) ServiceNames() []string {
	self.srvCentersLock.Lock()
	defer self.srvCentersLock.Unlock()

	ret := make([]string, 0, len(self.serviceCenterMap))

	for srv, _ := range self.serviceCenterMap {
		ret = append(ret, srv)
	}
	return ret
}

func (self *MessageCenter) getServiceCenter(srv string) *serviceCenter {
	self.srvCentersLock.Lock()
	defer self.srvCentersLock.Unlock()

	center, ok := self.serviceCenterMap[srv]
	if !ok {
		conf := self.config.ReadConfig(srv)
		if conf == nil {
			return nil
		}
		center = newServiceCenter(conf, self.fwdChan, self.peers)
		if center != nil {
			self.serviceCenterMap[srv] = center
		}
	}
	return center
}

func (self *MessageCenter) serveConn(c net.Conn) {
	if tcpConn, ok := c.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
	}
	conn, err := server.AuthConn(c, self.privkey, self.config, self.config.HandshakeTimeout)
	if err != nil {
		if err != server.ErrAuthFail {
			self.config.OnError(c.RemoteAddr(), err)
		}
		c.Close()
		return
	}
	srv := conn.Service()

	center := self.getServiceCenter(srv)
	if center == nil {
		self.config.OnError(c.RemoteAddr(), fmt.Errorf("unknown service: %v", srv))
		c.Close()
		return
	}
	center.NewConn(conn)
	return
}

func (self *MessageCenter) Start() {
	for {
		conn, err := self.ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				// It's a temporary error.
				if ne.Temporary() {
					continue
				}
			}
			self.config.OnError(self.ln.Addr(), err)
			return
		}
		go self.serveConn(conn)
	}
}

// NOTE: you cannot restart it!
func (self *MessageCenter) Stop() {
	self.srvCentersLock.Lock()

	for _, center := range self.serviceCenterMap {
		center.Stop()
	}
	self.ln.Close()
	close(self.fwdChan)
}

func (self *MessageCenter) NrConns(srv string) int {
	center := self.getServiceCenter(srv)
	if center == nil {
		return 0
	}
	return center.NrConns()
}

func (self *MessageCenter) NrUsers(srv string) int {
	center := self.getServiceCenter(srv)
	if center == nil {
		return 0
	}
	return center.NrUsers()
}

func (self *MessageCenter) do(srv string, f func(center *serviceCenter) *rpc.Result) *rpc.Result {
	center := self.getServiceCenter(srv)
	if center == nil {
		ret := new(rpc.Result)
		//ret.SetError(fmt.Errorf("unknown service: %v", srv))
		return ret
	}
	return f(center)
}

func (self *MessageCenter) Send(req *rpc.SendRequest) *rpc.Result {
	return self.do(req.ReceiverService, func(center *serviceCenter) *rpc.Result {
		return center.Send(req)
	})
}

func (self *MessageCenter) Forward(req *rpc.ForwardRequest) *rpc.Result {
	return self.do(req.ReceiverService, func(center *serviceCenter) *rpc.Result {
		return center.Forward(req)
	})
}

func (self *MessageCenter) Redirect(req *rpc.RedirectRequest) *rpc.Result {
	return self.do(req.ReceiverService, func(center *serviceCenter) *rpc.Result {
		return center.Redirect(req)
	})
}

func (self *MessageCenter) CheckUserStatus(req *rpc.UserStatusQuery) *rpc.Result {
	return self.do(req.Service, func(center *serviceCenter) *rpc.Result {
		return center.CheckUserStatus(req)
	})
}

func (self *MessageCenter) AddPeer(peer rpc.UniqushConnPeer) {
	self.peers.AddPeer(peer)
}
