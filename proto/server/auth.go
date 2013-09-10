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
	"crypto/rsa"
	"errors"
	. "github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/proto"
	"net"
	"strings"
	"time"
)

var ErrAuthFail = errors.New("authentication failed")

// The conn will be closed if any error occur
func AuthConn(conn net.Conn, privkey *rsa.PrivateKey, auth Authenticator, timeout time.Duration) (c Conn, err error) {
	conn.SetDeadline(time.Now().Add(timeout))
	defer func() {
		if err == nil {
			err = conn.SetDeadline(time.Time{})
			if err != nil {
				conn.Close()
			}
		}
	}()

	ks, err := proto.ServerKeyExchange(privkey, conn)
	if err != nil {
		conn.Close()
		return
	}
	cmdio := ks.ServerCommandIO(conn)
	cmd, err := cmdio.ReadCommand()
	if err != nil {
		return
	}
	if cmd.Type != proto.CMD_AUTH {
		err = ErrAuthFail
		return
	}
	if len(cmd.Params) != 3 {
		err = ErrAuthFail
		return
	}
	service := cmd.Params[0]
	username := cmd.Params[1]
	token := cmd.Params[2]

	// Username and service should not contain "\n"
	if strings.Contains(service, "\n") || strings.Contains(username, "\n") ||
		strings.Contains(service, ":") || strings.Contains(username, ":") {
		err = ErrAuthFail
		return
	}

	ok, err := auth.Authenticate(service, username, token, conn.RemoteAddr().String())
	if err != nil {
		return
	}
	if !ok {
		err = ErrAuthFail
		return
	}

	cmd.Type = proto.CMD_AUTHOK
	cmd.Params = nil
	cmd.Message = nil
	err = cmdio.WriteCommand(cmd, false)
	if err != nil {
		return
	}
	c = NewConn(cmdio, service, username, conn)
	err = nil
	return
}
