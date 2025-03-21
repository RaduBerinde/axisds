// Copyright 2025 Radu Berinde.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package axisds

import "testing"

func TestFormatters(t *testing.T) {
	bFmt := MakeBasicFormatter[int]()
	expect(t, bFmt.FormatInterval(1, 5), "[1, 5)")

	eFmt := MakeEndpointFormatter[int](bFmt)

	str := func(start, end Endpoint[int]) string {
		return eFmt.FormatInterval(start, end)
	}
	expect(t, str(MakeEndpoints(1, Inclusive, 5, Inclusive)), "[1, 5]")
	expect(t, str(MakeEndpoints(1, Inclusive, 5, Exclusive)), "[1, 5)")
	expect(t, str(MakeEndpoints(1, Exclusive, 5, Inclusive)), "(1, 5]")
	expect(t, str(MakeEndpoints(1, Exclusive, 5, Exclusive)), "(1, 5)")

	x, y := MakeEndpoints(1, Exclusive, 5, Exclusive)
	expect(t, eFmt.FormatBoundary(x), "1+")
	expect(t, eFmt.FormatBoundary(y), "5")
}

func expect[T comparable](t *testing.T, actual, expected T) {
	if actual != expected {
		t.Helper()
		t.Errorf("expected '%v' got '%v'", expected, actual)
	}
}
