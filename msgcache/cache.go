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
	"fmt"
	"github.com/uniqush/uniqush-conn/rpc"
	"math/rand"
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
	Init(addr, username, password, database string) error
	GetCache(addr, username, password, database string) (Cache, error)
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

func GetCache(engine, addr, username, password, database string) (Cache, error) {
	cacheEngineMapLock.Lock()
	defer cacheEngineMapLock.Unlock()
	if c, ok := cacheEngineMap[engine]; ok {
		err := c.Init(addr, username, password, database)
		if err != nil {
			return nil, err
		}
		return c.GetCache(addr, username, password, database)
	}
	return nil, fmt.Errorf("%v is not supported", engine)
}

func randomId() string {
	return fmt.Sprintf("%x-%x", time.Now().Unix(), rand.Int63())
}
