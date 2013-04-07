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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"github.com/monnand/dhkx"
	pss "github.com/monnand/rsa"
	"io"
	"net"
	"strings"
	"time"
)

type Authenticator interface {
	Authenticate(srv, usr, token string) (bool, error)
}

var ErrAuthFail = errors.New("authentication failed")

func AuthConn(conn net.Conn, privkey *rsa.PrivateKey, auth Authenticator, timeout time.Duration) (c Conn, err error) {
	conn.SetDeadline(time.Now().Add(timeout))
	defer conn.SetDeadline(time.Time{})

	ks, err := serverKeyExchange(privkey, conn)
	if err != nil {
		return
	}
	cmdio := ks.getServerCommandIO(conn)
	cmd, err := cmdio.ReadCommand()
	if err != nil {
		return
	}
	if cmd.Type != cmdtype_AUTH {
		return
	}
	if len(cmd.Params) != 3 {
		return
	}
	service := cmd.Params[0]
	username := cmd.Params[1]
	token := cmd.Params[2]

	// Username and service should not contain "\n"
	if strings.Contains(service, "\n") || strings.Contains(username, "\n") {
		err = ErrAuthFail
		return
	}

	ok, err := auth.Authenticate(service, username, token)
	if err != nil {
		return
	}
	if !ok {
		err = ErrAuthFail
		return
	}

	cmd.Type = cmdtype_AUTHOK
	cmd.Params = nil
	cmd.Message = nil
	err = cmdio.WriteCommand(cmd, false, true)
	if err != nil {
		return
	}
	c = newMessageChannel(cmdio, service, username, conn)
	err = nil
	return
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
func serverKeyExchange(privKey *rsa.PrivateKey, conn net.Conn) (ks *keySet, err error) {
	group, _ := dhkx.GetGroup(dhGroupID)
	priv, _ := group.GeneratePrivateKey(nil)

	mypub := priv.Bytes()
	mypub = leftPaddingZero(mypub, dhPubkeyLen)

	salt := make([]byte, pssSaltLen)
	n, err := io.ReadFull(rand.Reader, salt)
	if err != nil || n != len(salt) {
		err = ErrZeroEntropy
		return
	}

	sha := sha256.New()
	hashed := make([]byte, sha.Size())
	sha.Write(mypub)
	hashed = sha.Sum(hashed[:0])

	sig, err := pss.SignPSS(rand.Reader, privKey, crypto.SHA256, hashed, salt)
	if err != nil {
		return
	}

	siglen := (privKey.N.BitLen() + 7) / 8
	keyExPkt := make([]byte, dhPubkeyLen+siglen+nonceLen)
	copy(keyExPkt, mypub)
	copy(keyExPkt[dhPubkeyLen:], sig)
	nonce := keyExPkt[dhPubkeyLen+siglen:]
	n, err = io.ReadFull(rand.Reader, nonce)
	if err != nil || n != len(nonce) {
		err = ErrZeroEntropy
		return
	}

	// Send to client:
	// - DH public key: g ^ x
	// - Signature of DH public key RSASSA-PSS(g ^ x)
	// - nonce
	err = writen(conn, keyExPkt)
	if err != nil {
		return
	}

	// Receive from client:
	// - Client's DH public key: g ^ y
	// - HMAC of client's DH public key: HMAC(g ^ y, clientAuthKey)
	keyExPkt = keyExPkt[:dhPubkeyLen+authKeyLen]

	// Receive the data from client
	n, err = io.ReadFull(conn, keyExPkt)
	if err != nil {
		return
	}
	if n != len(keyExPkt) {
		err = ErrBadKeyExchangePacket
		return
	}

	// First, recover client's DH public key
	clientpub := dhkx.NewPublicKey(keyExPkt[:dhPubkeyLen])

	// Compute a shared key K.
	K, err := group.ComputeKey(clientpub, priv)
	if err != nil {
		return
	}

	// Generate keys from the shared key
	ks, err = generateKeys(K.Bytes(), nonce)
	if err != nil {
		return
	}

	// Check client's hmac
	err = ks.checkClientHMAC(keyExPkt[:dhPubkeyLen], keyExPkt[dhPubkeyLen:])
	if err != nil {
		return
	}
	return
}
