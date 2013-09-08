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
	"strconv"

	"github.com/uniqush/uniqush-conn/proto"
)

type Digest struct {
	MsgId         string
	Sender        string
	SenderService string
	Size          int
	Info          map[string]string
}

type digestProcessor struct {
	digestChan chan<- *Digest
	service    string
}

func (self *digestProcessor) ProcessCommand(cmd *proto.Command) (mc *proto.MessageContainer, err error) {
	if cmd.Type != proto.CMD_DIGEST || self.digestChan == nil {
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
			digest.SenderService = self.service
		}
	}
	self.digestChan <- digest

	return
}
