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
	"net"
	"strconv"
	"time"
)

type Conn interface {
	proto.Conn
	Config(digestThreshold, compressThreshold int, digestFields []string) error
	SetDigestChannel(digestChan chan<- *Digest)
	RequestMessage(id string) error
	RequestAllCachedMessages(excludes ...string) error
	ForwardRequest(receiver, service string, msg *proto.Message, ttl time.Duration) error
	SetVisibility(v bool) error
	SendMessage(msg *proto.Message) error
}

type Digest struct {
	MsgId         string
	Sender        string
	SenderService string
	Size          int
	Info          map[string]string
}

type clientConn struct {
	proto.Conn
	cmdio *proto.CommandIO

	digestChan chan<- *Digest

	digestThreshold   int
	compressThreshold int
}

func (self *clientConn) SetVisibility(v bool) error {
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_SET_VISIBILITY
	if v {
		cmd.Params = []string{"1"}
	} else {
		cmd.Params = []string{"0"}
	}
	return self.cmdio.WriteCommand(cmd, false)
}

func (self *clientConn) subscribe(params map[string]string, sub bool) error {
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_SUBSCRIPTION
	if sub {
		cmd.Params = []string{"1"}
	} else {
		cmd.Params = []string{"0"}
	}
	cmd.Message = new(proto.Message)
	cmd.Message.Header = params
	return self.cmdio.WriteCommand(cmd, false)
}

func (self *clientConn) Subscribe(params map[string]string) error {
	return self.subscribe(params, true)
}

func (self *clientConn) Unsubscribe(params map[string]string) error {
	return self.subscribe(params, false)
}

func (self *clientConn) RequestAllCachedMessages(excludes ...string) error {
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_REQ_ALL_CACHED
	if len(excludes) > 0 {
		msg := new(proto.Message)
		data := make([]byte, 0, len(excludes)*90)
		for _, i := range excludes {
			data = append(data, []byte(i)...)
			data = append(data, byte(0))
		}
		msg.Body = data
		cmd.Message = msg
	}
	return self.cmdio.WriteCommand(cmd, false)
}

func (self *clientConn) RequestMessage(id string) error {
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_MSG_RETRIEVE
	cmd.Params = []string{id}
	return self.cmdio.WriteCommand(cmd, false)
}

func (self *clientConn) SetDigestChannel(digestChan chan<- *Digest) {
	self.digestChan = digestChan
}

func (self *clientConn) Config(digestThreshold, compressThreshold int, digestFields []string) error {
	self.digestThreshold = digestThreshold
	self.compressThreshold = compressThreshold
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_SETTING
	cmd.Params = make([]string, 2, 2+len(digestFields))
	cmd.Params[0] = fmt.Sprintf("%v", digestThreshold)
	cmd.Params[1] = fmt.Sprintf("%v", compressThreshold)
	for _, f := range digestFields {
		cmd.Params = append(cmd.Params, f)
	}
	err := self.cmdio.WriteCommand(cmd, false)
	return err
}

func (self *clientConn) SendMessage(msg *proto.Message) error {
	sz := msg.Size()
	compress := false
	if self.compressThreshold > 0 && self.compressThreshold < sz {
		compress = true
	}
	return self.WriteMessage(msg, compress)
}

func (self *clientConn) ForwardRequest(receiver, service string, msg *proto.Message, ttl time.Duration) error {
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_FWD_REQ
	cmd.Params = make([]string, 2, 3)
	cmd.Params[0] = fmt.Sprintf("%v", ttl)
	cmd.Params[1] = receiver
	if len(service) > 0 && service != self.Service() {
		cmd.Params = append(cmd.Params, service)
	}
	cmd.Message = msg
	sz := msg.Size()
	compress := false
	if self.compressThreshold > 0 && self.compressThreshold < sz {
		compress = true
	}
	return self.cmdio.WriteCommand(cmd, compress)
}

func (self *clientConn) ProcessCommand(cmd *proto.Command) (msg *proto.Message, err error) {
	if cmd == nil {
		return
	}
	switch cmd.Type {
	case proto.CMD_DIGEST:
		if self.digestChan == nil {
			return
		}
		if len(cmd.Params) < 2 {
			err = proto.ErrBadPeerImpl
			return
		}
		digest := new(Digest)
		digest.Size, err = strconv.Atoi(cmd.Params[0])
		if err != nil {
			err = proto.ErrBadPeerImpl
			return
		}
		digest.MsgId = cmd.Params[1]
		if cmd.Message != nil {
			digest.Info = cmd.Message.Header
		}
		if len(cmd.Params) > 2 {
			digest.Sender = cmd.Params[2]
			if len(cmd.Params) > 3 {
				digest.SenderService = cmd.Params[3]
			} else {
				digest.SenderService = self.Service()
			}
		}
		self.digestChan <- digest
	case proto.CMD_FWD:
		if len(cmd.Params) < 1 {
			err = proto.ErrBadPeerImpl
			return
		}
		msg = cmd.Message
		if msg == nil {
			msg = new(proto.Message)
		}
		msg.Sender = cmd.Params[0]
		if len(cmd.Params) > 1 {
			msg.SenderService = cmd.Params[1]
		} else {
			msg.SenderService = self.Service()
		}
		if len(cmd.Params) > 2 {
			msg.Id = cmd.Params[2]
		}
	}
	return
}

func NewConn(cmdio *proto.CommandIO, service, username string, conn net.Conn) Conn {
	cc := new(clientConn)
	cc.cmdio = cmdio
	cc.Conn = proto.NewConn(cmdio, service, username, conn, cc)
	cc.compressThreshold = 512
	return cc
}
