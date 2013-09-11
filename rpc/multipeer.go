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

package rpc

import "sync"

type MultiPeer struct {
	peers []UniqushConnPeer
	lock  sync.Mutex
}

func NewMultiPeer(peers ...UniqushConnPeer) *MultiPeer {
	ret := new(MultiPeer)
	ret.peers = peers
	return ret
}

func (self *MultiPeer) AddPeer(p UniqushConnPeer) {
	if self == nil {
		return
	}
	self.lock.Lock()
	defer self.lock.Unlock()

	self.peers = append(self.peers, p)
}

func (self *MultiPeer) do(f func(p UniqushConnPeer) *Result) *Result {
	if self == nil {
		return nil
	}
	ret := new(Result)
	self.lock.Lock()
	defer self.lock.Unlock()

	for _, p := range self.peers {
		r := f(p)
		if r.Error != nil {
			ret.Error = r.Error
			return ret
		}

		ret.Results = append(ret.Results, r.Results...)
	}
	return ret
}

func (self *MultiPeer) Send(req *SendRequest) *Result {
	return self.do(func(p UniqushConnPeer) *Result {
		return p.Send(req)
	})
}

func (self *MultiPeer) Forward(req *ForwardRequest) *Result {
	return self.do(func(p UniqushConnPeer) *Result {
		return p.Forward(req)
	})
}
