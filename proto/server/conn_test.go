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
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/client"

	"io"
	"sync"
	"testing"
	"time"
)

type messageContainerProcessor interface {
	ProcessMessageContainer(mc *proto.MessageContainer) error
}

func iterateOverContainers(srcProc, dstProc messageContainerProcessor, mcs ...*proto.MessageContainer) error {
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

func randomMessage() *proto.Message {
	msg := new(proto.Message)
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

func (self *serverSender) ProcessMessageContainer(mc *proto.MessageContainer) error {
	if mc.FromUser() {
		return self.conn.ForwardMessage(mc.Sender, mc.SenderService, mc.Message, mc.Id)
	}
	return self.conn.SendMessage(mc.Message, mc.Id, self.extra)
}

type serverReceiver struct {
	conn Conn
}

func (self *serverReceiver) ProcessMessageContainer(mc *proto.MessageContainer) error {
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

func (self *clientReceiver) ProcessMessageContainer(mc *proto.MessageContainer) error {
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

func (self *clientSender) ProcessMessageContainer(mc *proto.MessageContainer) error {
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
	mcs := make([]*proto.MessageContainer, N)

	for i := 0; i < N; i++ {
		mcs[i] = &proto.MessageContainer{
			Message: randomMessage(),
			Id:      fmt.Sprintf("%v", i),
		}
	}

	ss := &serverSender{
		conn: servConn,
	}

	cr := &clientReceiver{
		conn: cliConn,
	}
	err = iterateOverContainers(ss, cr, mcs...)
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
	mcs := make([]*proto.MessageContainer, N)

	for i := 0; i < N; i++ {
		mcs[i] = &proto.MessageContainer{
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
