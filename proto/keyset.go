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

package proto

import (
	"crypto/hmac"
	"crypto/sha256"
	"io"
)

type keySet struct {
	serverEncrKey []byte
	serverAuthKey []byte
	clientEncrKey []byte
	clientAuthKey []byte
}

func newKeySet(serverEncrKey, serverAuthKey, clientEncrKey, clientAuthKey []byte) *keySet {
	result := new(keySet)

	result.serverEncrKey = serverEncrKey
	result.serverAuthKey = serverAuthKey
	result.clientEncrKey = clientEncrKey
	result.clientAuthKey = clientAuthKey

	return result
}

func (self *keySet) serverHMAC(data, mac []byte) error {
	hash := hmac.New(sha256.New, self.serverAuthKey)
	err := writen(hash, data)
	if err != nil {
		return err
	}
	mac = hash.Sum(mac[:0])
	return nil
}

func (self *keySet) checkServerHMAC(data, mac []byte) error {
	if len(mac) != authKeyLen {
		return ErrCorruptedData
	}
	hmac := make([]byte, len(mac))
	err := self.serverHMAC(data, hmac)
	if err != nil {
		return err
	}
	if !bytesEq(hmac, mac) {
		return ErrCorruptedData
	}
	return nil
}

func (self *keySet) getClientCommandIO(conn io.ReadWriter) *commandIO {
	ret := newCommandIO(self.clientEncrKey, self.clientAuthKey, self.serverEncrKey, self.serverAuthKey, conn)
	return ret
}

func (self *keySet) getServerCommandIO(conn io.ReadWriter) *commandIO {
	ret := newCommandIO(self.serverEncrKey, self.serverAuthKey, self.clientEncrKey, self.clientAuthKey, conn)
	return ret
}

func (self *keySet) clientHMAC(data, mac []byte) error {
	hash := hmac.New(sha256.New, self.clientAuthKey)
	err := writen(hash, data)
	if err != nil {
		return err
	}
	mac = hash.Sum(mac[:0])
	return nil
}

func (self *keySet) checkClientHMAC(data, mac []byte) error {
	if len(mac) != authKeyLen {
		return ErrCorruptedData
	}
	hmac := make([]byte, len(mac))
	err := self.clientHMAC(data, hmac)
	if err != nil {
		return err
	}
	if !bytesEq(hmac, mac) {
		return ErrCorruptedData
	}
	return nil
}

func generateKeys(k, nonce []byte) (ks *keySet, err error) {
	mkey := make([]byte, 48)
	mgf1XOR(mkey, sha256.New(), append(k, nonce...))

	h := hmac.New(sha256.New, mkey)

	serverEncrKey := make([]byte, encrKeyLen)
	h.Write([]byte("ServerEncr"))
	serverEncrKey = h.Sum(serverEncrKey[:0])
	h.Reset()

	serverAuthKey := make([]byte, authKeyLen)
	h.Write([]byte("ServerAuth"))
	serverAuthKey = h.Sum(serverAuthKey[:0])
	h.Reset()

	clientEncrKey := make([]byte, encrKeyLen)
	h.Write([]byte("ClientEncr"))
	clientEncrKey = h.Sum(clientEncrKey[:0])
	h.Reset()

	clientAuthKey := make([]byte, authKeyLen)
	h.Write([]byte("ClientAuth"))
	clientAuthKey = h.Sum(clientAuthKey[:0])
	h.Reset()

	ks = newKeySet(serverEncrKey, serverAuthKey, clientEncrKey, clientAuthKey)
	return
}
