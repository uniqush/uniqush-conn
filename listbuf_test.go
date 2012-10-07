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
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

func testReadWrite(size int, writeStep int, readStep int) error {
	lb := NewListBuffer(-1)
	buf := make([]byte, size)
	var i int
	for i = 0; i < len(buf); i++ {
		buf[i] = byte(i)
	}
	for i = 0; i+writeStep < size; i += writeStep {
		_, err := lb.Write(buf[i : i+writeStep])
		if err != nil {
			return err
		}
	}
	if i < size {
		_, err := lb.Write(buf[i:])
		if err != nil {
			return err
		}
	}

	nbuf := make([]byte, size)

	for i = 0; i+readStep < size; i += readStep {
		_, err := lb.Read(nbuf[i : i+readStep])
		/*
			if size <= 100 {
				fmt.Println(nbuf[i : i+readStep])
			}
		*/
		if err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}
	if i < size {
		_, err := lb.Read(nbuf[i:])
		/*
			if size <= 100 {
				fmt.Println(nbuf[i:])
			}
		*/
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
	testCases := [][]int{
		{100, 10, 20},
		{99, 10, 20},
		{100, 20, 10},
		{1024, 100, 10},
		{1024, 10, 10},
	}
	for _, c := range testCases {
		//fmt.Printf("Test on %v\n", c)
		err := testReadWrite(c[0], c[1], c[2])
		if err != nil {
			t.Errorf("Error on %v: %v", c, err)
		}
	}
}

func producer(buf *ListBuffer, data []byte, stepSize int, report chan<- error, silence bool) {
	for len(data) > 0 {
		s := stepSize
		if s > len(data) {
			s = len(data)
		}
		n, err := buf.Write(data[:s])
		if !silence {
			fmt.Printf("[Producer] Wrote %v bytes of data; %v bytes of data in buffer now\n", n, buf.Size())
		}
		if err != nil && err != ErrFull {
			report <- err
		}
		if err == ErrFull {
			if !silence {
				fmt.Println("[Producer] is blocked because of limitation of space. Current data size = ", buf.Size())
			}
			buf.WaitForSpace(s)
			if !silence {
				fmt.Println("[Producer] got extra space")
			}
		}
		data = data[n:]
	}
	close(report)
}

func consumer(buf *ListBuffer, data []byte, stepSize int, report chan<- error, silence bool) {
	d := make([]byte, len(data))
	i := d

	sz := 0
	for len(data) > 0 {
		s := stepSize
		if s > len(data) {
			s = len(data)
		}
		n, err := buf.Read(i[:s])
		if !silence {
			fmt.Printf("[Consumer] Read %v bytes of data; %v bytes of data in buffer now\n", n, buf.Size())
		}
		if err != nil && err != io.EOF {
			report <- err
		}
		if err == io.EOF {
			if !silence {
				fmt.Println("[Consumer] is blocked because there is no data, Current Data size = ", buf.Size())
			}
			buf.WaitForData()
			if !silence {
				fmt.Println("[Consumer] got extra data")
			}

		}

		for j := 0; j < n; j++ {
			if data[j] != i[j] {
				if !silence {
					fmt.Printf("[Consumer] Error: @ %v: data[i] = %v; received[i] = %v\n", sz+j, data[j], i[j])
				}
				report <- fmt.Errorf("@ %v: data[i] = %v; received[i] = %v", sz+j, data[j], i[j])
			}
		}
		data = data[n:]
		i = i[n:]
		sz += n
		if !silence {
			fmt.Printf("[Consumer] %v bytes left\n", len(data))
		}
	}
	close(report)
}

func testWait(bufSize, dataSize, readStep, writeStep int, silence bool) int {
	lb := NewListBuffer(bufSize)
	dl := dataSize
	data := make([]byte, dl)

	for i := 0; i < dl; i++ {
		data[i] = byte(i)
	}

	prodErrChan := make(chan error)
	consErrChan := make(chan error)

	go producer(lb, data, writeStep, prodErrChan, silence)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	var failed int32

	failed = 0
	go func() {
		for e := range prodErrChan {
			fmt.Printf("Producer Error: %v\n", e)
			atomic.AddInt32(&failed, 1)
		}
		wg.Done()
	}()
	go consumer(lb, data, readStep, consErrChan, silence)

	wg.Add(1)
	go func() {
		for e := range consErrChan {
			fmt.Printf("Consumer Error: %v\n", e)
			atomic.AddInt32(&failed, 1)
		}
		wg.Done()
	}()

	wg.Wait()

	if failed == 0 {
		return 0
	}
	return -1
}

func TestWait(t *testing.T) {

	testCases := [][]int{
		{10, 100, 5, 10},
		{10, 100, 10, 5},
		{10, 100, 10, 10},
		{10, 100, 15, 10},
		{555, 1024, 100, 10},
		{123, 1024, 100, 10},
		{1, 2, 2, 2},
	}

	for _, c := range testCases {
		fmt.Println("-------------------------")
		fmt.Printf("Test on %v\n", c)
		err := testWait(c[0], c[1], c[2], c[3], true)
		if err < 0 {
			t.Errorf("Error on %v", c)
		}
	}
}
