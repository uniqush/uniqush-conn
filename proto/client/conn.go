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
	ReceiveMessage() (mc *proto.MessageContainer, err error)

	Config(digestThreshold, compressThreshold int, digestFields ...string) error
	SetDigestChannel(digestChan chan<- *Digest)
	RequestMessage(id string) error
	SetVisibility(v bool) error
}

type CommandProcessor interface {
	ProcessCommand(cmd *proto.Command) (mc *proto.MessageContainer, err error)
}

type clientConn struct {
	cmdio             *proto.CommandIO
	conn              net.Conn
	compressThreshold int32
	digestThreshold   int32
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

func (self *clientConn) processCommand(cmd *proto.Command) (mc *proto.MessageContainer, err error) {
	if cmd == nil {
		return
	}

	t := int(cmd.Type)
	if t > len(self.cmdProcs) {
		return
	}
	proc := self.cmdProcs[t]
	if proc != nil {
		mc, err = proc.ProcessCommand(cmd)
	}
	return
}

func (self *clientConn) ReceiveMessage() (mc *proto.MessageContainer, err error) {
	var cmd *proto.Command
	for {
		cmd, err = self.cmdio.ReadCommand()
		if err != nil {
			return
		}
		switch cmd.Type {
		case proto.CMD_DATA:
			mc = new(proto.MessageContainer)
			mc.Message = cmd.Message
			if len(cmd.Params[0]) > 0 {
				mc.Id = cmd.Params[0]
			}
			return
		case proto.CMD_FWD:
			if len(cmd.Params) < 1 {
				err = proto.ErrBadPeerImpl
				return
			}
			mc = new(proto.MessageContainer)
			mc.Message = cmd.Message
			mc.Sender = cmd.Params[0]
			if len(cmd.Params) > 1 {
				mc.SenderService = cmd.Params[1]
			} else {
				mc.SenderService = self.Service()
			}
			if len(cmd.Params) > 2 {
				mc.Id = cmd.Params[2]
			}
			return
		case proto.CMD_BYE:
			err = io.EOF
			return
		default:
			mc, err = self.processCommand(cmd)
			if err != nil || mc != nil {
				return
			}
		}
	}
	return
}

func (self *clientConn) setCommandProcessor(cmdType uint8, proc CommandProcessor) {
	if cmdType >= proto.CMD_NR_CMDS {
		return
	}
	if len(self.cmdProcs) <= int(cmdType) {
		self.cmdProcs = make([]CommandProcessor, proto.CMD_NR_CMDS)
	}
	self.cmdProcs[cmdType] = proc
}

func (self *clientConn) SetDigestChannel(digestChan chan<- *Digest) {
	proc := new(digestProcessor)
	proc.digestChan = digestChan
	proc.service = self.Service()
	self.setCommandProcessor(proto.CMD_DIGEST, proc)
}

func (self *clientConn) Config(digestThreshold, compressThreshold int, digestFields ...string) error {
	self.digestThreshold = int32(digestThreshold)
	self.compressThreshold = int32(compressThreshold)
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_SETTING
	cmd.Params = make([]string, 2, 2+len(digestFields))
	cmd.Params[0] = fmt.Sprintf("%v", self.digestThreshold)
	cmd.Params[1] = fmt.Sprintf("%v", self.compressThreshold)
	for _, f := range digestFields {
		cmd.Params = append(cmd.Params, f)
	}
	err := self.cmdio.WriteCommand(cmd, false)
	return err
}

func (self *clientConn) RequestMessage(id string) error {
	cmd := &proto.Command{
		Type:   proto.CMD_MSG_RETRIEVE,
		Params: []string{id},
	}
	return self.cmdio.WriteCommand(cmd, false)
}

func (self *clientConn) SetVisibility(v bool) error {
	cmd := &proto.Command{
		Type: proto.CMD_SET_VISIBILITY,
	}
	if v {
		cmd.Params = []string{"1"}
	} else {
		cmd.Params = []string{"0"}
	}
	return self.cmdio.WriteCommand(cmd, false)
}

func NewConn(cmdio *proto.CommandIO, service, username string, conn net.Conn) Conn {
	ret := new(clientConn)
	ret.conn = conn
	ret.cmdio = cmdio
	ret.service = service
	ret.username = username
	ret.connId = fmt.Sprintf("%x-%x", time.Now().UnixNano(), rand.Int63())

	ret.cmdProcs = make([]CommandProcessor, proto.CMD_NR_CMDS)
	return ret
}
