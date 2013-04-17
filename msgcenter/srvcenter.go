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
	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/evthandler"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/server"
	"strings"
	"time"
)

type eventConnIn struct {
	errChan chan error
	conn    server.Conn
}

type eventConnLeave struct {
	conn server.Conn
	err  error
}

type EventConnError struct {
	Err error
	C   server.Conn
}

func (self *EventConnError) Service() string {
	return self.C.Service()
}

func (self *EventConnError) Username() string {
	return self.C.Username()
}

func (self *EventConnError) Error() string {
	return fmt.Sprintf("[Service=%v][User=%v] %v", self.C.Service(), self.C.Username(), self.Err)
}

type ServiceConfig struct {
	MaxNrConns        int
	MaxNrUsers        int
	MaxNrConnsPerUser int

	MsgCache msgcache.Cache

	LoginHandler          evthandler.LoginHandler
	LogoutHandler         evthandler.LogoutHandler
	MessageHandler        evthandler.MessageHandler
	ForwardRequestHandler evthandler.ForwardRequestHandler
	ErrorHandler          evthandler.ErrorHandler
}

type writeMessageResponse struct {
	err []error
	n   int
}

type writeMessageRequest struct {
	user      string
	msg       *proto.Message
	posterKey string
	ttl       time.Duration
	extra     map[string]string
	resChan   chan<- *writeMessageResponse
}

type serviceCenter struct {
	serviceName string
	config *ServiceConfig
	fwdChan     chan<- *server.ForwardRequest

	writeReqChan chan *writeMessageRequest
	connIn       chan *eventConnIn
	connLeave    chan *eventConnLeave
}

var ErrTooManyConns = errors.New("too many connections")
var ErrInvalidConnType = errors.New("invalid connection type")

func (self *serviceCenter) ReceiveForward(fwdreq *server.ForwardRequest) {
	shouldFwd := false
	if self.config != nil {
		if self.config.ForwardRequestHandler != nil {
			shouldFwd = self.config.ForwardRequestHandler.ShouldForward(fwdreq)
		}
	}
	if !shouldFwd {
		return
	}
	receiver := fwdreq.Receiver
	self.SendMail(receiver, fwdreq.Message, nil, 24 * time.Hour)
}

func (self *serviceCenter) reportError(service, username, connId string, err error) {
	if self.config != nil {
		if self.config.ErrorHandler != nil {
			self.config.ErrorHandler.OnError(service, username, connId, err)
		}
	}
}

func (self *serviceCenter) reportLogin(service, username, connId string) {
	if self.config != nil {
		if self.config.LoginHandler != nil {
			self.config.LoginHandler.OnLogin(service, username, connId)
		}
	}
}

func (self *serviceCenter) reportMessage(connId string, msg *proto.Message) {
	if self.config != nil {
		if self.config.MessageHandler != nil {
			self.config.MessageHandler.OnMessage(connId, msg)
		}
	}
}

func (self *serviceCenter) reportLogout(service, username, connId string, err error) {
	if self.config != nil {
		if self.config.LogoutHandler != nil {
			self.config.LogoutHandler.OnLogout(service, username, connId, err)
		}
	}
}

func (self *serviceCenter) setPoster(service, username, key string, msg *proto.Message, ttl time.Duration) (id string ,err error) {
	if self.config != nil {
		if self.config.MsgCache != nil {
			id, err = self.config.MsgCache.SetPoster(service, username, key, msg, ttl)
		}
	}
	return
}

func (self *serviceCenter) process(maxNrConns, maxNrConnsPerUser, maxNrUsers int) {
	connMap := newTreeBasedConnMap()
	nrConns := 0
	for {
		select {
		case connInEvt := <-self.connIn:
			if maxNrConns > 0 && nrConns >= maxNrConns {
				if connInEvt.errChan != nil {
					connInEvt.errChan <- ErrTooManyConns
				}
				continue
			}
			err := connMap.AddConn(connInEvt.conn, maxNrConnsPerUser, maxNrUsers)
			if err != nil {
				if connInEvt.errChan != nil {
					connInEvt.errChan <- err
				}
				continue
			}
			nrConns++
			if connInEvt.errChan != nil {
				connInEvt.errChan <- nil
			}
		case leaveEvt := <-self.connLeave:
			connMap.DelConn(leaveEvt.conn)
			leaveEvt.conn.Close()
			nrConns--
			conn := leaveEvt.conn
			self.reportLogout(conn.Service(), conn.Username(), conn.UniqId(), leaveEvt.err)
		case wreq := <-self.writeReqChan:
			wres := new(writeMessageResponse)
			wres.n = 0
			conns := connMap.GetConn(wreq.user)
			if len(wreq.posterKey) != 0 && len(conns) > 0 {
				self.setPoster(self.serviceName, wreq.user, wreq.posterKey, wreq.msg, wreq.ttl)
			}
			for _, conn := range conns {
				if conn == nil {
					continue
				}
				var err error
				sconn, ok := conn.(server.Conn)
				if !ok {
					wres.err = append(wres.err, ErrInvalidConnType)
					break
				}
				if len(wreq.posterKey) == 0 {
					_, err = sconn.SendMail(wreq.msg, wreq.extra, wreq.ttl)
				} else {
					_, err = sconn.SendPoster(wreq.msg, wreq.extra, wreq.posterKey, wreq.ttl, false)
				}
				if err != nil {
					wres.err = append(wres.err, err)
					self.reportError(sconn.Service(), sconn.Username(), sconn.UniqId(), err)
					continue
				}
				if sconn.Visible() {
					wres.n++
				}
			}
			if wreq.resChan != nil {
				wreq.resChan <- wres
			}
		}
	}
}

func (self *serviceCenter) SendMail(username string, msg *proto.Message, extra map[string]string, ttl time.Duration) (n int, err []error) {
	req := new(writeMessageRequest)
	ch := make(chan *writeMessageResponse)
	req.msg = msg
	req.posterKey = ""
	req.user = username
	req.ttl = ttl
	req.resChan = ch
	req.extra = extra
	self.writeReqChan <- req
	res := <-ch
	n = res.n
	err = res.err
	return
}

func (self *serviceCenter) SendPoster(username string, msg *proto.Message, extra map[string]string, key string, ttl time.Duration) (n int, err []error) {
	req := new(writeMessageRequest)
	ch := make(chan *writeMessageResponse)
	req.msg = msg
	req.posterKey = key
	req.ttl = ttl
	req.extra = extra
	req.user = username
	req.resChan = ch
	self.writeReqChan <- req
	res := <-ch
	n = res.n
	err = res.err
	return
}

func (self *serviceCenter) serveConn(conn server.Conn) {
	conn.SetForwardRequestChannel(self.fwdChan)
	var err error
	defer func() {
		self.connLeave <- &eventConnLeave{conn: conn, err: err}
	}()
	for {
		var msg *proto.Message
		msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		self.reportMessage(conn.UniqId(), msg)
	}
}

func (self *serviceCenter) NewConn(conn server.Conn) error {
	usr := conn.Username()
	if len(usr) == 0 || strings.Contains(usr, ":") || strings.Contains(usr, "\n") {
		return fmt.Errorf("[Username=%v] Invalid Username")
	}
	evt := new(eventConnIn)
	ch := make(chan error)

	conn.SetMessageCache(self.config.MsgCache)
	evt.conn = conn
	evt.errChan = ch
	self.connIn <- evt
	err := <-ch
	if err == nil {
		go self.serveConn(conn)
		self.reportLogin(conn.Service(), usr, conn.UniqId())
	}
	return err
}

func newServiceCenter(serviceName string, conf *ServiceConfig, fwdChan chan<- *server.ForwardRequest) *serviceCenter {
	ret := new(serviceCenter)
	ret.config = conf
	if ret.config == nil {
		ret.config = new(ServiceConfig)
	}
	ret.serviceName = serviceName
	ret.fwdChan = fwdChan

	ret.connIn = make(chan *eventConnIn)
	ret.connLeave = make(chan *eventConnLeave)
	ret.writeReqChan = make(chan *writeMessageRequest)
	go ret.process(conf.MaxNrConns, conf.MaxNrConnsPerUser, conf.MaxNrUsers)
	return ret
}
