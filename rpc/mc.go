/*
 * Copyright 2013 Nan Deng
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

package rpc

// MessageContainer is used to represent a message inside
// the program. It has meta-data about a message like:
// the message id, the sender and the service of the sender.
type MessageContainer struct {
	Message       *Message `json:"msg"`
	Id            string   `json:"id,omitempty"`
	Sender        string   `json:"sender,omitempty"`
	SenderService string   `json:"sender-service,omitempty"`
}

func (self *MessageContainer) FromServer() bool {
	return len(self.Sender) == 0
}

func (self *MessageContainer) FromUser() bool {
	return !self.FromServer()
}

func (a *MessageContainer) Eq(b *MessageContainer) bool {
	if a.Id != b.Id {
		return false
	}
	if a.Sender != b.Sender {
		return false
	}
	if a.SenderService != b.SenderService {
		return false
	}
	return a.Message.Eq(b.Message)
}
