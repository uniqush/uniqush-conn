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
	"github.com/uniqush/uniqush-conn/rpc"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

type Conn interface {
	Close() error
	Service() string
	Username() string
	UniqId() string

	SendMessageToUsers(msg *rpc.Message, ttl time.Duration, service string, receiver ...string) error
	SendMessageToServer(msg *rpc.Message) error
	ReceiveMessage() (mc *rpc.MessageContainer, err error)

	Config(digestThreshold, compressThreshold int, digestFields ...string) error
	SetDigestChannel(digestChan chan<- *Digest)
	RequestMessage(id string) error
	SetVisibility(v bool) error
	Subscribe(params map[string]string) error
	Unsubscribe(params map[string]string) error
	RequestAllCachedMessages(since time.Time) error
}

type CommandProcessor interface {
	ProcessCommand(cmd *proto.Command) (mc *rpc.MessageContainer, err error)
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

func (self *clientConn) SendMessageToServer(msg *rpc.Message) error {
	compress := self.shouldCompress(msg.Size())

	cmd := &proto.Command{}
	cmd.Message = msg
	cmd.Type = proto.CMD_DATA
	err := self.cmdio.WriteCommand(cmd, compress)
	return err
}

func (self *clientConn) SendMessageToUsers(msg *rpc.Message, ttl time.Duration, service string, receiver ...string) error {
	if len(receiver) == 0 {
		return nil
	}
	cmd := &proto.Command{}
	cmd.Type = proto.CMD_FWD_REQ
	cmd.Params = make([]string, 2, 3)
	cmd.Params[0] = fmt.Sprintf("%v", ttl)
	cmd.Params[1] = strings.Join(receiver, ",")
	if len(service) > 0 && service != self.Service() {
		cmd.Params = append(cmd.Params, service)
	}
	cmd.Message = msg
	compress := self.shouldCompress(msg.Size())
	return self.cmdio.WriteCommand(cmd, compress)
}

func (self *clientConn) processCommand(cmd *proto.Command) (mc *rpc.MessageContainer, err error) {
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

func (self *clientConn) ReceiveMessage() (mc *rpc.MessageContainer, err error) {
	var cmd *proto.Command
	for {
		cmd, err = self.cmdio.ReadCommand()
		if err != nil {
			return
		}
		switch cmd.Type {
		case proto.CMD_DATA:
			mc = new(rpc.MessageContainer)
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
			mc = new(rpc.MessageContainer)
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
	proc := &digestProcessor{}
	proc.digestChan = digestChan
	proc.service = self.Service()
	self.setCommandProcessor(proto.CMD_DIGEST, proc)
}

func (self *clientConn) Config(digestThreshold, compressThreshold int, digestFields ...string) error {
	self.digestThreshold = int32(digestThreshold)
	self.compressThreshold = int32(compressThreshold)
	cmd := &proto.Command{}
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

func (self *clientConn) subscribe(params map[string]string, sub bool) error {
	cmd := &proto.Command{}
	cmd.Type = proto.CMD_SUBSCRIPTION
	if sub {
		cmd.Params = []string{"1"}
	} else {
		cmd.Params = []string{"0"}
	}
	cmd.Message = &rpc.Message{}
	cmd.Message.Header = params
	return self.cmdio.WriteCommand(cmd, false)
}

func (self *clientConn) Subscribe(params map[string]string) error {
	return self.subscribe(params, true)
}

func (self *clientConn) Unsubscribe(params map[string]string) error {
	return self.subscribe(params, false)
}

func (self *clientConn) RequestAllCachedMessages(since time.Time) error {
	cmd := &proto.Command{}
	cmd.Type = proto.CMD_REQ_ALL_CACHED
	if !since.IsZero() {
		cmd.Params = []string{fmt.Sprintf("%v", since.Unix())}
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
