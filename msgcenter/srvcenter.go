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
	"fmt"
	"github.com/uniqush/uniqush-conn/config"
	"github.com/uniqush/uniqush-conn/proto/server"
	"github.com/uniqush/uniqush-conn/rpc"
	"io"
	"strings"
	"time"
)

type serviceCenter struct {
	config     *config.ServiceConfig
	fwdChan    chan<- *rpc.ForwardRequest
	subReqChan chan<- *server.SubscribeRequest
	conns      connMap
}

func (self *serviceCenter) serveConn(conn server.Conn) {
	var reason error
	defer func() {
		self.conns.DelConn(conn)
		self.config.OnLogout(conn, reason)
		conn.Close()
	}()
	for {
		msg, err := conn.ReceiveMessage()
		if err != nil {
			if err != io.EOF {
				self.config.OnError(conn, err)
				reason = err
			}
		}
		if msg != nil {
			self.config.OnMessage(conn, msg)
		}
	}
}

func (self *serviceCenter) NewConn(conn server.Conn) {
	if conn == nil {
		//self.config.OnError(conn, fmt.Errorf("Nil conn")
		return
	}
	usr := conn.Username()
	if len(usr) == 0 || strings.Contains(usr, ":") || strings.Contains(usr, "\n") {
		self.config.OnError(conn, fmt.Errorf("invalid username"))
		conn.Close()
		return
	}
	conn.SetMessageCache(self.config.Cache())
	conn.SetForwardRequestChannel(self.fwdChan)
	conn.SetSubscribeRequestChan(self.subReqChan)
	err := self.conns.AddConn(conn)
	if err != nil {
		self.config.OnError(conn, err)
		conn.Close()
		return
	}

	go self.serveConn(conn)
	return
}

func (self *serviceCenter) Send(callId string, username string, msg *rpc.Message, ttl time.Duration, infoForPush map[string]string) *rpc.Result {
	conns := self.conns.GetConn(username)
	ret := new(rpc.Result)
	ret.CallID = callId
	var mid string
	mc := &rpc.MessageContainer{
		Sender:        "",
		SenderService: "",
		Message:       msg,
	}
	mid, ret.Error = self.config.CacheMessage(username, mc, ttl)
	if ret.Error != nil {
		return ret
	}

	n := 0

	for _, minc := range conns {
		if conn, ok := minc.(server.Conn); ok {
			err := conn.SendMessage(msg, mid, nil)
			ret.Append(conn, err)
			if err != nil {
				conn.Close()
			} else {
				n++
			}
		} else {
			self.conns.DelConn(minc)
		}
	}

	if n == 0 {
		self.config.Push(username, "", "", infoForPush, mid, msg.Size())
	}
	return ret
}

func newServiceCenter(conf *config.ServiceConfig, fwdChan chan<- *rpc.ForwardRequest, subReqChan chan<- *server.SubscribeRequest) *serviceCenter {
	if conf == nil || fwdChan == nil || subReqChan == nil {
		return nil
	}
	ret := new(serviceCenter)
	ret.config = conf
	ret.conns = newTreeBasedConnMap(conf.MaxNrConns, conf.MaxNrUsers, conf.MaxNrConnsPerUser)
	ret.fwdChan = fwdChan
	ret.subReqChan = subReqChan
	return ret
}
