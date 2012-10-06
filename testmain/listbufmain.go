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
	"io"
	"fmt"
	"sync"
	"sync/atomic"
)

func dataProducer(buf *ListBuffer, data []byte, stepSize int, report chan<- error) {
	for stepSize <= len(data) {
		s := stepSize
		if s > len(data) {
			s = len(data)
		}
		n, err := buf.Write(data[:s])
		fmt.Printf("[Producer] Wrote %v bytes of data; %v bytes of data in buffer now\n", n, buf.Size())
		if err != nil && err != ErrFull {
			report <- err
		}
		for err == ErrFull {
			fmt.Println("[Producer] is blocked because of limitation of space. Current data size = ", buf.Size())
			buf.WaitForSpace(s)
			fmt.Println("[Producer] got extra space")
			n, err = buf.Write(data[:s])
			fmt.Printf("[Producer] Wrote %v bytes of data; %v bytes of data in buffer now\n", n, buf.Size())
			if err != nil && err != ErrFull {
				report <- err
			}
		}
		data = data[stepSize:]
	}
	close(report)
}

func dataConsumer(buf *ListBuffer, data []byte, stepSize int, report chan<- error) {
	d := make([]byte, len(data))
	backup := data
	i := d
	for stepSize <= len(data) {
		s := stepSize
		if s > len(data) {
			s = len(data)
		}
		n, err := buf.Read(i[:s])
		fmt.Printf("[Consumer] Read %v bytes of data; %v bytes of data in buffer now\n", n, buf.Size())
		if err != nil && err != io.EOF{
			report <- err
		}
		for err == io.EOF {
			fmt.Println("[Consumer] is blocked because there is no data, Current Data size = ", buf.Size())
			buf.WaitForData()
			fmt.Println("[Consumer] got extra data")
			n, err = buf.Read(i[:s])
			fmt.Printf("[Consumer] Read %v bytes of data; %v bytes of data in buffer now\n", n, buf.Size())
			if err != nil && err != io.EOF {
				report <- err
			}
		}
		data = data[n:]
		i = i[n:]
	}
	for j := 0; j < len(backup); j++ {
		if backup[j] != d[j] {
			report <- fmt.Errorf("@ %v: data[i] = %v; received[i] = %v", j, backup[j], d[j])
		}
	}
	close(report)
}

func waitOnCase(bufSize, dataSize, readStep, writeStep int) int {
	lb := NewListBuffer(bufSize)
	dl := dataSize
	data := make([]byte, dl)

	for i := 0; i < dl; i++ {
		data[i] = byte(i)
	}

	prodErrChan := make(chan error)
	consErrChan := make(chan error)

	go dataProducer(lb, data, writeStep, prodErrChan)

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
	go dataConsumer(lb, data, readStep, consErrChan)

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


func main() {
	testCases := [][]int {
		{10, 100, 5, 10},
	}

	for _, c := range testCases {
		fmt.Println("-------------------------")
		fmt.Printf("Test on %v\n", c)
		err := waitOnCase(c[0], c[1], c[2], c[3])
		if err < 0 {
			fmt.Printf("Error on %v", c)
		}
	}
}


