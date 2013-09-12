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
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"crypto/rsa"
	"github.com/garyburd/redigo/redis"
	"github.com/uniqush/uniqush-conn/config"
	"github.com/uniqush/uniqush-conn/proto/client"
	"github.com/uniqush/uniqush-conn/rpc"
)

func clearCache() {
	db := 1
	c, _ := redis.Dial("tcp", "localhost:6379")
	c.Do("SELECT", db)
	c.Do("FLUSHDB")
	c.Close()
}

var configFileContent string = `
http-addr: 127.0.0.1:8088
handshake-timeout: 10s
auth:
  default: allow
  url: http://localhost:8080/auth
  timeout: 3s
err: 
  url: http://localhost:8080/err
  timeout: 3s
default:
  msg:
    url: http://localhost:8080/msg
    timeout: 3s
  err: 
    url: http://localhost:8080/err
    timeout: 3s
  login: 
    url: http://localhost:8080/login
    timeout: 3s
  logout: 
    url: http://localhost:8080/logout
    timeout: 3s
  fwd: 
    default: allow
    url: http://localhost:8080/fwd
    timeout: 3s
    max-ttl: 36h
  subscribe:
    default: allow
    url: http://localhost:8080/subscribe
    timeout: 3s
  unsubscribe:
    default: allow
    url: http://localhost:8080/unsubscribe
    timeout: 3s
  uniqush-push:
    addr: localhost:9898
    timeout: 3s
  max-conns: 2048
  max-online-users: 2048
  max-conns-per-user: 10
  db:
    engine: redis
    addr: 127.0.0.1:6379
    name: 1
`

type serverInfo struct {
	port           int
	privkey        *rsa.PrivateKey
	defaultService string
}

func newServerInfo(port int) *serverInfo {
	ret := new(serverInfo)
	err := io.EOF
	for err != nil {
		ret.privkey, err = rsa.GenerateKey(rand.Reader, 2048)
	}
	ret.port = port
	ret.defaultService = "service"
	return ret
}

func (self *serverInfo) getMessageCenter() (center *MessageCenter, err error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%v", self.port))
	if err != nil {
		return
	}

	confReader := bytes.NewBufferString(configFileContent)
	conf, err := config.Parse(confReader)
	if err != nil {
		return
	}
	center = NewMessageCenter(ln, self.privkey, conf)
	return
}

func (self *serverInfo) connClient(username string) (conn client.Conn, err error) {
	c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%v", self.port))
	if err != nil {
		return
	}
	conn, err = client.Dial(c, &self.privkey.PublicKey, self.defaultService, username, "token", 10*time.Second)
	return
}

type clientMessageVerifier struct {
	conns []client.Conn
}

func (self *clientMessageVerifier) genClient(n int, si *serverInfo) (usrnames []string, err error) {
	usrnames = make([]string, 0, n)
	for i := 0; i < n; i++ {
		usr := fmt.Sprintf("usr-%v", i)
		usrnames = append(usrnames, usr)
		c, err := si.connClient(usr)
		if err != nil {
			return nil, err
		}
		self.addClient(c)
	}
	return
}

func (self *clientMessageVerifier) addClient(c client.Conn) {
	self.conns = append(self.conns, c)
}

func verifySingle(conn client.Conn, mcs ...*rpc.MessageContainer) error {
	for i := 0; i < len(mcs); i++ {
		rmc, err := conn.ReceiveMessage()
		if err != nil {
			return err
		}

		found := false
		for _, mc := range mcs {
			if rmc.Message.Eq(mc.Message) && rmc.Sender == mc.Sender && rmc.SenderService == mc.SenderService {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%vth message is corrupted for user %v; %+v. sender: %v; sender's service: %v", i, conn.Username(), rmc.Message, rmc.Sender, rmc.SenderService)
		}
	}
	return nil
}

func (self *clientMessageVerifier) RunAndVerify(mc ...*rpc.MessageContainer) []error {
	ret := make([]error, len(self.conns))
	wg := &sync.WaitGroup{}

	for i, conn := range self.conns {
		wg.Add(1)
		c := conn
		go func() {
			ret[i] = verifySingle(c, mc...)
			wg.Done()
		}()
	}
	wg.Wait()
	return ret
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

func TestSendMessageFromServerToClients(t *testing.T) {
	clearCache()
	defer clearCache()
	si := newServerInfo(9891)

	center, err := si.getMessageCenter()
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}

	go center.Start()
	defer center.Stop()

	nrClients := 100
	clients := &clientMessageVerifier{}
	usrs, err := clients.genClient(nrClients, si)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}

	N := 3
	mcs := make([]*rpc.MessageContainer, N)
	for i := 0; i < N; i++ {
		mcs[i] = &rpc.MessageContainer{
			Message: randomMessage(),
			Id:      fmt.Sprintf("%v", i),
		}
	}

	go func() {
		for _, username := range usrs {
			usr := username
			go func() {
				for _, mc := range mcs {
					req := &rpc.SendRequest{
						ReceiverService: si.defaultService,
						Receivers:       []string{usr},
						TTL:             1 * time.Hour,
						Message:         mc.Message,
					}
					result := center.Send(req)
					if result.Error != nil {
						t.Errorf("Error on sending: %v", result.Error)
					}
					if result.NrSuccess() != 1 {
						t.Errorf("We got %v success", result.NrSuccess())
					}
				}
			}()
		}
	}()

	errs := clients.RunAndVerify(mcs...)
	for _, err := range errs {
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
}

func TestForwardMessagesBetweenClients(t *testing.T) {
	clearCache()
	defer clearCache()
	si := newServerInfo(9891)

	center, err := si.getMessageCenter()
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}

	go center.Start()
	defer center.Stop()

	nrReceivers := 10
	clients := &clientMessageVerifier{}
	usrs, err := clients.genClient(nrReceivers, si)
	if err != nil {
		t.Errorf("Error: %v\n", err)
		return
	}

	sender, err := si.connClient("sender")
	if err != nil {
		t.Errorf("Error: %v\n", err)
		return
	}
	N := 10
	mcs := make([]*rpc.MessageContainer, N)
	for i := 0; i < N; i++ {
		mcs[i] = &rpc.MessageContainer{
			Message:       randomMessage(),
			Id:            fmt.Sprintf("%v", i),
			Sender:        sender.Username(),
			SenderService: sender.Service(),
		}
	}

	go func() {
		for _, username := range usrs {
			usr := username
			for _, mc := range mcs {
				err := sender.SendMessageToUsers(mc.Message, 1*time.Hour, si.defaultService, usr)
				if err != nil {
					t.Errorf("Error on sending: %v", err)
				}
			}
		}
	}()

	errs := clients.RunAndVerify(mcs...)
	for _, err := range errs {
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
}

func TestSendMessageFromServerToClientsWithSingleRequest(t *testing.T) {
	clearCache()
	defer clearCache()
	si := newServerInfo(9891)

	center, err := si.getMessageCenter()
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}

	go center.Start()
	defer center.Stop()

	nrClients := 100
	clients := &clientMessageVerifier{}
	usrs, err := clients.genClient(nrClients, si)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}

	N := 10
	mcs := make([]*rpc.MessageContainer, N)
	for i := 0; i < N; i++ {
		mcs[i] = &rpc.MessageContainer{
			Message: randomMessage(),
			Id:      fmt.Sprintf("%v", i),
		}
	}

	go func() {
		for _, mc := range mcs {
			req := &rpc.SendRequest{
				ReceiverService: si.defaultService,
				Receivers:       usrs,
				TTL:             1 * time.Hour,
				Message:         mc.Message,
			}
			result := center.Send(req)
			if result.Error != nil {
				t.Errorf("Error on sending: %v", result.Error)
			}
			if result.NrSuccess() != len(usrs) {
				t.Errorf("We got %v success", result.NrSuccess())
			}
		}
	}()

	errs := clients.RunAndVerify(mcs...)
	for _, err := range errs {
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
}
