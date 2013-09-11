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

package server

import (
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/rpc"
)

type subscribeProcessor struct {
	conn    *serverConn
	subChan chan<- *rpc.SubscribeRequest
}

func (self *subscribeProcessor) ProcessCommand(cmd *proto.Command) (msg *rpc.Message, err error) {
	if cmd == nil || cmd.Type != proto.CMD_SUBSCRIPTION || self.conn == nil || self.subChan == nil {
		return
	}
	if len(cmd.Params) < 1 {
		err = proto.ErrBadPeerImpl
		return
	}
	if cmd.Message == nil {
		err = proto.ErrBadPeerImpl
		return
	}
	if len(cmd.Message.Header) == 0 {
		err = proto.ErrBadPeerImpl
		return
	}
	sub := true
	if cmd.Params[0] == "0" {
		sub = false
	} else if cmd.Params[0] == "1" {
		sub = true
	} else {
		return
	}
	req := new(rpc.SubscribeRequest)
	req.Params = cmd.Message.Header
	req.Service = self.conn.Service()
	req.Username = self.conn.Username()
	req.Subscribe = sub
	self.subChan <- req
	return
}
