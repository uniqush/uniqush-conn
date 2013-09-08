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
	"github.com/uniqush/uniqush-conn/proto"
	"testing"
	"time"
)

func TestSetInvisible(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer servConn.Close()
	defer cliConn.Close()
	N := 10
	mcs := make([]*proto.MessageContainer, N)

	for i := 0; i < N; i++ {
		mcs[i] = &proto.MessageContainer{
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

	cliConn.SetVisibility(false)
	err = iterateOverContainers(src, dst, mcs...)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if servConn.Visible() {
		t.Errorf("Error: should be invisible")
	}
}
