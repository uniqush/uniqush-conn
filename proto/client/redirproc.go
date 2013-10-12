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
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/rpc"
)

type RedirectRequest struct {
	addresses []string
}

type redirectProcessor struct {
	redirChan chan<- *RedirectRequest
}

func (self *redirectProcessor) ProcessCommand(cmd *proto.Command) (mc *rpc.MessageContainer, err error) {
	if cmd.Type != proto.CMD_REDIRECT || self.redirChan == nil {
		return
	}

	if len(cmd.Params) == 0 {
		return
	}

	req := new(RedirectRequest)
	req.addresses = cmd.Params
	self.redirChan <- req
	return
}
