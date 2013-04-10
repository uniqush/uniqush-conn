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
	"bytes"
	"fmt"
	"github.com/petar/GoLLRB/llrb"
	"github.com/uniqush/uniqush-conn/proto"
)

type connMap interface {
	AddConn(conn proto.Conn, maxNrConnsPerUser int, maxNrUsers int) error
	GetConn(username string) []proto.Conn
	DelConn(conn proto.Conn)
}

func connKey(conn proto.Conn) string {
	return conn.Username()
}

func lessConnList(a, b interface{}) bool {
	cl1, ok := a.([]proto.Conn)
	if !ok {
		return true
	}
	cl2, ok := b.([]proto.Conn)
	if !ok {
		return false
	}
	if len(cl1) == 0 {
		return true
	}
	if len(cl2) == 0 {
		return false
	}
	conn1 := cl1[0]
	conn2 := cl2[0]

	akey := connKey(conn1)
	bkey := connKey(conn2)

	cmp := bytes.Compare([]byte(akey), []byte(bkey))
	return cmp < 0
}

type treeBasedConnMap struct {
	tree *llrb.Tree
}

func (self *treeBasedConnMap) GetConn(user string) []proto.Conn {
	key := user
	clif := self.tree.Get(key)
	cl, ok := clif.([]proto.Conn)
	if !ok || cl == nil {
		return nil
	}
	return cl
}

var ErrTooManyUsers = errors.New("too many users")
var ErrTooManyConnForThisUser = errors.New("too many connections under this user")

func (self *treeBasedConnMap) AddConn(conn proto.Conn, maxNrConnsPerUser int, maxNrUsers int) error {
	if conn == nil {
		return nil
	}
	cl := self.GetConn(conn.Username())
	if cl == nil {
		if maxNrUsers > 0 && self.tree.Len() >= maxNrUsers {
			return ErrTooManyUsers
		}
		cl = make([]proto.Conn, 0, 3)
	}
	if maxNrConnsPerUser > 0 && len(cl) >= maxNrConnsPerUser {
		return ErrTooManyConnForThisUser
	}
	for _, c := range cl {
		if c.UniqId() == conn.UniqId() {
			return nil
		}
	}
	cl = append(cl, conn)
	self.tree.ReplaceOrInsert(cl)
	return nil
}

func (self *treeBasedConnMap) DelConn(conn proto.Conn) {
	if conn == nil {
		return
	}
	cl := self.GetConn(conn.Username())
	if cl == nil {
		return
	}
	i := -1
	var c proto.Conn
	for i, c = range cl {
		if c.UniqId() == conn.UniqId() {
			break
		}
	}
	if i < 0 {
		return
	}
	if len(cl) == 1 {
		self.tree.Delete(connKey(conn))
		return
	}
	cl[i] = cl[len(cl)-1]
	cl = cl[:len(cl)-1]
	return
}

func newTreeBasedConnMap() connMap {
	ret := new(treeBasedConnMap)
	ret.tree = llrb.New(lessConnList)
	return ret
}
