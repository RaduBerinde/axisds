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
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/RaduBerinde/axisds"
	"github.com/cockroachdb/datadriven"
)

const debug = false

func TestDataDriven(t *testing.T) {
	t.Run("ints", func(t *testing.T) {
		testDataDriven(t, "testdata/ints", cmp.Compare[int], axisds.MakeBasicFormatter[int](), axisds.MakeBasicParser[int]())
	})
	t.Run("endpoints-ints", func(t *testing.T) {
		testDataDriven(
			t, "testdata/endpoints-ints",
			axisds.EndpointCompareFn(cmp.Compare[int]),
			axisds.MakeEndpointFormatter(axisds.MakeBasicFormatter[int]()),
			axisds.MakeEndpointParser(axisds.MakeBasicParser[int]()),
		)
	})
}

func testDataDriven[B Boundary](
	t *testing.T, path string, cmpFn func(a, b B) int, f axisds.Formatter[B], p axisds.Parser[B],
) {
	// lowWatermark is a value that we can increase which makes any value <
	// lowWatermark be equivalent to 0.
	lowWatermark := -100000
	rt := Make[B, int](cmpFn, func(a, b int) bool {
		if a < lowWatermark && b < lowWatermark {
			return true
		}
		return a == b
	})
	datadriven.RunTest(t, path, func(t *testing.T, td *datadriven.TestData) string {
		var buf strings.Builder
		switch td.Cmd {
		case "add":
			for _, l := range strings.Split(strings.TrimSpace(td.Input), "\n") {
				start, end, rem := axisds.MustParseIntervalPrefix(p, l)
				var val int
				if _, err := fmt.Sscanf(rem, "%d", &val); err != nil {
					td.Fatalf(t, "invalid input %q: %v", l, err)
				}
				rt.Update(start, end, func(v int) int { return v + val })
			}

		case "zero":
			for _, l := range strings.Split(strings.TrimSpace(td.Input), "\n") {
				start, end := axisds.MustParseInterval(p, l)
				rt.Update(start, end, func(v int) int { return 0 })
			}

		case "watermark":
			var w int
			td.ScanArgs(t, "w", &w)
			if w <= lowWatermark {
				td.Fatalf(t, "watermark must be increasing")
			}
			lowWatermark = w

		default:
			td.Fatalf(t, "unknown command: %q", td.Cmd)
		}
		buf.WriteString("regions:\n")
		for _, l := range strings.Split(strings.TrimSpace(rt.String(f)), "\n") {
			fmt.Fprintf(&buf, "  %s\n", l)
		}
		return buf.String()
	})
}

func TestRegionTreeRand(t *testing.T) {
	for test := 0; test < 100; test++ {
		var log bytes.Buffer
		seed := rand.Uint64()
		rng := rand.New(rand.NewPCG(seed, seed))

		rt := Make[int, int](cmp.Compare[int], func(a, b int) bool { return a == b })
		n := naiveInts{}

		valRange := rng.IntN(maxRange) + 1
		if rng.IntN(10) == 0 {
			valRange = rng.IntN(10) + 1
		}
		for op := 0; op < 500; op++ {
			a, b := rng.IntN(valRange), rng.IntN(valRange)
			if a > b {
				a, b = b, a
			}
			if rng.IntN(5) == 0 {
				delta := rng.IntN(10) - 5
				rt.Update(a, b, func(p int) int { return p + delta })
				n.Add(a, b, delta)
				if debug {
					fmt.Fprintf(&log, "[%d, %d) += %d\n", a, b, delta)
					rt.tree.Ascend(func(r region[int, int]) bool {
						fmt.Fprintf(&log, "  region: [%d, = %d\n", r.start, r.prop)
						return true
					})
				}
			} else {
				var b1, b2 strings.Builder
				rt.Enumerate(a, b, func(start, end, val int) bool {
					fmt.Fprintf(&b1, "  [%d, %d) = %d\n", start, end, val)
					return true
				})
				n.Enumerate(a, b, func(start, end, val int) {
					fmt.Fprintf(&b2, "  [%d, %d) = %d\n", start, end, val)
				})
				if b1.String() != b2.String() {
					if debug {
						fmt.Printf("log:\n%s\n", log.String())
					}
					t.Fatalf("Enumerate(%d,%d) mismatch:\n%sexpected:\n%s\nseed: %d", a, b, b1.String(), b2.String(), seed)
				}
				if rng.IntN(4) == 0 {
					if exp, actual := n.IsEmpty(), rt.IsEmpty(); exp != actual {
						if debug {
							fmt.Printf("log:\n%s\n", log.String())
						}
						t.Fatalf("IsEmpty %t instead of %t\nseed: %d", actual, exp, seed)
					}
				}
			}
			rt.CheckInvariants()
		}
	}
}

const maxRange = 1000

type naiveInts struct {
	values [maxRange]int
}

func (n *naiveInts) Add(start int, end int, delta int) {
	for i := start; i < end; i++ {
		n.values[i] += delta
	}
}

func (n *naiveInts) Enumerate(start int, end int, emit func(start, end, val int)) {
	if start >= end {
		return
	}
	lastBoundary := start
	lastVal := n.values[start]
	for i := start + 1; i < end; i++ {
		if lastVal != n.values[i] {
			if lastVal != 0 {
				emit(lastBoundary, i, lastVal)
			}
			lastBoundary = i
			lastVal = n.values[i]
		}
	}
	if lastVal != 0 {
		emit(lastBoundary, end, lastVal)
	}
}

func (n *naiveInts) IsEmpty() bool {
	for i := range n.values {
		if n.values[i] != 0 {
			return false
		}
	}
	return true
}

func TestClone(t *testing.T) {
	expect := func(rt *T[int, int], vals ...int) {
		var r [][3]int
		rt.Enumerate(0, 1000, func(start, end, prop int) bool {
			r = append(r, [3]int{start, end, prop})
			return true
		})
		var exp [][3]int
		for i := 0; i < len(vals); i += 3 {
			exp = append(exp, [3]int{vals[i], vals[i+1], vals[i+2]})
		}
		if !reflect.DeepEqual(r, exp) {
			t.Helper()
			t.Fatalf("expected:\n%v\ngot:\n%v", exp, r)
		}
	}
	t1 := Make[int, int](cmp.Compare[int], func(a, b int) bool { return a == b })
	t1.Update(5, 10, func(v int) int { return 100 })
	t1.Update(9, 22, func(v int) int { return 200 })
	expect(&t1, 5, 9, 100, 9, 22, 200)
	t2 := t1.Clone()
	expect(&t2, 5, 9, 100, 9, 22, 200)
	t2.Update(6, 10, func(v int) int { return 0 })
	expect(&t1, 5, 9, 100, 9, 22, 200)
	expect(&t2, 5, 6, 100, 10, 22, 200)
	t1.Update(3, 8, func(v int) int { return 300 })
	expect(&t1, 3, 8, 300, 8, 9, 100, 9, 22, 200)
	expect(&t2, 5, 6, 100, 10, 22, 200)
}
