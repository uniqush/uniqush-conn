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

import "testing"

func TestXorBytesEq(t *testing.T) {
	a := []byte{0, 2, 1, 3}
	b := []byte{0, 2, 1, 3}

	if !xorBytesEq(a, b) {
		t.Errorf("should be eq")
	}
	if !xorBytesEq(b, a) {
		t.Errorf("should be eq")
	}
	if !xorBytesEq(nil, nil) {
		t.Errorf("should be eq")
	}

	b = []byte{1, 2, 3, 4}
	if xorBytesEq(a, b) {
		t.Errorf("should not be eq")
	}
	if xorBytesEq(b, a) {
		t.Errorf("should not be eq")
	}

	b = []byte{1, 2, 3}
	if xorBytesEq(a, b) {
		t.Errorf("should not be eq")
	}
	if xorBytesEq(b, a) {
		t.Errorf("should not be eq")
	}

	if xorBytesEq(a, nil) {
		t.Errorf("should not be eq")
	}
	if xorBytesEq(b, nil) {
		t.Errorf("should not be eq")
	}
}
