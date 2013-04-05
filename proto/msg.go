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

package proto

import (
)

type Message struct {
	Header map[string]string ",omitempty"
	Body   []byte            ",omitempty"
}

func (a *Message) Eq(b *Message) bool {
	if len(a.Header) != len(b.Header) {
		return false
	}
	for k, v := range a.Header {
		if bv, ok := b.Header[k]; ok {
			if bv != v {
				return false
			}
		} else {
			return false
		}
	}
	return bytesEq(a.Body, b. Body)
}

const (
	cmdflag_COMPRESS = 1 << iota
	cmdflag_ENCRYPT
)

const (
	cmdtype_DATA = iota
)

type command struct {
	Type    uint16   ",omitempty"
	Params  [][]byte ",omitempty"
	Message *Message ",omitempty"
}

func (self *command) eq(cmd *command) bool {
	if self.Type != cmd.Type {
		return false
	}
	if len(self.Params) != len(cmd.Params) {
		return false
	}
	for i, p := range self.Params {
		if !bytesEq(cmd.Params[i], p) {
			return false
		}
	}
	return self.Message.Eq(cmd.Message)
}

