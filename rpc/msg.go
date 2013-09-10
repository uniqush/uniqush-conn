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

import "bytes"

type Message struct {
	Header map[string]string `json:"header,omitempty"`
	Body   []byte            `json:"body,omitempty"`
}

func (self *Message) IsEmpty() bool {
	if self == nil {
		return true
	}
	return len(self.Header) == 0 && len(self.Body) == 0
}

func (self *Message) Size() int {
	if self == nil {
		return 0
	}
	ret := len(self.Body)
	for k, v := range self.Header {
		ret += len(k) + 1
		ret += len(v) + 1
	}
	ret += 8
	return ret
}

func (a *Message) Eq(b *Message) bool {
	if a == nil {
		if b == nil {
			return true
		} else {
			return false
		}
	}
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
	return bytes.Equal(a.Body, b.Body)
}

/*
func (a *Message) Eq(b *Message) bool {
	if !a.EqContent(b) {
		return false
	}
	if a.Id != b.Id {
		return false
	}
	if a.Sender != b.Sender {
		return false
	}
	if a.SenderService != b.SenderService {
		return false
	}
	return true
}
*/
