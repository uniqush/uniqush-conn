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
)

type Conn interface {
	proto.Conn
	Config(digestThreshold, compressThreshold int, encrypt bool, digestFields []string) error
	SetDigestChannel(digestChan chan<- *Digest)
	RequestMessage(id string) error
}

type Digest struct {
	MsgId string
	Size  int
	Info  map[string]string
}

type clientConn struct {
	proto.Conn
	cmdio *proto.CommandIO

	digestChan chan<- *Digest

	digestThreshold   int
	compressThreshold int
	encrypt           bool
}

func (self *clientConn) RequestMessage(id string) error {
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_MSG_RETRIEVE
	cmd.Params = []string{id}
	return self.cmdio.WriteCommand(cmd, false, true)
}

func (self *clientConn) SetDigestChannel(digestChan chan<- *Digest) {
	self.digestChan = digestChan
}

func (self *clientConn) Config(digestThreshold, compressThreshold int, encrypt bool, digestFields []string) error {
	self.digestThreshold = digestThreshold
	self.compressThreshold = compressThreshold
	self.encrypt = encrypt
	cmd := new(proto.Command)
	cmd.Type = proto.CMD_SETTING
	cmd.Params = make([]string, 3, 3+len(digestFields))
	cmd.Params[0] = fmt.Sprintf("%v", digestThreshold)
	cmd.Params[1] = fmt.Sprintf("%v", compressThreshold)
	if encrypt {
		cmd.Params[2] = "1"
	} else {
		cmd.Params[2] = "0"
	}
	for _, f := range digestFields {
		cmd.Params = append(cmd.Params, f)
	}
	err := self.cmdio.WriteCommand(cmd, false, true)
	return err
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
		self.digestChan <- digest
	case proto.CMD_FWD:
		if len(cmd.Params) < 1 {
			err = proto.ErrBadPeerImpl
			return
		}
		msg = new(proto.Message)
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
	return cc
}
