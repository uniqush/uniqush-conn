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

package proto

import (
	"net"
	"io"
	"crypto/sha256"
	"crypto/rsa"
	"crypto/rand"
	"crypto"
	pss "github.com/monnand/rsa"
	"github.com/monnand/dhkx"
)

type Authenticator interface {
	Authenticate(usr, token string) (bool, error)
}

type serverListener struct {
	listener net.Listener
	auth Authenticator
	privKey *rsa.PrivateKey
}

func Listen(listener net.Listener, auth Authenticator, privKey *rsa.PrivateKey) (l net.Listener, err error) {
	ret := new(serverListener)
	ret.listener = listener
	ret.auth = auth
	ret.privKey = privKey
	l = ret
	return
}

func (self *serverListener) Addr() net.Addr {
	return self.listener.Addr()
}

func (self *serverListener) Close() error {
	err := self.listener.Close()
	return err
}

// The authentication here is quite similar with, if not same as, tarsnap's auth algorithm.
//
// First, server generate a Diffie-Hellman public key, dhpub1, sign it with 
// server's private key using RSASSA-PSS signing algorithm.
// Send dhpub1, its signature and a nonce to client.
// An nonce is just a sequence of random bytes.
//
// Server -- dhpub1 + sign(dhpub1) + nonce --> Client
//
// Then client generate its own Diffie-Hellman key. It now can calculate
// a key, K, using its own Diffie-Hellman key and server's DH public key.
// (According to DH key exchange algorithm)
//
// Now, we can use K to derive any key we need on server and client side.
// master key, mkey = MGF1(nonce || K, 48)
//
//
func (self *serverListener) serverAuthenticate(conn net.Conn) *authResult {
	ret := new(authResult)

	group, _ := dhkx.GetGroup(dhGroupID)
	priv, _ := group.GeneratePrivateKey(nil)

	mypub := priv.Bytes()
	mypub = leftPaddingZero(mypub, dhPubkeyLen)

	salt := make([]byte, pssSaltLen)
	n, err := io.ReadFull(rand.Reader, salt)
	if err != nil || n != len(salt) {
		ret.err = ErrZeroEntropy
		return ret
	}

	sha := sha256.New()
	hashed := make([]byte, sha.Size())
	sha.Write(mypub)
	hashed = sha.Sum(hashed[:0])

	sig, err := pss.SignPSS(rand.Reader, self.privKey, crypto.SHA256, hashed, salt)
	if err != nil {
		ret.err = err
		return ret
	}

	siglen := (self.privKey.N.BitLen() + 7) / 8
	keyExPkt := make([]byte, dhPubkeyLen + siglen + nonceLen)
	copy(keyExPkt, mypub)
	copy(keyExPkt[dhPubkeyLen:], sig)
	nonce := keyExPkt[dhPubkeyLen + siglen:]
	n, err = io.ReadFull(rand.Reader, nonce)
	if err != nil || n != len(nonce) {
		ret.err = ErrZeroEntropy
		return ret
	}

	// Send to client:
	// - DH public key: g ^ x
	// - Signature of DH public key RSASSA-PSS(g ^ x)
	// - nonce
	ret.err = writen(conn, keyExPkt)
	if ret.err != nil {
		return ret
	}

	// Receive from client:
	// - Client's DH public key: g ^ y
	// - HMAC of client's DH public key: HMAC(g ^ y, clientAuthKey)
	keyExPkt = keyExPkt[:dhPubkeyLen + authKeyLen]

	// Receive the data from client
	n, err = io.ReadFull(conn, keyExPkt)
	if err != nil {
		ret.err = err
		return ret
	}
	if n != len(keyExPkt) {
		ret.err = ErrBadKeyExchangePacket
		return ret
	}

	// First, recover client's DH public key
	clientpub := dhkx.NewPublicKey(keyExPkt[:dhPubkeyLen])

	// Compute a shared key K.
	K, err := group.ComputeKey(clientpub, priv)
	if err != nil {
		ret.err = err
		return ret
	}

	// Generate keys from the shared key
	ret.ks, err = generateKeys(K.Bytes(), nonce)
	if err != nil {
		ret.err = err
		return ret
	}

	// Check client's hmac
	ret.err = ret.ks.checkClientHMAC(keyExPkt[:dhPubkeyLen], keyExPkt[dhPubkeyLen:])
	if ret.err != nil {
		return ret
	}

	// TODO username/password auth
	ret.conn = conn
	return ret
}

func (self *serverListener) Accept() (conn net.Conn, err error) {
	c, err := self.listener.Accept()
	if err != nil {
		return
	}

	res := self.serverAuthenticate(c)

	if res.err != nil {
		err = res.err
		return
	}
	conn = res.conn
	return
}

