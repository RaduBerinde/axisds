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
	"iter"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/RaduBerinde/axisds/v3"
	"github.com/RaduBerinde/btreemap"
	"github.com/cockroachdb/datadriven"
)

const debug = false

func TestDataDriven(t *testing.T) {
	t.Run("ints", func(t *testing.T) {
		testDataDriven(
			t, "testdata/ints",
			cmp.Compare[int],
			axisds.MakeIntervalFormatter(axisds.MakeBoundaryFormatter[int]()),
			axisds.MakeBasicParser[int](),
		)
	})
	t.Run("endpoints-ints", func(t *testing.T) {
		testDataDriven(
			t, "testdata/endpoints-ints",
			axisds.EndpointCompareFn(cmp.Compare[int]),
			axisds.MakeEndpointIntervalFormatter(axisds.MakeBoundaryFormatter[int]()),
			axisds.MakeEndpointParser(axisds.MakeBasicParser[int]()),
		)
	})
}

func testDataDriven[B Boundary](
	t *testing.T,
	path string,
	cmpFn func(a, b B) int,
	iFmt axisds.IntervalFormatter[B],
	p axisds.Parser[B],
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
		rt.CheckInvariants()
		buf.WriteString("regions:\n")
		for _, l := range strings.Split(strings.TrimSpace(rt.String(iFmt)), "\n") {
			fmt.Fprintf(&buf, "  %s\n", l)
		}
		return buf.String()
	})
}

func TestRegionTreeRand(t *testing.T) {
	for test := 0; test < 100; test++ {
		seed := rand.Uint64()
		rng := rand.New(rand.NewPCG(seed, seed))

		var debugLog bytes.Buffer
		fmt.Fprintf(&debugLog, "seed: %d", seed)
		if debug {
			fmt.Fprintf(&debugLog, "\nlog:\n")
		}

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

			switch rng.IntN(10) {
			case 0:
				delta := rng.IntN(10) - 5
				rt.Update(a, b, func(p int) int { return p + delta })
				n.Add(a, b, delta)
				if debug {
					fmt.Fprintf(&debugLog, "[%d, %d) += %d\n", a, b, delta)
					for start, prop := range rt.tree.Ascend(btreemap.Min[int](), btreemap.Max[int]()) {
						fmt.Fprintf(&debugLog, "  region: [%d, = %d\n", start, prop)
					}
				}

			case 1:
				value := rng.IntN(10) - 5
				rt.Update(a, b, func(p int) int { return value })
				n.Set(a, b, value)
				if debug {
					fmt.Fprintf(&debugLog, "[%d, %d) = %d\n", a, b, value)
					for start, prop := range rt.tree.Ascend(btreemap.Min[int](), btreemap.Max[int]()) {
						fmt.Fprintf(&debugLog, "  region: [%d, = %d\n", start, prop)
					}
				}

			case 2:
				value := rng.IntN(10) - 5
				withGC := rng.IntN(2) == 0
				lower, naiveA := randLowerBound(rng, a)
				upper, naiveB := randUpperBound(rng, b)
				propFn := func(prop int) bool { return prop == value }
				actual := rt.any(lower, upper, propFn, withGC)
				expected := n.Any(naiveA, naiveB, propFn)
				// When Min() is used, the tree covers (−∞, 0) which always has
				// zero property; the naive model can't represent this range.
				if lower.isMin && propFn(0) {
					expected = true
				}
				if actual != expected {
					t.Fatalf("Any(%v,%v,%d) mismatch: expected %t, got %t\n%s", lower, upper, value, expected, actual, debugLog.String())
				}

			case 3:
				if exp, actual := n.IsEmpty(), rt.IsEmpty(); exp != actual {
					t.Fatalf("IsEmpty %t instead of %t\n%s", actual, exp, debugLog.String())
				}

			case 4:
				// Test EnumerateDesc against naive (reversed).
				var b1 strings.Builder
				withGC := rng.IntN(2) == 0
				lower, naiveA := randLowerBound(rng, a)
				upper, naiveB := randUpperBound(rng, b)
				rt.enumerateDesc(upper, lower, func(i axisds.Interval[int], val int) bool {
					fmt.Fprintf(&b1, "  [%d, %d) = %d\n", i.Start, i.End, val)
					return true
				}, withGC)
				var naiveResults []string
				n.Enumerate(naiveA, naiveB, func(start, end, val int) {
					naiveResults = append(naiveResults, fmt.Sprintf("  [%d, %d) = %d\n", start, end, val))
				})
				var b2 strings.Builder
				for i := len(naiveResults) - 1; i >= 0; i-- {
					b2.WriteString(naiveResults[i])
				}
				if b1.String() != b2.String() {
					t.Fatalf("EnumerateDesc(%v,%v) mismatch:\n%sexpected:\n%s\n%s", upper, lower, b1.String(), b2.String(), debugLog.String())
				}

			default:
				var b1, b2 strings.Builder
				withGC := rng.IntN(2) == 0
				lower, naiveA := randLowerBound(rng, a)
				upper, naiveB := randUpperBound(rng, b)
				rt.enumerate(lower, upper, func(i axisds.Interval[int], val int) bool {
					fmt.Fprintf(&b1, "  [%d, %d) = %d\n", i.Start, i.End, val)
					return true
				}, withGC)
				n.Enumerate(naiveA, naiveB, func(start, end, val int) {
					fmt.Fprintf(&b2, "  [%d, %d) = %d\n", start, end, val)
				})
				if b1.String() != b2.String() {
					t.Fatalf("Enumerate(%v,%v) mismatch:\n%sexpected:\n%s\n%s", lower, upper, b1.String(), b2.String(), debugLog.String())
				}
			}

			rt.CheckInvariants()
		}
	}
}

// randLowerBound randomly returns either GE(key) or Min(). When Min() is
// returned, the naive equivalent bound (0) is returned as the second value.
func randLowerBound(rng *rand.Rand, key int) (LowerBound[int], int) {
	if rng.IntN(4) == 0 {
		return Min[int](), 0
	}
	return GE(key), key
}

// randUpperBound randomly returns either LT(key) or Max(). When Max() is
// returned, the naive equivalent bound (maxRange) is returned as the second value.
func randUpperBound(rng *rand.Rand, key int) (UpperBound[int], int) {
	if rng.IntN(4) == 0 {
		return Max[int](), maxRange
	}
	return LT(key), key
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

func (n *naiveInts) Set(start int, end int, value int) {
	for i := start; i < end; i++ {
		n.values[i] = value
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

func (n *naiveInts) Any(start int, end int, fn func(int) bool) bool {
	for i := start; i < end; i++ {
		if fn(n.values[i]) {
			return true
		}
	}
	return false
}

func (n *naiveInts) IsEmpty() bool {
	for i := range n.values {
		if n.values[i] != 0 {
			return false
		}
	}
	return true
}

func TestWithGC(t *testing.T) {
	// global is a threshold; properties with value <= global are effectively
	// zero (they all compare equal to 0).
	global := 0
	rt := Make[int, int](cmp.Compare[int], func(a, b int) bool {
		ea, eb := max(0, a-global), max(0, b-global)
		return ea == eb
	})

	// Set up regions with adjacent values that will become equal after
	// increasing global: [0,10)=3  [10,20)=5  [20,30)=15  [30,40)=2
	// After global=5: effective values are 0, 0, 10, 0.
	// Adjacent pairs (3,5)→(0,0) and (2,0)→(0,0) become equal, so boundaries
	// at 10 and 40 can be GC'd.
	rt.Update(0, 10, func(int) int { return 3 })
	rt.Update(10, 20, func(int) int { return 5 })
	rt.Update(20, 30, func(int) int { return 15 })
	rt.Update(30, 40, func(int) int { return 2 })

	// Collect results from an iterator.
	collect := func(it iter.Seq2[axisds.Interval[int], int]) [][3]int {
		var result [][3]int
		for i, prop := range it {
			result = append(result, [3]int{i.Start, i.End, prop})
		}
		return result
	}

	// Initially all four regions are present (4 starts + 1 trailing zero).
	initialLen := rt.InternalLen()
	if initialLen != 5 {
		t.Fatalf("expected InternalLen 5, got %d", initialLen)
	}

	// Increase global so that values 3, 5, and 2 become effectively zero.
	global = 5
	// Without GC, results should reflect effective values but boundaries remain.
	got := collect(rt.Enumerate(GE(0), LT(50)))
	exp := [][3]int{{20, 30, 15}}
	if !reflect.DeepEqual(got, exp) {
		t.Fatalf("Enumerate without GC: expected %v, got %v", exp, got)
	}
	if rt.InternalLen() != initialLen {
		t.Fatalf("InternalLen should not have changed without GC: expected %d, got %d", initialLen, rt.InternalLen())
	}

	// With GC, same results but unnecessary boundaries should be cleaned up.
	got = collect(rt.Enumerate(GE(0), LT(50), WithGC))
	if !reflect.DeepEqual(got, exp) {
		t.Fatalf("Enumerate with GC: expected %v, got %v", exp, got)
	}
	gcLen := rt.InternalLen()
	if gcLen >= initialLen {
		t.Fatalf("InternalLen should have decreased after GC: was %d, now %d", initialLen, gcLen)
	}

	// Test Any with GC.
	global = 0
	rt = Make[int, int](cmp.Compare[int], func(a, b int) bool {
		ea, eb := max(0, a-global), max(0, b-global)
		return ea == eb
	})
	// [0,10)=2  [10,20)=1  [20,30)=7
	// After global=3: effective 0, 0, 4. Boundary at 10 becomes GC-able.
	rt.Update(0, 10, func(int) int { return 2 })
	rt.Update(10, 20, func(int) int { return 1 })
	rt.Update(20, 30, func(int) int { return 7 })
	initialLen = rt.InternalLen()

	global = 3
	// Any without GC.
	if !rt.Any(GE(0), LT(30), func(prop int) bool { return max(0, prop-global) > 0 }) {
		t.Fatalf("Any should find region with effective value > 0")
	}
	if rt.InternalLen() != initialLen {
		t.Fatalf("InternalLen should not change without GC")
	}
	// Any with GC.
	if !rt.Any(GE(0), LT(30), func(prop int) bool { return max(0, prop-global) > 0 }, WithGC) {
		t.Fatalf("Any with GC should find region with effective value > 0")
	}
	if rt.InternalLen() >= initialLen {
		t.Fatalf("InternalLen should decrease after Any with GC: was %d, now %d", initialLen, rt.InternalLen())
	}

	// Test All with GC.
	global = 0
	rt = Make[int, int](cmp.Compare[int], func(a, b int) bool {
		ea, eb := max(0, a-global), max(0, b-global)
		return ea == eb
	})
	// [0,10)=4  [10,20)=3  [20,30)=9
	// After global=4: effective 0, 0, 5. Boundary at 10 becomes GC-able.
	rt.Update(0, 10, func(int) int { return 4 })
	rt.Update(10, 20, func(int) int { return 3 })
	rt.Update(20, 30, func(int) int { return 9 })
	initialLen = rt.InternalLen()

	global = 4
	got = collect(rt.All(WithGC))
	exp = [][3]int{{20, 30, 9}}
	if !reflect.DeepEqual(got, exp) {
		t.Fatalf("All with GC: expected %v, got %v", exp, got)
	}
	if rt.InternalLen() >= initialLen {
		t.Fatalf("InternalLen should decrease after All with GC: was %d, now %d", initialLen, rt.InternalLen())
	}

	// Test EnumerateDesc with GC.
	global = 0
	rt = Make[int, int](cmp.Compare[int], func(a, b int) bool {
		ea, eb := max(0, a-global), max(0, b-global)
		return ea == eb
	})
	// [0,10)=3  [10,20)=5  [20,30)=15  [30,40)=2
	// After global=5: effective 0, 0, 10, 0.
	// Boundaries at 10 and 40 can be GC'd.
	rt.Update(0, 10, func(int) int { return 3 })
	rt.Update(10, 20, func(int) int { return 5 })
	rt.Update(20, 30, func(int) int { return 15 })
	rt.Update(30, 40, func(int) int { return 2 })
	initialLen = rt.InternalLen()

	global = 5
	// Without GC.
	got = collect(rt.EnumerateDesc(LT(50), GE(0)))
	exp = [][3]int{{20, 30, 15}}
	if !reflect.DeepEqual(got, exp) {
		t.Fatalf("EnumerateDesc without GC: expected %v, got %v", exp, got)
	}
	if rt.InternalLen() != initialLen {
		t.Fatalf("InternalLen should not have changed without GC: expected %d, got %d", initialLen, rt.InternalLen())
	}
	// With GC.
	got = collect(rt.EnumerateDesc(LT(50), GE(0), WithGC))
	if !reflect.DeepEqual(got, exp) {
		t.Fatalf("EnumerateDesc with GC: expected %v, got %v", exp, got)
	}
	if rt.InternalLen() >= initialLen {
		t.Fatalf("InternalLen should have decreased after EnumerateDesc with GC: was %d, now %d", initialLen, rt.InternalLen())
	}
}

func TestClone(t *testing.T) {
	expect := func(rt *T[int, int], vals ...int) {
		var r [][3]int
		for i, prop := range rt.Enumerate(GE(0), LT(1000)) {
			r = append(r, [3]int{i.Start, i.End, prop})
		}
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
