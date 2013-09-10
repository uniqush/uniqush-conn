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
	"crypto/rsa"
	. "github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/proto/client"
	"net"
	"sync"
	"testing"
	"time"
)

type singleUserAuth struct {
	service, username, token string
}

func (self *singleUserAuth) Authenticate(srv, usr, token, addr string) (bool, error) {
	if self.service == srv && self.username == usr && self.token == token {
		return true, nil
	}
	return false, nil
}

func getClient(addr string, priv *rsa.PrivateKey, auth Authenticator, timeout time.Duration) (conn Conn, err error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}
	c, err := ln.Accept()
	if err != nil {
		return
	}
	ln.Close()
	conn, err = AuthConn(c, priv, auth, timeout)
	return
}

func connectServer(addr string, pub *rsa.PublicKey, service, username, token string, timeout time.Duration) (conn client.Conn, err error) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	conn, err = client.Dial(c, pub, service, username, token, timeout)
	return
}

func buildServerClientConns(addr string, token string, timeout time.Duration) (servConn Conn, cliConn client.Conn, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	pub := &priv.PublicKey

	auth := new(singleUserAuth)
	auth.service = "service"
	auth.username = "username"
	auth.token = "token"

	wg := new(sync.WaitGroup)
	wg.Add(2)

	var ec error
	var es error
	go func() {
		servConn, es = getClient(addr, priv, auth, timeout)
		wg.Done()
	}()

	time.Sleep(1 * time.Second)

	go func() {
		cliConn, ec = connectServer(addr, pub, auth.service, auth.username, token, timeout)
		wg.Done()
	}()
	wg.Wait()
	if es != nil {
		err = es
		return
	}
	if ec != nil {
		err = ec
		return
	}
	return
}

func TestAuthOK(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
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

func TestAuthFail(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "wrong token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err == nil {
		t.Errorf("Error: Should be failed")
	}
	if servConn != nil {
		servConn.Close()
	}
	if cliConn != nil {
		cliConn.Close()
	}
}
