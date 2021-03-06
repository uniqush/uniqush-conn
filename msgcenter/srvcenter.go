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
	"errors"
	"fmt"
	"github.com/uniqush/uniqush-conn/config"
	"github.com/uniqush/uniqush-conn/proto/server"
	"github.com/uniqush/uniqush-conn/rpc"
	"io"
	"strings"
)

type serviceCenter struct {
	config     *config.ServiceConfig
	fwdChan    chan<- *rpc.ForwardRequest
	subReqChan chan *rpc.SubscribeRequest
	conns      connMap
	peer       rpc.UniqushConnPeer
}

func (self *serviceCenter) Stop() {
	self.conns.CloseAll()
	close(self.subReqChan)
}

func (self *serviceCenter) serveConn(conn server.Conn) {
	var reason error
	defer func() {
		self.config.OnLogout(conn, reason)
		self.conns.DelConn(conn)
		conn.Close()
	}()
	for {
		msg, err := conn.ReceiveMessage()
		if err != nil {
			if err != io.EOF {
				self.config.OnError(conn, err)
				reason = err
			}
			return
		}
		if msg != nil {
			self.config.OnMessage(conn, msg)
		}
	}
}

func (self *serviceCenter) NewConn(conn server.Conn) {
	if conn == nil {
		//self.config.OnError(conn, fmt.Errorf("Nil conn")
		return
	}
	usr := conn.Username()
	if len(usr) == 0 || strings.Contains(usr, ":") || strings.Contains(usr, "\n") {
		self.config.OnError(conn, fmt.Errorf("invalid username"))
		conn.Close()
		return
	}
	conn.SetMessageCache(self.config.Cache())
	conn.SetForwardRequestChannel(self.fwdChan)
	conn.SetSubscribeRequestChan(self.subReqChan)
	err := self.conns.AddConn(conn)
	if err != nil {
		self.config.OnError(conn, err)
		conn.Close()
		return
	}

	self.config.OnLogin(conn)
	go self.serveConn(conn)
	return
}

func (self *serviceCenter) Send(req *rpc.SendRequest) *rpc.Result {
	ret := new(rpc.Result)

	if req == nil {
		ret.SetError(fmt.Errorf("invalid request"))
		return ret
	}
	if req.Message == nil || req.Message.IsEmpty() {
		ret.SetError(fmt.Errorf("invalid request: empty message"))
		return ret
	}
	if len(req.Receivers) == 0 {
		ret.SetError(fmt.Errorf("invalid request: no receiver"))
		return ret
	}

	shouldPush := !req.DontPush
	shouldCache := !req.DontCache
	shouldPropagate := !req.DontPropagate
	receivers := req.Receivers

	for _, recver := range receivers {
		mid := req.Id
		msg := req.Message

		if shouldCache {
			mc := &rpc.MessageContainer{
				Sender:        "",
				SenderService: "",
				Message:       msg,
			}
			var err error
			mid, err = self.config.CacheMessage(recver, mc, req.TTL)
			if err != nil {
				ret.SetError(err)
				return ret
			}
		}

		if len(mid) == 0 {
			ret.SetError(fmt.Errorf("undefined message Id"))
			return ret
		}

		n := 0

		conns := self.conns.GetConn(recver)
		conns.Traverse(func(conn server.Conn) error {
			err := conn.SendMessage(msg, mid, nil, !req.NeverDigest)
			ret.Append(conn, err)
			if err != nil {
				conn.Close()
				// We won't delete this connection here.
				// Instead, we close it and let the reader
				// goroutine detect the error and close it.
			} else {
				n++
			}
			// Instead of returning an error,
			// we wourld rather let the Traverse() move forward.
			return nil
		})

		// Don't propagate this request to other instances in the cluster.
		req.DontPropagate = true
		// Don't push the message. We will push it on this node.
		req.DontPush = true
		// Don't cache the message. We have already cached it.
		req.DontCache = true
		req.Id = mid
		req.Receivers = []string{recver}

		if shouldPropagate {
			r := self.peer.Send(req)
			n += r.NrSuccess()
			ret.Join(r)
		}

		if n == 0 && shouldPush {
			self.config.Push(recver, "", "", req.PushInfo, mid, msg.Size())
		}
	}
	return ret
}

func (self *serviceCenter) Forward(req *rpc.ForwardRequest) *rpc.Result {
	ret := new(rpc.Result)

	if req == nil {
		ret.SetError(fmt.Errorf("invalid request"))
		return ret
	}
	if req.Message == nil || req.Message.IsEmpty() {
		ret.SetError(fmt.Errorf("invalid request: empty message"))
		return ret
	}
	if len(req.Receivers) == 0 {
		ret.SetError(fmt.Errorf("invalid request: no receiver"))
		return ret
	}

	mid := req.Id
	msg := req.Message
	mc := &req.MessageContainer

	var pushInfo map[string]string
	var shouldForward bool
	shouldPush := !req.DontPush
	shouldCache := !req.DontCache
	shouldPropagate := !req.DontPropagate

	if !req.DontAsk {
		// We need to ask for permission to forward this message.
		// This means the forward request is generated directly from a user,
		// not from a uniqush-conn node in a cluster.

		mc.Id = ""
		shouldForward, shouldPush, pushInfo = self.config.ShouldForward(req)

		if !shouldForward {
			return nil
		}
	}

	receivers := req.Receivers

	for _, recver := range receivers {
		if shouldCache {
			var err error
			mid, err = self.config.CacheMessage(recver, mc, req.TTL)
			if err != nil {
				ret.SetError(err)
				return ret
			}
		}

		if len(mid) == 0 {
			ret.SetError(fmt.Errorf("undefined message Id"))
			return ret
		}

		n := 0

		conns := self.conns.GetConn(recver)
		conns.Traverse(func(conn server.Conn) error {
			err := conn.ForwardMessage(req.Sender, req.SenderService, msg, mid, !req.NeverDigest)
			ret.Append(conn, err)
			if err != nil {
				conn.Close()
				// We won't delete this connection here.
				// Instead, we close it and let the reader
				// goroutine detect the error and close it.
			} else {
				n++
			}
			// Instead of returning an error,
			// we wourld rather let the Traverse() move forward.
			return nil
		})

		// forward the message if possible,
		// Don't propagate this request to other instances in the cluster.
		req.DontPropagate = true
		// Don't ask the permission to forward (we have already got the permission)
		req.DontAsk = true
		// And don't push the message. We will push it on this node.
		req.DontPush = true
		// Dont' cache it
		req.DontCache = true
		req.Id = mid
		req.Receivers = []string{recver}

		if shouldPropagate {
			r := self.peer.Forward(req)
			n += r.NrSuccess()
			ret.Join(r)
		}

		if n == 0 && shouldPush {
			self.config.Push(recver, req.SenderService, req.Sender, pushInfo, mid, msg.Size())
		}
	}
	return ret
}

func (self *serviceCenter) Redirect(req *rpc.RedirectRequest) *rpc.Result {
	conns := self.conns.GetConn(req.Receiver)
	var sc server.Conn
	result := new(rpc.Result)
	conns.Traverse(func(conn server.Conn) error {
		if len(req.ConnId) == 0 || conn.UniqId() == req.ConnId {
			sc = conn
			result.Append(sc, nil)
			return errors.New("done")
		}
		return nil
	})
	if sc != nil {
		// self.conns.DelConn(sc)
		sc.Redirect(req.CandidateSersers...)
		sc.Close()
		return result
	}

	if req.DontPropagate {
		return result
	}
	req.DontPropagate = true
	return self.peer.Redirect(req)
}

func (self *serviceCenter) CheckUserStatus(req *rpc.UserStatusQuery) *rpc.Result {
	conns := self.conns.GetConn(req.Username)
	result := new(rpc.Result)
	conns.Traverse(func(conn server.Conn) error {
		result.Append(conn, nil)
		return nil
	})
	if req.DontPropagate {
		return result
	}
	req.DontPropagate = true
	r := self.peer.CheckUserStatus(req)
	result.Join(r)
	return result
}

func (self *serviceCenter) processSubscription() {
	for req := range self.subReqChan {
		if req == nil {
			return
		}
		go self.config.Subscribe(req)
	}
}

func (self *serviceCenter) NrConns() int {
	return self.conns.NrConns()
}

func (self *serviceCenter) NrUsers() int {
	return self.conns.NrUsers()
}

func (self *serviceCenter) AllUsernames() []string {
	return self.conns.AllUsernames()
}

func newServiceCenter(conf *config.ServiceConfig, fwdChan chan<- *rpc.ForwardRequest, peer rpc.UniqushConnPeer) *serviceCenter {
	if conf == nil || fwdChan == nil {
		return nil
	}
	ret := new(serviceCenter)
	ret.config = conf
	ret.conns = newTreeBasedConnMap(conf.MaxNrConns, conf.MaxNrUsers, conf.MaxNrConnsPerUser)
	ret.fwdChan = fwdChan
	ret.subReqChan = make(chan *rpc.SubscribeRequest)
	ret.peer = peer

	go ret.processSubscription()
	return ret
}
