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

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type MultiPeer struct {
	peers []UniqushConnPeer
	lock  sync.RWMutex
	id    string
}

func NewMultiPeer(peers ...UniqushConnPeer) *MultiPeer {
	ret := new(MultiPeer)
	ret.peers = peers
	ret.id = fmt.Sprintf("%v-%v", time.Now().UnixNano(), rand.Int63())
	return ret
}

func (self *MultiPeer) Id() string {
	return self.id
}

func (self *MultiPeer) AddPeer(p UniqushConnPeer) {
	if self == nil {
		return
	}
	self.lock.Lock()
	defer self.lock.Unlock()

	peerId := p.Id()

	// We don't want to add same peer twice
	for _, i := range self.peers {
		if i.Id() == peerId {
			return
		}
	}
	self.peers = append(self.peers, p)
}

func (self *MultiPeer) do(f func(p UniqushConnPeer) *Result) *Result {
	if self == nil {
		return nil
	}
	ret := new(Result)
	self.lock.RLock()
	defer self.lock.RUnlock()

	for _, p := range self.peers {
		r := f(p)
		if r == nil {
			continue
		}
		if r.Error != "" {
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

func (self *MultiPeer) Redirect(req *RedirectRequest) *Result {
	return self.do(func(p UniqushConnPeer) *Result {
		return p.Redirect(req)
	})
}

func (self *MultiPeer) CheckUserStatus(req *UserStatusQuery) *Result {
	return self.do(func(p UniqushConnPeer) *Result {
		return p.CheckUserStatus(req)
	})
}
