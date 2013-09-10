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
	"sync"

	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/rpc"

	"testing"
	"time"
)

type serverCache struct {
	cache msgcache.Cache
	conn  Conn
}

func (self *serverCache) ProcessMessageContainer(mc *rpc.MessageContainer) error {
	_, err := self.cache.CacheMessage(self.conn.Service(), self.conn.Username(), mc, 1*time.Hour)
	return err
}

func TestRequestAllCachedMessages(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer servConn.Close()
	defer cliConn.Close()

	cache := getCache()
	servConn.SetMessageCache(cache)

	N := 100
	mcs := make([]*rpc.MessageContainer, N)

	for i := 0; i < N; i++ {
		mcs[i] = &rpc.MessageContainer{
			Message: randomMessage(),
			Id:      fmt.Sprintf("%v", i),
		}
		_, err := cache.CacheMessage(servConn.Service(), servConn.Username(), mcs[i], 1*time.Hour)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		cliConn.RequestAllCachedMessages()
		for _, mc := range mcs {
			rmc, err := cliConn.ReceiveMessage()
			if err != nil {
				t.Errorf("Error: %v", err)
			}
			if !rmc.Eq(mc) {
				t.Errorf("corrupted data")
			}
		}
		wg.Done()
	}()

	go func() {
		servConn.ReceiveMessage()
	}()
	wg.Wait()
}
