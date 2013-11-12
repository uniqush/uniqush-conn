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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	. "github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/proto"
	"io"
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
		err = fmt.Errorf("invalid command type")
		return
	}
	if len(cmd.Params) != 3 {
		err = fmt.Errorf("invalid parameters")
		return
	}
	service := cmd.Params[0]
	username := cmd.Params[1]
	token := cmd.Params[2]

	// Username and service should not contain "\n", ":", ","
	if strings.Contains(service, "\n") || strings.Contains(username, "\n") ||
		strings.Contains(service, ":") || strings.Contains(username, ":") ||
		strings.Contains(service, ",") || strings.Contains(username, ",") {
		err = fmt.Errorf("invalid service name or username")
		return
	}

	var d [16]byte
	io.ReadFull(rand.Reader, d[:])
	connId := fmt.Sprintf("%x-%v", time.Now().Unix(), base64.URLEncoding.EncodeToString(d[:]))
	ok, redir, err := auth.Authenticate(service, username, connId, token, conn.RemoteAddr().String())
	if err != nil {
		return
	}
	if len(redir) > 0 {
		cmd.Type = proto.CMD_REDIRECT
		cmd.Params = redir
		cmd.Message = nil
	} else if !ok {
		err = ErrAuthFail
		return
	} else {
		cmd.Type = proto.CMD_AUTHOK
		cmd.Params = nil
		cmd.Message = nil
		cmd.Randomize()
	}
	err = cmdio.WriteCommand(cmd, false)
	if err != nil {
		return
	}

	if len(redir) > 0 {
		c = nil
		err = io.EOF
	} else {
		c = NewConn(cmdio, service, username, connId, conn)
		err = nil
	}
	return
}
