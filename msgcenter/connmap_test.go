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
	"fmt"
	"testing"
)

type fakeConn struct {
	username string
	n int
}

func (self *fakeConn) Username() string {
	return self.username
}

func (self *fakeConn) UniqId() string {
	return fmt.Sprintf("%v-%v", self.username, self.n)
}

type connGenerator struct {
	nextId int
}

func (self *connGenerator) nextConn() minimalConn {
	usr := fmt.Sprintf("user-%v", self.nextId)
	self.nextId++
	return &fakeConn{username:usr}
}

func TestInsertConnMap(t *testing.T) {
	N := 10
	cmap := newTreeBasedConnMap()
	g := new(connGenerator)
	conns := make([]minimalConn, N)
	for i, _ := range conns {
		c := g.nextConn()
		err := cmap.AddConn(c, 0, 0)
		if err != nil {
			t.Errorf("%v", err)
		}
		conns[i] = c
	}
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if len(cs) != 1 {
			t.Errorf("Bad for user %v: nr conns=%v", c.Username(), len(cs))
			continue
		}
		if cs[0].Username() != c.Username() {
			t.Errorf("Bad for user %v", c.Username())
		}
	}
}

func TestInsertDupConnMap(t *testing.T) {
	N := 10
	M := 2
	cmap := newTreeBasedConnMap()
	g := new(connGenerator)
	conns := make([]minimalConn, N)
	for i, _ := range conns {
		c := g.nextConn()

		for i := 0; i < M; i++ {
			u := c.Username()
			fc := &fakeConn{username:u, n:i}
			err := cmap.AddConn(fc, 0, 0)
			if err != nil {
				t.Errorf("%v", err)
			}
		}
		conns[i] = c
	}
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if len(cs) != M {
			t.Errorf("Bad for user %v: nr conns=%v", c.Username(), len(cs))
			continue
		}
		for _, conn := range cs {
			if conn.Username() != c.Username() {
				t.Errorf("Bad for user %v", c.Username())
			}
		}
	}
}

