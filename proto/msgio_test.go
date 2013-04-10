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

package proto

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"testing"
)

func testMessageExchange(addr string, msgs ...*Message) error {
	c2sConn, s2cConn, err := buildServerClient(addr)
	if err != nil {
		return err
	}
	defer c2sConn.Close()
	defer s2cConn.Close()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	pub := &priv.PublicKey

	ch := make(chan bool)
	var ec error
	var es error
	go func() {
		defer func() {
			ch <- true
		}()
		ks, err := ClientKeyExchange(pub, c2sConn)
		if err != nil {
			es = fmt.Errorf("Client: key ex error: %v", err)
			return
		}
		cmdio := ks.ClientCommandIO(c2sConn)
		cliConn := NewConn(cmdio, "service", "username", c2sConn, nil)
		for i, msg := range msgs {
			m, err := cliConn.ReadMessage()
			if err != nil {
				ec = err
				return
			}
			if m == nil {
				if msg != nil {
					ec = fmt.Errorf("%vth message corrupted", i)
				}
				continue
			}
			if msg == nil {
				msg = new(Message)
			}
			msg.Sender = cliConn.Username()
			msg.SenderService = cliConn.Service()
			if !m.Eq(msg) {
				ec = fmt.Errorf("%vth message corrupted", i)
				return
			}
		}
		ec = nil
	}()
	go func() {
		defer func() {
			ch <- true
		}()
		ks, err := ServerKeyExchange(priv, s2cConn)
		if err != nil {
			es = fmt.Errorf("Server: key ex error: %v", err)
			return
		}
		cmdio := ks.ServerCommandIO(s2cConn)
		servConn := NewConn(cmdio, "service", "username", s2cConn, nil)
		for _, msg := range msgs {
			err := servConn.WriteMessage(msg, true, true)
			if err != nil {
				es = err
				return
			}
		}
	}()
	i := 0
	for _ = range ch {
		if es != nil {
			return es
		}
		if ec != nil {
			return ec
		}
		i++
		if i >= 2 {
			break
		}
	}
	return nil
}

func randomMessage() *Message {
	msg := new(Message)
	msg.Body = make([]byte, 10)
	io.ReadFull(rand.Reader, msg.Body)
	msg.Header = make(map[string]string, 2)
	msg.Header["aaa"] = "hello"
	msg.Header["aa"] = "hell"
	if msg.Body[0]%2 == 0 {
		msg.Id = "messageId"
	}
	return msg
}

func TestExchangingSingleMessage(t *testing.T) {
	msg := randomMessage()
	err := testMessageExchange("127.0.0.1:8088", msg)
	if err != nil {
		t.Errorf("%v", err)
	}
	return
}

func TestExchangingSingleMessageWithSender(t *testing.T) {
	msg := randomMessage()
	msg.Sender = "newsender"
	err := testMessageExchange("127.0.0.1:8088", msg)
	if err == nil {
		t.Errorf("should be error, because we didn't process forward command")
	}
	return
}

func TestExchangingEmpty(t *testing.T) {
	err := testMessageExchange("127.0.0.1:8088", nil)
	if err != nil {
		t.Errorf("%v", err)
	}
	return
}
