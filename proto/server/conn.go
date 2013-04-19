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
	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/proto"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type SubscribeRequest struct {
	Subscribe bool // false: unsubscribe; true: subscribe
	Service   string
	Username  string
	Params    map[string]string
}

type ForwardRequest struct {
	Receiver        string         `json:"receiver"`
	ReceiverService string         `json:"service"`
	TTL             time.Duration  `json:"ttl"`
	Message         *proto.Message `json:"msg"`
}

type Conn interface {
	// Send the message to client.
	// If the message is larger than the digest threshold,
	// then send a digest to the client and cache the whole message
	// in the .
	SendMail(msg *proto.Message, extra map[string]string, ttl time.Duration) (id string, err error)
	SendPoster(msg *proto.Message, extra map[string]string, key string, ttl time.Duration, setposter bool) (id string, err error)
	SetMessageCache(cache msgcache.Cache)
	SetForwardRequestChannel(fwdChan chan<- *ForwardRequest)
	SetSubscribeRequestChan(subChan chan<- *SubscribeRequest)
	Visible() bool
	proto.Conn
}

type serverConn struct {
	proto.Conn
	cmdio             *proto.CommandIO
	digestThreshold   int32
	compressThreshold int32
	encrypt           int32
	visible           int32
	digestFielsLock   sync.Mutex
	digestFields      []string
	mcache            msgcache.Cache
	fwdChan           chan<- *ForwardRequest
	subChan           chan<- *SubscribeRequest
}

func (self *serverConn) Visible() bool {
	v := atomic.LoadInt32(&self.visible)
	return v > 0
}

func (self *serverConn) SetForwardRequestChannel(fwdChan chan<- *ForwardRequest) {
	self.fwdChan = fwdChan
}

func (self *serverConn) SetSubscribeRequestChan(subChan chan<- *SubscribeRequest) {
}

func (self *serverConn) shouldDigest(msg *proto.Message) (sz int, sendDigest bool) {
	sz = msg.Size()
	d := atomic.LoadInt32(&self.digestThreshold)
	if d >= 0 && d < int32(sz) {
		sendDigest = true
	}
	return
}

func (self *serverConn) writeAutoCompress(msg *proto.Message, sz int) error {
	compress := false
	c := atomic.LoadInt32(&self.compressThreshold)
	if c > 0 && c < int32(sz) {
		compress = true
	}
	encrypt := atomic.LoadInt32(&self.encrypt) > 0
	return self.WriteMessage(msg, compress, encrypt)
}

func (self *serverConn) SendMail(msg *proto.Message, extra map[string]string, ttl time.Duration) (id string, err error) {
	sz, sendDigest := self.shouldDigest(msg)
	if sendDigest {
		id, err = self.mcache.SetMail(self.Service(), self.Username(), msg, ttl)
		if err != nil {
			return
		}
		err = self.writeDigest(msg, extra, sz, id)
		if err != nil {
			return
		}
		return
	}

	// Otherwise, send the message directly
	err = self.writeAutoCompress(msg, sz)
	return
}

func (self *serverConn) SendPoster(msg *proto.Message, extra map[string]string, key string, ttl time.Duration, setposter bool) (id string, err error) {
	sz, sendDigest := self.shouldDigest(msg)
	if sendDigest {
		if len(key) == 0 {
			key = "defaultPoster"
		}
		if setposter {
			id, err = self.mcache.SetPoster(self.Service(), self.Username(), key, msg, ttl)
			if err != nil {
				return
			}
		} else {
			id = self.mcache.PosterId(key)
		}
		err = self.writeDigest(msg, extra, sz, id)
		if err != nil {
			return
		}
		return
	}

	id = ""
	// Otherwise, send the message directly
	err = self.writeAutoCompress(msg, sz)
	return
}

func (self *serverConn) writeDigest(msg *proto.Message, extra map[string]string, sz int, id string) (err error) {
	digest := new(proto.Command)
	digest.Type = proto.CMD_DIGEST
	digest.Params = make([]string, 2)
	digest.Params[0] = fmt.Sprintf("%v", sz)
	digest.Params[1] = id

	dmsg := new(proto.Message)

	header := make(map[string]string, len(extra))

	self.digestFielsLock.Lock()
	defer self.digestFielsLock.Unlock()
	for _, f := range self.digestFields {
		if len(msg.Header) > 0 {
			if v, ok := msg.Header[f]; ok {
				header[f] = v
			}
		}
		if len(extra) > 0 {
			if v, ok := extra[f]; ok {
				header[f] = v
			}
		}
	}
	if len(header) > 0 {
		dmsg.Header = header
		digest.Message = dmsg
	}

	compress := false
	c := atomic.LoadInt32(&self.compressThreshold)
	if c > 0 && c < int32(sz) {
		compress = true
	}
	encrypt := atomic.LoadInt32(&self.encrypt) > 0

	err = self.cmdio.WriteCommand(digest, compress, encrypt)
	if err != nil {
		return
	}
	return
}

func (self *serverConn) ProcessCommand(cmd *proto.Command) (msg *proto.Message, err error) {
	if cmd == nil {
		return
	}
	switch cmd.Type {
	case proto.CMD_SUBSCRIPTION:
		if self.subChan == nil {
			return
		}
		if len(cmd.Params) < 1 {
			err = proto.ErrBadPeerImpl
			return
		}
		if cmd.Message == nil {
			err = proto.ErrBadPeerImpl
			return
		}
		if len(cmd.Message.Header) == 0 {
			err = proto.ErrBadPeerImpl
			return
		}
		sub := true
		if cmd.Params[0] == "0" {
			sub = false
		} else if cmd.Params[0] == "1" {
			sub = true
		} else {
			return
		}
		req := new(SubscribeRequest)
		req.Params = cmd.Message.Header
		req.Service = self.Service()
		req.Username = self.Username()
		req.Subscribe = sub
		self.subChan <- req

	case proto.CMD_SET_VISIBILITY:
		if len(cmd.Params) < 1 {
			err = proto.ErrBadPeerImpl
			return
		}
		var v int32
		v = -1
		if cmd.Params[0] == "0" {
			v = 0
		} else if cmd.Params[0] == "1" {
			v = 1
		}
		if v >= 0 {
			atomic.StoreInt32(&self.visible, v)
		}
	case proto.CMD_FWD_REQ:
		if len(cmd.Params) < 1 {
			err = proto.ErrBadPeerImpl
			return
		}
		if self.fwdChan == nil {
			return
		}
		fwdreq := new(ForwardRequest)
		if cmd.Message == nil {
			cmd.Message = new(proto.Message)
		}
		cmd.Message.Sender = self.Username()
		cmd.Message.SenderService = self.Service()
		fwdreq.Receiver = cmd.Params[0]
		if len(cmd.Params) > 1 {
			fwdreq.ReceiverService = cmd.Params[1]
		} else {
			fwdreq.ReceiverService = self.Service()
		}
		cmd.Message.Id = ""
		fwdreq.Message = cmd.Message
		if cmd.Message != nil && len(cmd.Message.Header) > 0 {
			if ttls, ok := cmd.Message.Header["uniqush.ttl"]; ok {
				fwdreq.TTL, _ = time.ParseDuration(ttls)
			}
		}
		self.fwdChan <- fwdreq
	case proto.CMD_SETTING:
		if len(cmd.Params) < 3 {
			err = proto.ErrBadPeerImpl
			return
		}
		if len(cmd.Params[0]) > 0 {
			var d int
			d, err = strconv.Atoi(cmd.Params[0])
			if err != nil {
				err = proto.ErrBadPeerImpl
				return
			}
			atomic.StoreInt32(&self.digestThreshold, int32(d))

		}
		if len(cmd.Params[1]) > 0 {
			var c int
			c, err = strconv.Atoi(cmd.Params[1])
			if err != nil {
				err = proto.ErrBadPeerImpl
				return
			}
			atomic.StoreInt32(&self.compressThreshold, int32(c))
		}
		if len(cmd.Params[2]) > 0 {
			var e int32
			e = -1
			if cmd.Params[2] == "0" {
				e = 0
			} else if cmd.Params[2] == "1" {
				e = 1
			}
			if e >= 0 {
				atomic.StoreInt32(&self.encrypt, e)
			}
		}
		if len(cmd.Params) > 3 {
			self.digestFielsLock.Lock()
			defer self.digestFielsLock.Unlock()
			self.digestFields = make([]string, len(cmd.Params)-3)
			for i, f := range cmd.Params[3:] {
				self.digestFields[i] = f
			}
		}
	case proto.CMD_MSG_RETRIEVE:
		if len(cmd.Params) < 1 {
			err = proto.ErrBadPeerImpl
			return
		}
		id := cmd.Params[0]

		// If there is no cache, then send an empty message
		if self.mcache == nil {
			m := new(proto.Message)
			m.Id = id
			err = self.writeAutoCompress(m, m.Size())
			return
		}

		var rmsg *proto.Message

		rmsg, err = self.mcache.GetOrDel(self.Service(), self.Username(), id)
		if err != nil {
			return
		}

		if rmsg == nil {
			rmsg = new(proto.Message)
		}
		rmsg.Id = id
		err = self.writeAutoCompress(rmsg, rmsg.Size())
	}
	return
}

func (self *serverConn) SetMessageCache(cache msgcache.Cache) {
	self.mcache = cache
}

func NewConn(cmdio *proto.CommandIO, service, username string, conn net.Conn) Conn {
	sc := new(serverConn)
	sc.cmdio = cmdio
	c := proto.NewConn(cmdio, service, username, conn, sc)
	sc.Conn = c
	sc.digestThreshold = -1
	sc.compressThreshold = 512
	sc.digestFields = make([]string, 0, 10)
	sc.encrypt = 1
	sc.visible = 1
	return sc
}
