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
	"strings"
	"time"
)

type forwardProcessor struct {
	conn    *serverConn
	fwdChan chan<- *rpc.ForwardRequest
}

func (self *forwardProcessor) ProcessCommand(cmd *proto.Command) (msg *rpc.Message, err error) {
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
	fwdreq := new(rpc.ForwardRequest)
	fwdreq.Sender = self.conn.Username()
	fwdreq.SenderService = self.conn.Service()
	fwdreq.Message = cmd.Message
	fwdreq.TTL, err = time.ParseDuration(cmd.Params[0])

	if err != nil {
		err = nil
		fwdreq.TTL = 72 * time.Hour
	}
	fwdreq.Receivers = strings.Split(cmd.Params[1], ",")
	if len(cmd.Params) > 2 {
		fwdreq.ReceiverService = cmd.Params[2]
	} else {
		fwdreq.ReceiverService = self.conn.Service()
	}
	self.fwdChan <- fwdreq
	return
}
