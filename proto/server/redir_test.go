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
	"github.com/uniqush/uniqush-conn/proto/client"

	"testing"
	"time"
)

func TestRedirectCommand(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer servConn.Close()
	defer cliConn.Close()

	go func() {
		servConn.ReceiveMessage()
	}()

	redirChan := make(chan *client.RedirectRequest)
	cliConn.SetRedirectChannel(redirChan)

	addresses := []string{"other-server.mydomain.com:8964", "others.com:8964"}

	go func() {
		cliConn.ReceiveMessage()
	}()

	servConn.Redirect(addresses...)

	go func() {
		for redir := range redirChan {
			if len(redir.Addresses) != len(addresses) {
				t.Errorf("Address length is not same: %v", len(redir.Addresses))
			}

			for i, a := range redir.Addresses {
				if addresses[i] != a {
					t.Errorf("I got a weird address: %v", a)
				}
			}
		}

	}()
	close(redirChan)
	cliConn.Close()
}
