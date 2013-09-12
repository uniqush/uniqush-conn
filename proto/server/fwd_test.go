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
	"fmt"

	"github.com/uniqush/uniqush-conn/proto/client"
	"github.com/uniqush/uniqush-conn/rpc"

	"testing"
	"time"
)

func TestForwardMessageFromServerToClient(t *testing.T) {
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
			Message:       randomMessage(),
			Id:            fmt.Sprintf("%v", i),
			Sender:        "sender",
			SenderService: "someservice",
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

type clientForwarder struct {
	conn client.Conn
}

func (self *clientForwarder) ProcessMessageContainer(mc *rpc.MessageContainer) error {
	err := self.conn.SendMessageToUsers(mc.Message, 1*time.Hour, mc.SenderService, mc.Sender)
	if err != nil {
		return err
	}
	return self.conn.SendMessageToServer(mc.Message)
}

func TestForwardRequestFromClientToServer(t *testing.T) {
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

	receiver := "receiver"
	receiverService := "someservice"

	for i := 0; i < N; i++ {
		mcs[i] = &rpc.MessageContainer{
			Message:       randomMessage(),
			Id:            fmt.Sprintf("%v", i),
			Sender:        receiver, // This is confusing. We hacked the struct.
			SenderService: receiverService,
		}
	}

	fwdChan := make(chan *rpc.ForwardRequest)

	servConn.SetForwardRequestChannel(fwdChan)
	src := &clientForwarder{
		conn: cliConn,
	}

	dst := &serverReceiver{
		conn: servConn,
	}

	go func() {
		i := 0
		for fwdreq := range fwdChan {
			mc := mcs[i]
			i++
			if !mc.Message.Eq(fwdreq.Message) {
				t.Errorf("corrupted data")
			}
			if len(fwdreq.Receivers) != 1 {
				t.Errorf("receivers: %v", fwdreq.Receivers)
			}
			if fwdreq.Receivers[0] != receiver {
				t.Errorf("receiver is %v, not %v", fwdreq.Receivers, receiver)
			}
			if fwdreq.ReceiverService != receiverService {
				t.Errorf("receiver's service is %v, not %v", fwdreq.ReceiverService, receiverService)
			}
		}
		if i != N {
			t.Errorf("received only %v fwdreq", i)
		}
	}()
	err = iterateOverContainers(src, dst, mcs...)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	close(fwdChan)
	cliConn.Close()
}
