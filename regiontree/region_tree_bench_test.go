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

package regiontree

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"math/rand/v2"
	"testing"
)

func BenchmarkRegionTree(b *testing.B) {
	for _, nBoundaries := range []int{100, 10000} {
		for _, averageRangeLength := range []int{1, 10, 100} {
			b.Run(fmt.Sprintf("boundaries=%d/range-length=%d", nBoundaries, averageRangeLength), func(b *testing.B) {
				b.Run("int-keys", func(b *testing.B) {
					boundaries := make([]int, nBoundaries)
					for i := range boundaries {
						boundaries[i] = i
					}
					benchRegionTree(b, boundaries, cmp.Compare[int], averageRangeLength)
				})

				b.Run("byte-keys", func(b *testing.B) {
					commonPrefix := make([]byte, 500)
					for i := range commonPrefix {
						commonPrefix[i] = 'a' + byte(i%26)
					}
					boundaries := make([][]byte, nBoundaries)
					for i := range boundaries {
						boundaries[i] = append(commonPrefix, []byte(fmt.Sprintf("-%05d", i))...)
					}
					benchRegionTree(b, boundaries, bytes.Compare, averageRangeLength)
				})
			})
		}
	}
}

func benchRegionTree[B any](
	b *testing.B, boundaries []B, cmp func(x, y B) int, averageRangeLength int,
) {
	rt := Make[B, int](cmp, func(p1, p2 int) bool { return p1 == p2 })
	var x int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startIdx := rand.IntN(len(boundaries) - 1)
		endIdx := startIdx + 1 + int(rand.ExpFloat64()*float64(averageRangeLength))
		endIdx = min(endIdx, len(boundaries)-1)
		start, end := boundaries[startIdx], boundaries[endIdx]

		s := rand.IntN(100)
		switch {
		case s < 5:
			// Increment all properties in the range.
			rt.Update(start, end, func(p int) int { return p + 1 })
		case s < 10:
			// Reset all properties in the range.
			rt.Update(start, end, func(p int) int { return 0 })
		default:
			rt.Enumerate(start, end, func(start, end B, prop int) bool {
				x += prop
				return true
			})
		}
	}
	fmt.Fprint(io.Discard, x)
}
