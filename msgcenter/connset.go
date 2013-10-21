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

package msgcenter

import (
	"github.com/petar/GoLLRB/llrb"
	"github.com/uniqush/uniqush-conn/proto/server"
	"sync"
)

type ConnSet interface {
	NrConn() int
	Traverse(f func(c server.Conn) error) error
	CloseAll()
}

type connSet struct {
	mutex sync.RWMutex
	name  string
	list  []server.Conn
}

func (self *connSet) CloseAll() {
	if self == nil {
		return
	}
	self.mutex.Lock()
	for _, c := range self.list {
		c.Close()
	}
	self.list = nil
}

func (self *connSet) unlock() {
	self.mutex.Unlock()
}

// Never manipulate another connSet inside the function f.
// It may lead to deadlock.
// Only perform read-only operation (to the connSet, not the Conn) in the function f().
func (self *connSet) Traverse(f func(c server.Conn) error) error {
	if self == nil {
		return nil
	}
	self.mutex.RLock()
	defer self.mutex.RUnlock()

	if self == nil {
		return nil
	}

	for _, c := range self.list {
		err := f(c)
		if err != nil {
			return err
		}
	}
	return nil
}

func (self *connSet) lock() {
	self.mutex.Lock()
}

func (self *connSet) NrConn() int {
	if self == nil {
		return 0
	}
	self.mutex.Lock()
	defer self.mutex.Unlock()

	return self.nrConn()
}

func (self *connSet) nrConn() int {
	if self == nil {
		return 0
	}
	return len(self.list)
}

func (self *connSet) del(key string) server.Conn {
	var c server.Conn
	i := -1
	for i, c = range self.list {
		if c.UniqId() == key {
			break
		}
	}
	if i < 0 || i >= len(self.list) {
		return nil
	}
	c = self.list[i]
	self.list[i] = self.list[len(self.list)-1]
	self.list = self.list[:len(self.list)-1]

	return c
}

func (self *connSet) add(conn server.Conn, max int) error {
	for _, c := range self.list {
		if c.UniqId() == conn.UniqId() {
			return nil
		}
	}
	if max > 0 && len(self.list) >= max {
		return ErrTooManyConnForThisUser
	}
	self.list = append(self.list, conn)
	return nil
}

func (self *connSet) key() string {
	if len(self.list) == 0 {
		return self.name
	}
	return connKey(self.list[0])
}

func (self *connSet) Less(than llrb.Item) bool {
	if self == nil {
		return true
	}
	thanCs := than.(*connSet)
	if thanCs == nil {
		return false
	}
	selfKey := llrb.String(self.key())
	thanKey := llrb.String(than.(*connSet).key())
	return selfKey.Less(thanKey)
}

func connKey(conn server.Conn) string {
	return conn.Username()
}
