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
	"time"

	"github.com/uniqush/uniqush-conn/proto"
)

type forwardProcessor struct {
	conn    *serverConn
	fwdChan chan<- *ForwardRequest
}

type ForwardRequest struct {
	Receiver         string                 `json:"receiver"`
	ReceiverService  string                 `json:"service"`
	TTL              time.Duration          `json:"ttl"`
	MessageContainer proto.MessageContainer `json:"msg"`
}

func (self *forwardProcessor) ProcessCommand(cmd *proto.Command) (msg *proto.Message, err error) {
	if cmd == nil || cmd.Type != proto.CMD_FWD_REQ || self.conn == nil || self.fwdChan == nil {
		return
	}

	if len(cmd.Params) < 2 {
		err = proto.ErrBadPeerImpl
		return
	}
	if self.fwdChan == nil {
		return
	}
	fwdreq := new(ForwardRequest)
	fwdreq.MessageContainer.Sender = self.conn.Username()
	fwdreq.MessageContainer.SenderService = self.conn.Service()
	fwdreq.MessageContainer.Message = cmd.Message
	fwdreq.TTL, err = time.ParseDuration(cmd.Params[0])

	if err != nil {
		err = nil
		fwdreq.TTL = 72 * time.Hour
	}
	fwdreq.Receiver = cmd.Params[1]
	if len(cmd.Params) > 2 {
		fwdreq.ReceiverService = cmd.Params[2]
	} else {
		fwdreq.ReceiverService = self.conn.Service()
	}
	self.fwdChan <- fwdreq
	return
}
