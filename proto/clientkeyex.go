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
	"crypto/rsa"
	"crypto/sha256"
	"github.com/monnand/dhkx"
	pss "github.com/monnand/rsa"
	"io"
	"net"
)

func clientKeyExchange(pubKey *rsa.PublicKey, conn net.Conn) (ks *keySet, err error) {
	// Generate a DH key
	group, _ := dhkx.GetGroup(dhGroupID)
	priv, _ := group.GeneratePrivateKey(nil)
	mypub := leftPaddingZero(priv.Bytes(), dhPubkeyLen)

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

	keyExPkt = keyExPkt[:dhPubkeyLen+authKeyLen]
	copy(keyExPkt, mypub)
	err = ks.clientHMAC(keyExPkt[:dhPubkeyLen], keyExPkt[dhPubkeyLen:])
	if err != nil {
		return
	}

	// Send the client message to server, which contains:
	// - Client's DH public key: g ^ y
	// - HMAC of client's DH public key: HMAC(g ^ y, clientAuthKey)
	err = writen(conn, keyExPkt)

	return
}

func Dial(conn net.Conn, pubkey *rsa.PublicKey, service, username, token string) (c Conn, err error) {
	ks, err := clientKeyExchange(pubkey, conn)
	if err != nil {
		return
	}
	cmdio := ks.getClientCommandIO(conn)

	cmd := new(command)
	cmd.Type = cmdtype_AUTH
	cmd.Params = make([]string, 3)
	cmd.Params[0] = service
	cmd.Params[1] = username
	cmd.Params[2] = token

	// don't compress, but encrypt it
	cmdio.WriteCommand(cmd, false, true)

	cmd, err = cmdio.ReadCommand()
	if err != nil {
		return
	}
	if cmd.Type != cmdtype_AUTHOK {
		return
	}
	c = newMessageChannel(cmdio, service, username, conn)
	err = nil
	return
}
