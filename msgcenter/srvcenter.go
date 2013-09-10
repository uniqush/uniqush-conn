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
	"fmt"
	"github.com/uniqush/uniqush-conn/config"
	"github.com/uniqush/uniqush-conn/proto/server"
	"strings"
)

type serviceCenter struct {
	config     *config.ServiceConfig
	fwdChan    chan<- *server.ForwardRequest
	subReqChan chan<- *server.SubscribeRequest
	conns      connMap
}

func (self *serviceCenter) NewConn(conn server.Conn) error {
	if conn == nil {
		return fmt.Errorf("[Username=%v] Invalid Username")
	}
	usr := conn.Username()
	if len(usr) == 0 || strings.Contains(usr, ":") || strings.Contains(usr, "\n") {
		return fmt.Errorf("[Username=%v] Invalid Username")
	}
	conn.SetMessageCache(self.config.Cache())
	conn.SetForwardRequestChannel(self.fwdChan)
	conn.SetSubscribeRequestChan(self.subReqChan)
	return nil
}

func newServiceCenter(conf *config.ServiceConfig, fwdChan chan<- *server.ForwardRequest, subReqChan chan<- *server.SubscribeRequest) *serviceCenter {
	if conf == nil || fwdChan == nil || subReqChan == nil {
		return nil
	}
	ret := new(serviceCenter)
	ret.config = conf
	ret.conns = newTreeBasedConnMap(conf.MaxNrConns, conf.MaxNrUsers, conf.MaxNrConnsPerUser)
	ret.fwdChan = fwdChan
	ret.subReqChan = subReqChan
	return ret
}
