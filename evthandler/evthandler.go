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

package evthandler

import (
	"github.com/uniqush/uniqush-conn/rpc"
	"time"
)

type Authenticator interface {
	Authenticate(srv, usr, connId, token, addr string) (bool, []string, error)
}

type LoginHandler interface {
	OnLogin(service, username, connId, addr string)
}

type LogoutHandler interface {
	OnLogout(service, username, connId, addr string, reason error)
}

type MessageHandler interface {
	OnMessage(service, username, connId string, msg *rpc.Message)
}

type ForwardRequestHandler interface {
	ShouldForward(senderService, sender, receiverService string, receivers []string, ttl time.Duration, msg *rpc.Message) (shouldForward bool, shouldPush bool, pushInfo map[string]string)
	MaxTTL() time.Duration
}

type ErrorHandler interface {
	OnError(service, username, connId, addr string, err error)
}

type SubscribeHandler interface {
	ShouldSubscribe(service, username string, info map[string]string) bool
}

type UnsubscribeHandler interface {
	OnUnsubscribe(service, username string, info map[string]string)
}
