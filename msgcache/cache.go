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

package msgcache

import (
	"crypto/rand"
	"encoding/base64"

	"fmt"
	"github.com/uniqush/uniqush-conn/rpc"
	"io"
	"sync"
	"time"
)

func init() {
	Register(&mysqlCacheManager{})
	Register(&redisCacheManager{})
}

type Cache interface {
	CacheMessage(service, username string, msg *rpc.MessageContainer, ttl time.Duration) (id string, err error)
	// XXX Is there any better way to support retrieve all feature?
	Get(service, username, id string) (msg *rpc.MessageContainer, err error)
	RetrieveAllSince(service, username string, since time.Time) (msgs []*rpc.MessageContainer, err error)
}

type CacheManager interface {
	GetCache(host, username, password, database string, port int) (Cache, error)
	Engine() string
}

var cacheEngineMapLock sync.Mutex
var cacheEngineMap map[string]CacheManager

func Register(cm CacheManager) {
	cacheEngineMapLock.Lock()
	defer cacheEngineMapLock.Unlock()
	if cacheEngineMap == nil {
		cacheEngineMap = make(map[string]CacheManager, 10)
	}
	cacheEngineMap[cm.Engine()] = cm
}

func GetCache(engine, host, username, password, database string, port int) (Cache, error) {
	cacheEngineMapLock.Lock()
	defer cacheEngineMapLock.Unlock()
	if c, ok := cacheEngineMap[engine]; ok {
		return c.GetCache(host, username, password, database, port)
	}
	return nil, fmt.Errorf("%v is not supported", engine)
}

func randomId() string {
	var d [8]byte
	io.ReadFull(rand.Reader, d[:])
	return fmt.Sprintf("%x-%v", time.Now().Unix(), base64.URLEncoding.EncodeToString(d[:]))
}
