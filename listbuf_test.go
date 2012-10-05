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

package main

import (
	"testing"
	"io"
	"fmt"
)

func testReadWrite(size int, writeStep int, readStep int) error {
	lb := NewListBuffer(-1, nil)
	buf := make([]byte, size)
	var i int
	for i = 0; i < len(buf); i++ {
		buf[i] = byte(i)
	}
	for i = 0; i + writeStep < size; i += writeStep {
		lb.Write(buf[i:i+writeStep])
	}
	if i < size {
		lb.Write(buf[i:])
	}

	nbuf := make([]byte, size)

	for i = 0; i + readStep < size; i += readStep {
		_, err := lb.Read(nbuf[i:i+readStep])
		if size <= 100 {
			fmt.Println(nbuf[i:i+readStep])
		}
		if err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}
	if i < size {
		_, err := lb.Read(nbuf[i:])
		if size <= 100 {
			fmt.Println(nbuf[i:])
		}
		if err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}

	_, err := lb.Read(nbuf)
	if err != io.EOF {
		return fmt.Errorf("Should be EOF")
	}
	for i = 0; i < size; i++ {
		if nbuf[i] != buf[i] {
			return fmt.Errorf("@ %v: nbuf[i] = %v; buf[i] = %v",
				i, nbuf[i], buf[i])
		}
	}
	return nil
}

func TestReadWrite(t *testing.T) {
	testCases := [][]int {
		{100, 10, 20},
		{99, 10, 20},
		{100, 20, 10},
		{1024, 100, 10},
		{1024, 10, 10},
	}
	for _, c := range testCases {
		fmt.Printf("Test on %v\n", c)
		err := testReadWrite(c[0], c[1], c[2])
		if err != nil {
			t.Errorf("Error on %v: %v", c, err)
		}
	}
}

