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
	"errors"

	"github.com/petar/GoLLRB/llrb"
	"github.com/uniqush/uniqush-conn/proto/server"
	"sync"
)

type connMap interface {
	AddConn(conn server.Conn) error
	GetConn(username string) ConnSet
	DelConn(conn server.Conn) server.Conn
	CloseAll()
}
type treeBasedConnMap struct {
	tree              *llrb.LLRB
	maxNrConn         int
	maxNrUsers        int
	maxNrConnsPerUser int

	nrConn int
	lock   sync.Mutex
}

func (self *treeBasedConnMap) GetConn(user string) ConnSet {
	self.lock.Lock()
	defer self.lock.Unlock()
	cset := self.getConn(user)
	return cset
}

func (self *treeBasedConnMap) getConn(user string) *connSet {
	key := &connSet{name: user, list: nil}
	clif := self.tree.Get(key)
	if clif == nil {
		return nil
	}
	cl, ok := clif.(*connSet)
	if !ok || cl == nil {
		return nil
	}
	return cl
}

var ErrTooManyUsers = errors.New("too many users")
var ErrTooManyConnForThisUser = errors.New("too many connections under this user")
var ErrTooManyConns = errors.New("too many connections")

// There's no way back!
func (self *treeBasedConnMap) CloseAll() {
	self.lock.Lock()
	var nilcs *connSet
	self.tree.AscendGreaterOrEqual(nilcs, func(i llrb.Item) bool {
		if cs, ok := i.(*connSet); ok {
			cs.CloseAll()
		}
		return true
	})
	self.tree = llrb.New()
}

func (self *treeBasedConnMap) AddConn(conn server.Conn) error {
	self.lock.Lock()
	defer self.lock.Unlock()
	if conn == nil {
		return nil
	}

	if self.maxNrConn > 0 && self.nrConn >= self.maxNrConn {
		return ErrTooManyConns
	}
	if self.maxNrUsers > 0 && self.tree.Len() >= self.maxNrUsers {
		return ErrTooManyUsers
	}
	cset := self.getConn(connKey(conn))

	if cset == nil {
		cset = &connSet{name: connKey(conn), list: make([]server.Conn, 0, 3)}
	}

	cset.lock()
	defer cset.unlock()

	err := cset.add(conn, self.maxNrConnsPerUser)
	if err != nil {
		return err
	}

	self.nrConn++
	self.tree.ReplaceOrInsert(cset)
	return nil
}

func (self *treeBasedConnMap) DelConn(conn server.Conn) server.Conn {
	self.lock.Lock()
	defer self.lock.Unlock()
	if conn == nil {
		return nil
	}
	cset := self.getConn(connKey(conn))
	if cset == nil {
		return nil
	}

	cset.lock()
	defer cset.unlock()

	ret := cset.del(conn.UniqId())

	if ret == nil {
		return ret
	}
	self.nrConn--
	if cset.nrConn() == 0 {
		self.tree.Delete(cset)
	} else {
		self.tree.ReplaceOrInsert(cset)
	}

	return ret
}

func newTreeBasedConnMap(maxNrConn, maxNrUsers, maxNrConnsPerUser int) connMap {
	ret := new(treeBasedConnMap)
	ret.tree = llrb.New()
	ret.maxNrConn = maxNrConn
	ret.maxNrUsers = maxNrUsers
	ret.maxNrConnsPerUser = maxNrConnsPerUser
	return ret
}
