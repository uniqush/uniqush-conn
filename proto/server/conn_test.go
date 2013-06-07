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
	"github.com/garyburd/redigo/redis"
	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/client"
	"io"
	"sync"
	"testing"
	"time"
)

func getCache() msgcache.Cache {
	db := 1
	c, _ := redis.Dial("tcp", "localhost:6379")
	c.Do("SELECT", db)
	c.Do("FLUSHDB")
	c.Close()
	return msgcache.NewRedisMessageCache("", "", db)
}

func sendTestMessages(s2c, c2s proto.Conn, serverToClient bool, msgs ...*proto.Message) error {
	wg := new(sync.WaitGroup)
	wg.Add(2)

	var src proto.Conn
	var dst proto.Conn

	if serverToClient {
		src = s2c
		dst = c2s
	} else {
		src = c2s
		dst = s2c
	}

	var es error
	var ed error

	go func() {
		defer wg.Done()
		for _, msg := range msgs {
			es = src.WriteMessage(msg, true)
			if es != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		var m *proto.Message
		for _, msg := range msgs {
			m, ed = dst.ReadMessage()
			if ed != nil {
				return
			}
			if msg == nil {
				msg = new(proto.Message)
			}
			msg.Sender = dst.Username()
			msg.SenderService = dst.Service()
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

func TestMessageSendServerToClient(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	N := 100
	msgs := make([]*proto.Message, N)

	for i := 0; i < N; i++ {
		msgs[i] = randomMessage()
	}

	err = sendTestMessages(servConn, cliConn, true, msgs...)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if servConn != nil {
		servConn.Close()
	}
	if cliConn != nil {
		cliConn.Close()
	}
}

func TestMessageSendClientToServer(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	N := 100
	msgs := make([]*proto.Message, N)

	for i := 0; i < N; i++ {
		msgs[i] = randomMessage()
	}

	err = sendTestMessages(servConn, cliConn, false, msgs...)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if servConn != nil {
		servConn.Close()
	}
	if cliConn != nil {
		cliConn.Close()
	}
}

func TestDigestSetting(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive digest
	err = cliConn.Config(0, 512, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	servConn.SetMessageCache(mcache)
	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)
	msg := randomMessage()

	wg := new(sync.WaitGroup)
	wg.Add(2)

	var id string

	// Server:
	go func() {
		var err error
		id, err = servConn.SendMessage(msg, nil, 0*time.Second)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		digest := <-diChan
		if nil == digest {
			t.Errorf("Error: Empty digest")
		}
		cliConn.RequestMessage(digest.MsgId)
		m, err := cliConn.ReadMessage()
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		if m.Id != id {
			t.Errorf("Error: wrong Id: %v; %v", m.Id, m)
		}
		msg.Sender = servConn.Username()
		msg.SenderService = servConn.Service()
		m.Id = ""
		if !msg.Eq(m) {
			t.Errorf("Error: should same: %v != %v", msg, m)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestDigestSettingWithFields(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	fields := []string{"digest"}
	// We always want to receive digest
	err = cliConn.Config(0, 512, fields)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	servConn.SetMessageCache(mcache)
	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)
	msg := randomMessage()
	msg.Header[fields[0]] = "new"

	wg := new(sync.WaitGroup)
	wg.Add(2)

	var id string

	// Server:
	go func() {
		var err error
		id, err = servConn.SendMessage(msg, nil, 0*time.Second)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		digest := <-diChan
		if nil == digest {
			t.Errorf("Error: Empty digest")
		}
		if digest.Info[fields[0]] != msg.Header[fields[0]] {
			t.Errorf("Error: field not match")
		}
		cliConn.RequestMessage(digest.MsgId)
		m, err := cliConn.ReadMessage()
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		if m.Id != id {
			t.Errorf("Error: wrong Id")
		}
		msg.Sender = servConn.Username()
		msg.SenderService = servConn.Service()
		m.Id = ""
		if !msg.Eq(m) {
			t.Errorf("Error: should same: %v != %v", msg, m)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestDigestSettingWithMessageQueue(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive digest
	err = cliConn.Config(0, 512, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	servConn.SetMessageCache(mcache)
	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)
	msg := randomMessage()

	wg := new(sync.WaitGroup)
	wg.Add(2)

	var msgId string

	// Server:
	go func() {
		var err error
		msgId, err = servConn.SendMessage(msg, nil, 0*time.Second)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		digest := <-diChan
		if nil == digest {
			t.Errorf("Error: Empty digest")
		}
		cliConn.RequestMessage(digest.MsgId)
		m, err := cliConn.ReadMessage()
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		msg.Sender = servConn.Username()
		msg.SenderService = servConn.Service()
		if m.Id != msgId {
			t.Errorf("Error: wrong Id")
		}
		m.Id = ""
		if !msg.Eq(m) {
			t.Errorf("Error: should same: %v != %v", msg, m)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestDigestSettingWithMultiMail(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive digest
	err = cliConn.Config(0, 512, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	servConn.SetMessageCache(mcache)
	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)

	N := 10
	msgs := make([]*proto.Message, N)
	for i, _ := range msgs {
		msgs[i] = randomMessage()
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	msgIdMapLock := new(sync.Mutex)
	msgIdMap := make(map[string]*proto.Message, N)

	// Server:
	go func() {
		for _, msg := range msgs {
			msgId, err := servConn.SendMessage(msg, nil, 0*time.Second)
			if err != nil {
				t.Errorf("Error: %v", err)
			}
			msgIdMapLock.Lock()
			msgIdMap[msgId] = msg
			msgIdMapLock.Unlock()
		}
		wg.Done()
	}()

	// Client:
	go func() {
		msgChan := make(chan *proto.Message)
		go func() {
			for {
				m, err := cliConn.ReadMessage()
				if err != nil {
					t.Errorf("Error: %v", err)
				}
				select {
				case msgChan <- m:
				case <-time.After(3 * time.Second):
					return
				}
			}

		}()
		i := 0
		for i < N {
			select {
			case digest := <-diChan:
				if nil == digest {
					t.Errorf("Error: Empty digest")
				}
				cliConn.RequestMessage(digest.MsgId)
			case m := <-msgChan:
				msgIdMapLock.Lock()
				msg := msgIdMap[m.Id]
				msgIdMapLock.Unlock()

				msg.Sender = servConn.Username()
				msg.SenderService = servConn.Service()
				m.Id = ""
				if !msg.Eq(m) {
					t.Errorf("Error: should same: %v != %v", msg, m)
				}
				i++
			}
		}
		wg.Done()

	}()
	wg.Wait()
}

func TestForwardFromServerSameService(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive the message
	err = cliConn.Config(1024, 1024, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	servConn.SetMessageCache(mcache)
	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)
	msg := randomMessage()
	msg.Sender = "random"

	wg := new(sync.WaitGroup)
	wg.Add(2)

	// Server:
	go func() {
		_, err := servConn.SendMessage(msg, nil, 0*time.Second)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		m, err := cliConn.ReadMessage()
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		msg.SenderService = cliConn.Service()
		if !msg.Eq(m) {
			t.Errorf("Error: should same: %v != %v", msg, m)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestForwardFromServerSameServiceWithId(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive digest
	err = cliConn.Config(1024, 1024, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	servConn.SetMessageCache(mcache)
	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)
	msg := randomMessage()
	msg.Sender = "random"
	msg.Id = "randomId"

	wg := new(sync.WaitGroup)
	wg.Add(2)

	// Server:
	go func() {
		_, err := servConn.SendMessage(msg, nil, 0*time.Second)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		m, err := cliConn.ReadMessage()
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		msg.SenderService = cliConn.Service()
		if !msg.Eq(m) {
			t.Errorf("Error: should same: %v != %v", msg, m)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestForwardFromServerDifferentServiceWithId(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive digest
	err = cliConn.Config(1024, 1024, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	servConn.SetMessageCache(mcache)
	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)
	msg := randomMessage()
	msg.Sender = "random"
	msg.SenderService = "randomService"
	msg.Id = "randomId"

	wg := new(sync.WaitGroup)
	wg.Add(2)

	// Server:
	go func() {
		_, err := servConn.SendMessage(msg, nil, 0*time.Second)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		m, err := cliConn.ReadMessage()
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		if !msg.Eq(m) {
			t.Errorf("Error: should same: %v != %v", msg, m)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestSetVisibility(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer servConn.Close()
	defer cliConn.Close()

	v := true
	cliConn.SetVisibility(v)
	time.Sleep(100 * time.Microsecond)
	if servConn.Visible() != v {
		t.Errorf("Not same visibility")
	}

	v = false
	cliConn.SetVisibility(v)
	time.Sleep(100 * time.Microsecond)
	if servConn.Visible() != v {
		t.Errorf("Not same visibility")
	}

}

func TestForwardFromServerDifferentService(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive digest
	err = cliConn.Config(1024, 1024, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	servConn.SetMessageCache(mcache)
	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)
	msg := randomMessage()
	msg.Sender = "random"
	msg.SenderService = "randomService"

	wg := new(sync.WaitGroup)
	wg.Add(2)

	// Server:
	go func() {
		_, err := servConn.SendMessage(msg, nil, 0*time.Second)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		m, err := cliConn.ReadMessage()
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		if !msg.Eq(m) {
			t.Errorf("Error: should same: %v != %v", msg, m)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestForwardRequestDifferentService(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive digest
	err = cliConn.Config(1024, 1024, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	fwdChan := make(chan *ForwardRequest)

	servConn.SetMessageCache(mcache)
	servConn.SetForwardRequestChannel(fwdChan)

	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)

	msg := randomMessage()

	fwd := "random"
	fwdSrv := "randomService"

	wg := new(sync.WaitGroup)
	wg.Add(2)

	// Server:
	go func() {
		fwdreq := <-fwdChan
		if fwdreq.Receiver != fwd {
			t.Errorf("Receiver is not correct: %v", fwdreq)
		}
		if fwdreq.ReceiverService != fwdSrv {
			t.Errorf("Receiver Service is not correct: %v", fwdreq)
		}
		msg.Sender = cliConn.Username()
		msg.SenderService = cliConn.Service()
		if !msg.Eq(fwdreq.Message) {
			t.Errorf("Error: should same: %v != %v", msg, fwdreq.Message)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		err := cliConn.ForwardRequest(fwd, fwdSrv, msg, 24*time.Hour)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestForwardRequestSameService(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	defer servConn.Close()
	defer cliConn.Close()

	// We always want to receive digest
	err = cliConn.Config(1024, 1024, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Wait it to be effect
	time.Sleep(1 * time.Second)
	mcache := getCache()
	fwdChan := make(chan *ForwardRequest)

	servConn.SetMessageCache(mcache)
	servConn.SetForwardRequestChannel(fwdChan)

	diChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(diChan)

	msg := randomMessage()

	fwd := "random"
	fwdSrv := cliConn.Service()

	wg := new(sync.WaitGroup)
	wg.Add(2)

	// Server:
	go func() {
		fwdreq := <-fwdChan
		if fwdreq.Receiver != fwd {
			t.Errorf("Receiver is not correct: %v", fwdreq)
		}
		if fwdreq.ReceiverService != fwdSrv {
			t.Errorf("Receiver Service is not correct: %v", fwdreq)
		}
		msg.Sender = cliConn.Username()
		msg.SenderService = cliConn.Service()
		if !msg.Eq(fwdreq.Message) {
			t.Errorf("Error: should same: %v != %v", msg, fwdreq.Message)
		}
		wg.Done()
	}()

	// Client:
	go func() {
		err := cliConn.ForwardRequest(fwd, fwdSrv, msg, 24*time.Hour)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		wg.Done()
	}()
	wg.Wait()
}
