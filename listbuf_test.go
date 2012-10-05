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

func TestReadWrite(t *testing.T) {
	lb := NewListBuffer()
	for i := 0; i < 10; i++ {
		buf := []byte{1,2,3,4,5}
		lb.Write(buf)
	}
	buf := make([]byte, 3)
	_, err := lb.Read(buf)
	for err != io.EOF {
		fmt.Println(buf)
		_, err = lb.Read(buf)
	}
}

