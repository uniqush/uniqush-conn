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
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/client"
	"github.com/uniqush/uniqush-conn/proto/server"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type alwaysAllowAuth struct{}

func (self *alwaysAllowAuth) Authenticate(service, user, token string) (bool, error) {
	return true, nil
}

type nolimitServiceConfigReader struct {
}

func (self *nolimitServiceConfigReader) ReadConfig(service string) *ServiceConfig {
	return new(ServiceConfig)
}

func getMessageCenter(addr string, msgChan chan<- *proto.Message, fwdChan chan<- *server.ForwardRequest, connErrChan chan<- *EventConnError, errChan chan<- error) (center *MessageCenter, pubkey *rsa.PublicKey, err error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}
	privkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	pubkey = &privkey.PublicKey
	authtimeout := 3 * time.Second

	center = NewMessageCenter(ln, privkey, msgChan, fwdChan, connErrChan, errChan, authtimeout, &alwaysAllowAuth{}, &nolimitServiceConfigReader{})
	return
}

func connectServer(addr, username string, pub *rsa.PublicKey, digestChan chan<- *client.Digest) (conn client.Conn, err error) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	conn, err = client.Dial(c, pub, "service", username, "token", 10*time.Second)
	return
}

func server2client(center *MessageCenter, clients []client.Conn, errChan chan<- error, msgs ...*proto.Message) {
	for _, client := range clients {
		if client == nil {
			continue
		}
		for _, msg := range msgs {
			_, err := center.SendOrQueue(client.Service(), client.Username(), msg, nil)
			if err != nil {
				errChan <- err
			}
		}
	}
}

func testClientReceived(client client.Conn, errChan chan<- error, msgs ...*proto.Message) {
	for _, msg := range msgs {
		m, err := client.ReadMessage()
		if err != nil {
			errChan <- err
			continue
		}
		if !m.EqContent(msg) {
			errChan <- fmt.Errorf("[client=%v] %v != %v", client.Username(), m, msg)
		}
	}
}

func reportError(errChan <-chan error, t *testing.T) {
	for err := range errChan {
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
}

func reportConnError(errChan <-chan *EventConnError, t *testing.T) {
	for err := range errChan {
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
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

func TestServerSendToClients(t *testing.T) {
	addr := "127.0.0.1:8964"
	N := 10
	connErrChan := make(chan *EventConnError)
	errChan := make(chan error)
	go reportConnError(connErrChan, t)
	defer close(connErrChan)
	go reportError(errChan, t)
	defer close(errChan)

	center, pubkey, err := getMessageCenter(addr, nil, nil, connErrChan, errChan)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	go center.Start()

	clients := make([]client.Conn, N)
	wg := new(sync.WaitGroup)

	msg := randomMessage()

	for i, _ := range clients {
		username := fmt.Sprintf("user-%v", i)
		client, err := connectServer(addr, username, pubkey, nil)
		if err != nil {
			t.Errorf("Error: %v", err)
			return
		}
		clients[i] = client
		wg.Add(1)
		go func() {
			testClientReceived(client, errChan, msg)
			wg.Done()
		}()
	}
	server2client(center, clients, errChan, msg)
	wg.Wait()
}

func receiveAndCompareMessages(msgChan <-chan *proto.Message, msgs map[string]*proto.Message, errChan chan<- error) {
	for msg := range msgChan {
		if m, ok := msgs[msg.Sender]; ok {
			if !m.EqContent(msg) {
				errChan <- fmt.Errorf("user %v should receive %v; but got %v", msg.Sender, m, msg)
			}
		} else {
			errChan <- fmt.Errorf("Received message from unknown user: %v.", msg.Sender)
		}
	}
}

func TestClientsSendToServer(t *testing.T) {
	addr := "127.0.0.1:8965"
	N := 10
	connErrChan := make(chan *EventConnError)
	errChan := make(chan error)
	go reportConnError(connErrChan, t)
	defer close(connErrChan)
	go reportError(errChan, t)
	defer close(errChan)

	msgChan := make(chan *proto.Message)
	center, pubkey, err := getMessageCenter(addr, msgChan, nil, connErrChan, errChan)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	go center.Start()

	clients := make([]client.Conn, N)
	msgs := make(map[string]*proto.Message, N)

	for i, _ := range clients {
		username := fmt.Sprintf("user-%v", i)
		client, err := connectServer(addr, username, pubkey, nil)
		if err != nil {
			t.Errorf("Error: %v", err)
			return
		}
		msg := randomMessage()
		msgs[username] = msg
		clients[i] = client
	}

	go receiveAndCompareMessages(msgChan, msgs, errChan)
	defer close(msgChan)

	wg := new(sync.WaitGroup)
	wg.Add(N)
	for _, client := range clients {
		msg := msgs[client.Username()]
		go func() {
			client.SendMessage(msg)
			wg.Done()
		}()
	}
	wg.Wait()
}

