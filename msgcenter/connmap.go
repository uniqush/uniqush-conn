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
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/petar/GoLLRB/llrb"
	"fmt"
	"bytes"
)

type connMap interface {
	AddConn(conn proto.Conn)
	GetConn(service, username string) []proto.Conn
	DelConn(conn proto.Conn)
}

func connKey(conn proto.Conn) string {
	return fmt.Sprintf("%v\n%v", conn.Service(), conn.Username())
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

func (self *treeBasedConnMap) GetConn(service, user string) []proto.Conn {
	key := fmt.Sprintf("%v\n%v", service, user)
	clif := self.tree.Get(key)
	cl, ok := clif.([]proto.Conn)
	if !ok || cl == nil {
		return nil
	}
	return cl
}

func (self *treeBasedConnMap) AddConn(conn proto.Conn) {
	if conn == nil {
		return
	}
	cl := self.GetConn(conn.Service(), conn.Username())
	if cl == nil {
		cl = make([]proto.Conn, 0, 3)
	}
	for _, c := range cl {
		if c.UniqId() == conn.UniqId() {
			return
		}
	}
	cl = append(cl, conn)
	self.tree.ReplaceOrInsert(cl)
	return
}

func (self *treeBasedConnMap) DelConn(conn proto.Conn) {
	if conn == nil {
		return
	}
	cl := self.GetConn(conn.Service(), conn.Username())
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
	cl[i] = cl[len(cl) - 1]
	cl = cl[:len(cl) - 1]
	return
}

func newTreeBasedConnMap() connMap {
	ret := new(treeBasedConnMap)
	ret.tree = llrb.New(lessConnList)
	return ret
}

