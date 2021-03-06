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

package server

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/uniqush/uniqush-conn/msgcache"

	"github.com/uniqush/uniqush-conn/proto/client"
	"github.com/uniqush/uniqush-conn/rpc"
	"io"
	"sync"
	"testing"
	"time"
)

func clearCache() {
	db := 1
	c, _ := redis.Dial("tcp", "localhost:6379")
	c.Do("SELECT", db)
	c.Do("FLUSHDB")
	c.Close()
}

func getCache() msgcache.Cache {
	clearCache()
	cache, _ := msgcache.GetCache("redis", "", "", "", "1", 0)
	return cache
}

type messageContainerProcessor interface {
	ProcessMessageContainer(mc *rpc.MessageContainer) error
}

func iterateOverContainers(srcProc, dstProc messageContainerProcessor, mcs ...*rpc.MessageContainer) error {
	wg := new(sync.WaitGroup)
	wg.Add(2)

	var es error
	var ed error

	go func() {
		defer wg.Done()
		for _, mc := range mcs {
			es = srcProc.ProcessMessageContainer(mc)
		}
	}()

	go func() {
		defer wg.Done()
		for _, mc := range mcs {
			ed = dstProc.ProcessMessageContainer(mc)
		}
	}()
	wg.Wait()
	if es != nil {
		return es
	}
	if ed != nil {
		return ed
	}
	return nil
}

func randomMessage() *rpc.Message {
	msg := new(rpc.Message)
	msg.Body = make([]byte, 10)
	io.ReadFull(rand.Reader, msg.Body)
	msg.Header = make(map[string]string, 2)
	msg.Header["aaa"] = "hello"
	msg.Header["aa"] = "hell"
	return msg
}

type serverSender struct {
	conn  Conn
	extra map[string]string
}

func (self *serverSender) ProcessMessageContainer(mc *rpc.MessageContainer) error {
	if mc.FromUser() {
		return self.conn.ForwardMessage(mc.Sender, mc.SenderService, mc.Message, mc.Id, true)
	}
	return self.conn.SendMessage(mc.Message, mc.Id, self.extra, true)
}

type serverReceiver struct {
	conn Conn
}

func (self *serverReceiver) ProcessMessageContainer(mc *rpc.MessageContainer) error {
	msg, err := self.conn.ReceiveMessage()
	if err != nil {
		return err
	}
	if !msg.Eq(mc.Message) {
		return errors.New("corrupted data")
	}
	return nil
}

type clientReceiver struct {
	conn client.Conn
}

func (self *clientReceiver) ProcessMessageContainer(mc *rpc.MessageContainer) error {
	rmc, err := self.conn.ReceiveMessage()
	if err != nil {
		return err
	}
	if !rmc.Eq(mc) {
		return errors.New("corrupted data")
	}
	return nil
}

type clientSender struct {
	conn client.Conn
}

func (self *clientSender) ProcessMessageContainer(mc *rpc.MessageContainer) error {
	return self.conn.SendMessageToServer(mc.Message)
}

func TestSendMessageFromServerToClient(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer servConn.Close()
	defer cliConn.Close()
	N := 100
	mcs := make([]*rpc.MessageContainer, N)

	for i := 0; i < N; i++ {
		mcs[i] = &rpc.MessageContainer{
			Message: randomMessage(),
			Id:      fmt.Sprintf("%v", i),
		}
	}

	src := &serverSender{
		conn: servConn,
	}

	dst := &clientReceiver{
		conn: cliConn,
	}
	err = iterateOverContainers(src, dst, mcs...)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestSendMessageFromClientToServer(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer servConn.Close()
	defer cliConn.Close()
	N := 100
	mcs := make([]*rpc.MessageContainer, N)

	for i := 0; i < N; i++ {
		mcs[i] = &rpc.MessageContainer{
			Message: randomMessage(),
			Id:      fmt.Sprintf("%v", i),
		}
	}

	src := &clientSender{
		conn: cliConn,
	}

	dst := &serverReceiver{
		conn: servConn,
	}
	err = iterateOverContainers(src, dst, mcs...)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}
