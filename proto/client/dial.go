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

package client

import (
	"crypto/rsa"
	"errors"
	"github.com/uniqush/uniqush-conn/proto"

	"net"
	"strings"
	"time"
)

var ErrBadServiceOrUserName = errors.New("service name or user name should not contain '\\n' or ':'")

// The conn will be closed if any error occur
func Dial(conn net.Conn, pubkey *rsa.PublicKey, service, username, token string, timeout time.Duration) (c Conn, err error) {
	if strings.Contains(service, "\n") || strings.Contains(username, "\n") ||
		strings.Contains(service, ":") || strings.Contains(username, ":") {
		err = ErrBadServiceOrUserName
		return
	}
	conn.SetDeadline(time.Now().Add(timeout))
	defer func() {
		conn.SetDeadline(time.Time{})
		if err != nil {
			conn.Close()
		}
	}()

	ks, err := proto.ClientKeyExchange(pubkey, conn)
	if err != nil {
		return
	}
	cmdio := ks.ClientCommandIO(conn)

	cmd := new(proto.Command)
	cmd.Type = proto.CMD_AUTH
	cmd.Params = make([]string, 3)
	cmd.Params[0] = service
	cmd.Params[1] = username
	cmd.Params[2] = token

	// don't compress, but encrypt it
	cmdio.WriteCommand(cmd, false)

	cmd, err = cmdio.ReadCommand()
	if err != nil {
		return
	}
	if cmd.Type != proto.CMD_AUTHOK {
		return
	}
	c = NewConn(cmdio, service, username, conn)
	err = nil
	return
}
