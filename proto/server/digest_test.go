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
	"github.com/uniqush/uniqush-conn/proto/client"

	"testing"
	"time"
)

func TestSendMessageDigestFromServerToClient(t *testing.T) {
	addr := "127.0.0.1:8088"
	token := "token"
	servConn, cliConn, err := buildServerClientConns(addr, token, 3*time.Second)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer servConn.Close()
	defer cliConn.Close()
	N := 100
	mcs := make([]*proto.MessageContainer, N)

	cache := getCache()
	defer clearCache()
	difields := make(map[string]string, 2)
	difields["df1"] = "df1value"
	difields["df2"] = "df2value"

	difieldNames := []string{"df1", "df2"}
	ttl := 1 * time.Hour

	for i := 0; i < N; i++ {
		mcs[i] = &proto.MessageContainer{
			Message: randomMessage(),
			Id:      fmt.Sprintf("%v", i),
		}
		for k, v := range difields {
			mcs[i].Message.Header[k] = v
		}
		id, err := cache.CacheMessage(servConn.Service(), servConn.Username(), mcs[i], ttl)
		if err != nil {
			t.Errorf("dberror: %v", err)
		}
		mcs[i].Id = id
	}

	servConn.SetMessageCache(cache)
	src := &serverSender{
		conn: servConn,
	}
	dst := &clientReceiver{
		conn: cliConn,
	}

	cliConn.Config(0, 2048, difieldNames...)

	digestChan := make(chan *client.Digest)
	cliConn.SetDigestChannel(digestChan)

	go func() {
		i := 0
		for digest := range digestChan {
			mc := mcs[i]
			i++
			if len(difieldNames) != len(digest.Info) {
				t.Errorf("Error: wrong digest")
			}
			for k, v := range difields {
				if df, ok := digest.Info[k]; ok {
					if df != v {
						t.Errorf("Error: wrong digest value on field %v", k)
					}
				} else {
					t.Errorf("cannot find field %v in the digest", k)
				}
			}
			if mc.Id != digest.MsgId {
				t.Errorf("wrong id: %v != %v", mc.Id, digest.MsgId)
			}
			cliConn.RequestMessage(digest.MsgId)
		}
	}()
	err = iterateOverContainers(src, dst, mcs...)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	close(digestChan)
}
