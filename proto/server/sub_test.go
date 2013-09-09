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
	"sync"
	"testing"
	"time"
)

func (a *SubscribeRequest) eq(b *SubscribeRequest) bool {
	if a.Subscribe != b.Subscribe {
		return false
	}
	if a.Service != b.Service {
		return false
	}
	if a.Username != b.Username {
		return false
	}
	if len(a.Params) != len(b.Params) {
		return false
	}
	for k, v := range a.Params {
		if v1, ok := b.Params[k]; ok {
			if v1 != v {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func TestSubscription(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer servConn.Close()
	defer cliConn.Close()
	subChan := make(chan *SubscribeRequest)
	servConn.SetSubscribeRequestChan(subChan)

	params := map[string]string{
		"pushservicetype": "gcm",
		"regid":           "someregid",
	}

	wg := &sync.WaitGroup{}
	go func() {
		err := cliConn.Subscribe(params)
		if err != nil {
			t.Errorf("sub error: %v", err)
		}
		err = cliConn.Unsubscribe(params)
		if err != nil {
			t.Errorf("unsub error: %v", err)
		}
	}()

	go func() {
		servConn.ReceiveMessage()
	}()

	wg.Add(1)
	go func() {
		subreq := <-subChan

		req := &SubscribeRequest{
			Subscribe: true,
			Service:   servConn.Service(),
			Username:  servConn.Username(),
			Params:    params,
		}

		if !req.eq(subreq) {
			t.Errorf("%+v is wrong", subreq)
		}
		subreq = <-subChan
		req.Subscribe = false
		if !req.eq(subreq) {
			t.Errorf("%+v is wrong", subreq)
		}
		wg.Done()
	}()

	wg.Wait()
}
