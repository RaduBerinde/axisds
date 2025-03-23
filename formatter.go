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

import "fmt"

// Formatter is an interface for formatting boundaries and intervals.
type Formatter[B Boundary] interface {
	// FormatBoundary formats a "bare" boundary. Used by Endpoint[B].
	FormatBoundary(b B) string
	// FormatInterval formats an interval with the given boundaries.
	FormatInterval(start, end B) string
}

// MakeBasicFormatter creates a Formatter[B] that uses the `%v` format for the
// boundaries.
func MakeBasicFormatter[B Boundary]() Formatter[B] {
	return basicFormatter[B]{}
}

// MakeEndpointFormatter creates a Formatter[Endpoint[B]].
func MakeEndpointFormatter[B Boundary](bFmt Formatter[B]) Formatter[Endpoint[B]] {
	return &endpointFormatter[B]{bFmt: bFmt}
}

type basicFormatter[B Boundary] struct{}

func (basicFormatter[B]) FormatBoundary(b B) string {
	return fmt.Sprint(b)
}

var _ Formatter[int] = basicFormatter[int]{}

func (basicFormatter[B]) FormatInterval(start, end B) string {
	return fmt.Sprintf("[%v, %v)", start, end)
}

type endpointFormatter[B Boundary] struct {
	bFmt Formatter[B]
}

var _ Formatter[Endpoint[int]] = &endpointFormatter[int]{}

func (f *endpointFormatter[B]) FormatBoundary(e Endpoint[B]) string {
	s := f.bFmt.FormatBoundary(e.B)
	if e.PlusEpsilon {
		s += "+"
	}
	return s
}
func (f *endpointFormatter[B]) FormatInterval(start, end Endpoint[B]) string {
	c1, c2 := '[', ')'
	if start.PlusEpsilon {
		c1 = '('
	}
	if end.PlusEpsilon {
		c2 = ']'
	}
	return fmt.Sprintf("%c%s, %s%c", c1, f.bFmt.FormatBoundary(start.B), f.bFmt.FormatBoundary(end.B), c2)
}
