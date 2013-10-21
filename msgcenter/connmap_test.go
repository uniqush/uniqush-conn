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
	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/proto/server"
	"github.com/uniqush/uniqush-conn/rpc"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"
)

type fakeConn struct {
	username string
	n        int
}

func (self *fakeConn) Username() string {
	return self.username
}

func (self *fakeConn) UniqId() string {
	return fmt.Sprintf("%v-%v", self.username, self.n)
}

func (self *fakeConn) RemoteAddr() net.Addr {
	return nil
}

func (self *fakeConn) Service() string {
	return "fakeservice"
}

func (self *fakeConn) Close() error {
	return nil
}

type connGenerator struct {
	nextId int
}

func (self *fakeConn) SendMessage(msg *rpc.Message, id string, extra map[string]string, tryDigest bool) error {
	return nil
}
func (self *fakeConn) ForwardMessage(sender, senderService string, msg *rpc.Message, id string, tryDigest bool) error {
	return nil
}
func (self *fakeConn) ReceiveMessage() (msg *rpc.Message, err error) {
	return nil, nil
}
func (self *fakeConn) SetMessageCache(cache msgcache.Cache) {
}

func (self *fakeConn) Redirect(addrs ...string) error {
	return nil
}
func (self *fakeConn) SetForwardRequestChannel(fwdChan chan<- *rpc.ForwardRequest) {
}
func (self *fakeConn) SetSubscribeRequestChan(subChan chan<- *rpc.SubscribeRequest) {
}
func (self *fakeConn) Visible() bool {
	return true
}

func (self *connGenerator) nextConn() server.Conn {
	usr := fmt.Sprintf("user-%v", self.nextId)
	self.nextId++
	return &fakeConn{username: usr}
}

func TestInsertConnMap(t *testing.T) {
	N := 10
	cmap := newTreeBasedConnMap(0, 0, 0)
	g := new(connGenerator)
	conns := make([]server.Conn, N)
	for i, _ := range conns {
		c := g.nextConn()
		err := cmap.AddConn(c)
		if err != nil {
			t.Errorf("%v", err)
		}
		conns[i] = c
	}
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if cs.NrConn() != 1 {
			t.Errorf("Bad for user %v: nr conns=%v", c.Username(), cs.NrConn())
			continue
		}

		err := cs.Traverse(func(c1 server.Conn) error {
			if c1.Username() != c.Username() {
				return fmt.Errorf("Bad for user %v", c.Username())
			}
			return nil
		})
		if err != nil {
			t.Errorf("%v", err)
		}
	}
}

func TestInsertDupConnMap(t *testing.T) {
	N := 10
	M := 2
	cmap := newTreeBasedConnMap(0, 0, 0)
	g := new(connGenerator)
	conns := make([]server.Conn, N)
	for i, _ := range conns {
		c := g.nextConn()

		for i := 0; i < M; i++ {
			u := c.Username()
			fc := &fakeConn{username: u, n: i}
			err := cmap.AddConn(fc)
			if err != nil {
				t.Errorf("%v", err)
			}
		}
		conns[i] = c
	}
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if cs.NrConn() != M {
			t.Errorf("Bad for user %v: nr conns=%v", c.Username(), cs.NrConn())
			continue
		}
		err := cs.Traverse(func(c1 server.Conn) error {
			if c1.Username() != c.Username() {
				return fmt.Errorf("Bad for user %v", c.Username())
			}
			return nil
		})
		if err != nil {
			t.Errorf("%v", err)
		}
	}
}

func TestDeleteConnMap(t *testing.T) {
	N := 10
	cmap := newTreeBasedConnMap(0, 0, 0)
	g := new(connGenerator)
	conns := make([]server.Conn, N)
	users := make([]string, N)
	for i, _ := range conns {
		c := g.nextConn()
		err := cmap.AddConn(c)
		if err != nil {
			t.Errorf("%v", err)
		}
		conns[i] = c
		users[i] = c.Username()
	}
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if cs.NrConn() != 1 {
			t.Errorf("Bad for user %v: nr conns=%v", c.Username(), cs.NrConn())
			continue
		}

		err := cs.Traverse(func(c1 server.Conn) error {
			if c1.Username() != c.Username() {
				return fmt.Errorf("Bad for user %v", c.Username())
			}
			return nil
		})
		if err != nil {
			t.Errorf("%v", err)
		}
	}
	for _, c := range conns {
		deleted := cmap.DelConn(c)
		if deleted == nil {
			t.Errorf("should delete a connection")
		}
	}
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if cs.NrConn() != 0 {
			t.Errorf("deletion failed. nr conn: %v", cs.NrConn())
		}
	}
}

func TestDeleteDupConnMap(t *testing.T) {
	N := 10
	M := 2
	cmap := newTreeBasedConnMap(0, 0, 0)
	g := new(connGenerator)
	conns := make([]server.Conn, N)
	for j, _ := range conns {
		c := g.nextConn()

		for i := 0; i < M; i++ {
			u := c.Username()
			fc := &fakeConn{username: u, n: i}
			err := cmap.AddConn(fc)
			if err != nil {
				t.Errorf("%v", err)
			}
		}
		conns[j] = c
	}
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if cs.NrConn() != M {
			t.Errorf("Bad for user %v: nr conns=%v", c.Username(), cs.NrConn())
			continue
		}

		i := 0
		var delConn server.Conn
		err := cs.Traverse(func(c1 server.Conn) error {
			if i == 0 {
				delConn = c1
			}
			i++
			if c1.Username() != c.Username() {
				return fmt.Errorf("Bad for user %v", c.Username())
			}
			return nil
		})
		if err != nil {
			t.Errorf("%v", err)
		}

		if delConn != nil {
			cmap.DelConn(delConn)
		}
	}
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if cs.NrConn() != M-1 {
			t.Errorf("should delete one connection for user %v: nr conns=%v", c.Username(), cs.NrConn())
		}
	}
}

func TestConcurrentGetAddDel(t *testing.T) {
	N := 1000
	cmap := newTreeBasedConnMap(0, 0, 0)
	g := new(connGenerator)
	conns := make([]server.Conn, N)
	users := make([]string, N)
	for i, _ := range conns {
		c := g.nextConn()
		conns[i] = c
		users[i] = c.Username()
	}

	var wg sync.WaitGroup

	for _, c := range conns {
		wg.Add(1)
		go func(c server.Conn) {
			defer wg.Done()
			err := cmap.AddConn(c)
			if err != nil {
				t.Errorf("%v", err)
			}
		}(c)
	}

	wg.Wait()
	for _, c := range conns {
		wg.Add(1)
		go func(c server.Conn) {
			defer wg.Done()
			cs := cmap.GetConn(c.Username())
			if cs.NrConn() != 1 {
				t.Errorf("Bad for user %v: nr conns=%v", c.Username(), cs.NrConn())
				return
			}

			err := cs.Traverse(func(c1 server.Conn) error {
				if c1.Username() != c.Username() {
					return fmt.Errorf("Bad for user %v", c.Username())
				}
				return nil
			})
			if err != nil {
				t.Errorf("%v", err)
			}
		}(c)
	}
	wg.Wait()
	for _, c := range conns {
		// This loop is only used to mimic concurrent read and write
		wg.Add(1)
		go func(c server.Conn) {
			defer wg.Done()
			time.Sleep(time.Duration(rand.Int63n(3)) * time.Second)
			cs := cmap.GetConn(c.Username())
			if cs.NrConn() == 1 {
				err := cs.Traverse(func(c1 server.Conn) error {
					if c1.Username() != c.Username() {
						return fmt.Errorf("Bad for user %v", c.Username())
					}
					return nil
				})
				if err != nil {
					t.Errorf("%v", err)
				}

			}
		}(c)
	}
	for _, c := range conns {
		wg.Add(1)
		go func(c server.Conn) {
			defer wg.Done()
			time.Sleep(time.Duration(rand.Int63n(3)) * time.Second)
			deleted := cmap.DelConn(c)
			if deleted == nil {
				t.Errorf("should delete a connection")
			}
		}(c)
	}
	wg.Wait()
	for _, c := range conns {
		cs := cmap.GetConn(c.Username())
		if cs.NrConn() != 0 {
			t.Errorf("deletion failed. nr conn: %v", cs.NrConn())
		}
	}
}
