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
	"fmt"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/client"
	"io"
	"sync"
	"testing"
	"time"
)

func sendTestMessagesFromServerToClient(sConn Conn, cConn client.Conn, msgs ...*proto.Message) error {
	wg := new(sync.WaitGroup)
	wg.Add(2)

	var es error
	var ed error

	go func() {
		defer wg.Done()
		for i, msg := range msgs {
			es = sConn.SendMessage(msg, fmt.Sprintf("%v", i), nil)
			if es != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		var m *proto.Message
		for _, msg := range msgs {
			m, ed = cConn.ReceiveMessage()
			if ed != nil {
				return
			}
			if !m.Eq(msg) {
				ed = fmt.Errorf("corrupted data")
				return
			}
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
	msgs := make([]*proto.Message, N)

	for i := 0; i < N; i++ {
		msgs[i] = randomMessage()
	}
	err = sendTestMessagesFromServerToClient(servConn, cliConn, msgs...)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}
