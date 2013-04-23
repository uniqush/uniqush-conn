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
	"errors"
	"hash"
	"io"
)

var ErrZeroEntropy = errors.New("Need more random number")
var ErrImcompatibleProtocol = errors.New("incompatible protocol")
var ErrBadServer = errors.New("Unkown Server")
var ErrCorruptedData = errors.New("corrupted data")
var ErrBadKeyExchangePacket = errors.New("Bad Key-exchange Packet")
var ErrBadPeerImpl = errors.New("bad protocol implementation on peer")

// incCounter increments a four byte, big-endian counter.
func incCounter(c *[4]byte) {
	if c[3]++; c[3] != 0 {
		return
	}
	if c[2]++; c[2] != 0 {
		return
	}
	if c[1]++; c[1] != 0 {
		return
	}
	c[0]++
}

// mgf1XOR XORs the bytes in out with a mask generated using the MGF1 function
// specified in PKCS#1 v2.1.
// out = out xor MGF1(seed, hash)
func mgf1XOR(out []byte, hash hash.Hash, seed []byte) {
	var counter [4]byte
	var digest []byte

	done := 0
	for done < len(out) {
		hash.Write(seed)
		hash.Write(counter[0:4])
		digest = hash.Sum(digest[:0])
		hash.Reset()

		for i := 0; i < len(digest) && done < len(out); i++ {
			out[done] ^= digest[i]
			done++
		}
		incCounter(&counter)
	}
}

func clearBytes(data []byte) {
	for i, _ := range data {
		data[i] = byte(0)
	}
}

func leftPaddingZero(data []byte, l int) []byte {
	if len(data) >= l {
		return data
	}
	ret := make([]byte, l-len(data), l)
	ret = append(ret, data...)
	return ret
}

// copyWithLeftPad copies src to the end of dest, padding with zero bytes as
// needed.
func copyWithLeftPad(dest, src []byte) {
	numPaddingBytes := len(dest) - len(src)
	for i := 0; i < numPaddingBytes; i++ {
		dest[i] = 0
	}
	copy(dest[numPaddingBytes:], src)
}

func bytesEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i, e := range a {
		if b[i] != e {
			return false
		}
	}
	return true
}

func writen(w io.Writer, buf []byte) error {
	n := len(buf)
	for n >= 0 {
		l, err := w.Write(buf)
		if err != nil {
			return err
		}
		if l >= n {
			return nil
		}
		n -= l
		buf = buf[l:]
	}
	return nil
}
