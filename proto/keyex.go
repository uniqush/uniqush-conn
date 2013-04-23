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
	"github.com/monnand/dhkx"
	pss "github.com/monnand/rsa"
	"io"
	"net"
)

const currentProtocolVersion byte = 1

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
func ServerKeyExchange(privKey *rsa.PrivateKey, conn net.Conn) (ks *keySet, err error) {
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
	// - Client's version (1 byte)
	// - Client's DH public key: g ^ y
	// - HMAC of client's DH public key: HMAC(g ^ y, clientAuthKey)
	keyExPkt = keyExPkt[:1+dhPubkeyLen+authKeyLen]

	// Receive the data from client
	n, err = io.ReadFull(conn, keyExPkt)
	if err != nil {
		return
	}
	if n != len(keyExPkt) {
		err = ErrBadKeyExchangePacket
		return
	}

	version := keyExPkt[0]
	if version > currentProtocolVersion {
		err = ErrImcompatibleProtocol
		return
	}
	keyExPkt = keyExPkt[1:]

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

func ClientKeyExchange(pubKey *rsa.PublicKey, conn net.Conn) (ks *keySet, err error) {
	// Receive the data from server, which contains:
	// - Server's DH public key: g ^ x
	// - Signature of server's DH public key RSASSA-PSS(g ^ x)
	// - nonce
	siglen := (pubKey.N.BitLen() + 7) / 8
	keyExPkt := make([]byte, dhPubkeyLen+siglen+nonceLen)
	n, err := io.ReadFull(conn, keyExPkt)
	if err != nil {
		return
	}
	if n != len(keyExPkt) {
		err = ErrBadKeyExchangePacket
		return
	}

	serverPubData := keyExPkt[:dhPubkeyLen]
	signature := keyExPkt[dhPubkeyLen : dhPubkeyLen+siglen]
	nonce := keyExPkt[dhPubkeyLen+siglen:]

	sha := sha256.New()
	hashed := make([]byte, sha.Size())
	sha.Write(serverPubData)
	hashed = sha.Sum(hashed[:0])

	// Verify the signature
	err = pss.VerifyPSS(pubKey, crypto.SHA256, hashed, signature, pssSaltLen)

	if err != nil {
		return
	}

	// Generate a DH key
	group, _ := dhkx.GetGroup(dhGroupID)
	priv, _ := group.GeneratePrivateKey(nil)
	mypub := leftPaddingZero(priv.Bytes(), dhPubkeyLen)

	// Generate the shared key from server's DH public key and client DH private key
	serverpub := dhkx.NewPublicKey(serverPubData)
	K, err := group.ComputeKey(serverpub, priv)
	if err != nil {
		return
	}

	ks, err = generateKeys(K.Bytes(), nonce)
	if err != nil {
		return
	}

	keyExPkt = keyExPkt[:1+dhPubkeyLen+authKeyLen]
	keyExPkt[0] = currentProtocolVersion
	copy(keyExPkt[1:], mypub)
	err = ks.clientHMAC(keyExPkt[1:dhPubkeyLen+1], keyExPkt[dhPubkeyLen+1:])
	if err != nil {
		return
	}

	// Send the client message to server, which contains:
	// - Protocol version (1 byte)
	// - Client's DH public key: g ^ y
	// - HMAC of client's DH public key: HMAC(g ^ y, clientAuthKey)
	err = writen(conn, keyExPkt)

	return
}
