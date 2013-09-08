/*
 * Copyright 2012 Nan Deng
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

package client

import (
	"fmt"
	"github.com/uniqush/uniqush-conn/proto"
	"io"
	"math/rand"
	"net"
	"sync/atomic"
	"time"
)

type Conn interface {
	Close() error
	Service() string
	Username() string
	UniqId() string

	SendMessageToUser(receiver, service string, msg *proto.Message, ttl time.Duration) error
	SendMessageToServer(msg *proto.Message) error
	ReceiveMessage() (msg *proto.Message, err error)
}

type CommandProcessor interface {
	ProcessCommand(cmd *proto.Command) (msg *proto.Message, err error)
}

type clientConn struct {
	cmdio             *proto.CommandIO
	conn              net.Conn
	compressThreshold int32
	service           string
	username          string
	connId            string
	cmdProcs          []CommandProcessor
}

func (self *clientConn) Service() string {
	return self.service
}

func (self *clientConn) Username() string {
	return self.username
}

func (self *clientConn) UniqId() string {
	return self.connId
}

func (self *clientConn) Close() error {
	return self.conn.Close()
}

func (self *clientConn) shouldCompress(size int) bool {
	t := int(atomic.LoadInt32(&self.compressThreshold))
	if t > 0 && t < size {
		return true
	}
	return false
}

func (self *clientConn) SendMessageToServer(msg *proto.Message) error {
	compress := self.shouldCompress(msg.Size())

	cmd := new(proto.Command)
	cmd.Message = msg
	cmd.Type = proto.CMD_DATA
	err := self.cmdio.WriteCommand(cmd, compress)
	return err
}

func (self *clientConn) SendMessageToUser(receiver, service string, msg *proto.Message, ttl time.Duration) error {
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_FWD_REQ
	cmd.Params = make([]string, 2, 3)
	cmd.Params[0] = fmt.Sprintf("%v", ttl)
	cmd.Params[1] = receiver
	if len(service) > 0 && service != self.Service() {
		cmd.Params = append(cmd.Params, service)
	}
	cmd.Message = msg
	compress := self.shouldCompress(msg.Size())
	return self.cmdio.WriteCommand(cmd, compress)
}

func (self *clientConn) processCommand(cmd *proto.Command) (msg *proto.Message, err error) {
	if cmd == nil {
		return
	}

	t := int(cmd.Type)
	if t > len(self.cmdProcs) {
		return
	}
	proc := self.cmdProcs[t]
	if proc != nil {
		msg, err = proc.ProcessCommand(cmd)
	}
	return
}

func (self *clientConn) ReceiveMessage() (msg *proto.Message, err error) {
	cmd, err := self.cmdio.ReadCommand()
	if err != nil {
		return
	}
	for {
		switch cmd.Type {
		case proto.CMD_DATA:
			msg = cmd.Message
			return
		case proto.CMD_BYE:
			err = io.EOF
			return
		default:
			msg, err = self.processCommand(cmd)
			if err != nil || msg != nil {
				return
			}
		}
	}
	return
}

func NewConn(cmdio *proto.CommandIO, service, username string, conn net.Conn) Conn {
	ret := new(clientConn)
	ret.conn = conn
	ret.cmdio = cmdio
	ret.service = service
	ret.username = username
	ret.connId = fmt.Sprintf("%x-%x", time.Now().UnixNano(), rand.Int63())
	return ret
}
