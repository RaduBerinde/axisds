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
	"fmt"
	"strings"

	"github.com/RaduBerinde/axisds"
	"github.com/google/btree"
)

type Boundary = axisds.Boundary

// Property is an arbitrary type that represents a property of a region of a
// one-dimensional axis.
type Property any

// PropertyEqualFn is a function used to compare two properties. If it returns
// true, the two property values can be used interchangeably.
//
// Note that it is allowed for the function to "evolve" over time, with values
// that were not equal becoming equal (but not the opposite: once two values are
// equal, they must stay equal forever).
//
// A property zero value is any value that is equal to the zero P value.
type PropertyEqualFn[P Property] func(a, b P) bool

// T is a tree of regions which fragment the entire one-dimensional space. Each
// region maintains a property. Neighboring regions with equal properties are
// automatically merged.
type T[B Boundary, P Property] struct {
	cmp    axisds.CompareFn[B]
	propEq PropertyEqualFn[P]
	tree   *btree.BTreeG[region[B, P]]
}

type region[B Boundary, P Property] struct {
	start B
	prop  P
}

func New[B Boundary, P Property](cmp axisds.CompareFn[B], propEq PropertyEqualFn[P]) *T[B, P] {
	t := &T[B, P]{}
	t.Init(cmp, propEq)
	return t
}

func (t *T[B, P]) Init(cmp axisds.CompareFn[B], propEq PropertyEqualFn[P]) {
	t.cmp = cmp
	t.propEq = propEq
	lessFn := func(a, b region[B, P]) bool {
		return cmp(a.start, b.start) < 0
	}
	t.tree = btree.NewG[region[B, P]](4, lessFn)
}

func (t *T[B, P]) Update(start, end B, updateProp func(p P) P) {
	t.ensureBoundary(start)
	t.ensureBoundary(end)

	var toUpdate []region[B, P]
	// Collect all regions in the range that need to be updated.
	t.tree.AscendRange(region[B, P]{start: start}, region[B, P]{start: end}, func(r region[B, P]) bool {
		toUpdate = append(toUpdate, region[B, P]{start: r.start, prop: updateProp(r.prop)})
		return true
	})
	for _, r := range toUpdate {
		t.tree.ReplaceOrInsert(r)
	}
	t.optimizeRange(start, end)
}

// Enumerate all fragments of the region [start, end) with non-zero property.
func (t *T[B, P]) Enumerate(start, end B, emit func(start, end B, prop P) bool) {
	if t.tree.Len() < 2 || t.cmp(start, end) >= 0 {
		return
	}
	var lastProp P
	skipFirst := false
	t.tree.DescendLessOrEqual(region[B, P]{start: start}, func(r region[B, P]) bool {
		if t.cmp(start, r.start) == 0 {
			skipFirst = true
		}
		lastProp = r.prop
		return false
	})
	lastBoundary := start
	var toDelete []region[B, P]
	t.tree.AscendGreaterOrEqual(region[B, P]{start: start}, func(r region[B, P]) bool {
		var zeroProp P
		if skipFirst {
			skipFirst = false
			return true
		}
		if t.cmp(end, r.start) <= 0 {
			if !t.propEq(lastProp, zeroProp) {
				emit(lastBoundary, end, lastProp)
			}
			return false
		}
		if t.propEq(lastProp, r.prop) {
			// This boundary is not useful, skip.
			toDelete = append(toDelete, r)
			return true
		}
		if !t.propEq(lastProp, zeroProp) && !emit(lastBoundary, r.start, lastProp) {
			return false
		}
		lastBoundary = r.start
		lastProp = r.prop
		return true
	})
	for _, b := range toDelete {
		t.tree.Delete(b)
	}
}

// IsEmpty returns true if the set contains no non-expired spans.
func (t *T[B, P]) IsEmpty() bool {
	if t.tree.Len() < 2 {
		return true
	}
	// Check that we have regions with non-zero property.
	var toDelete []region[B, P]
	t.tree.Ascend(func(r region[B, P]) bool {
		var zeroProp P
		if t.propEq(r.prop, zeroProp) {
			toDelete = append(toDelete, r)
			return true
		}
		return false
	})
	for _, r := range toDelete {
		t.tree.Delete(r)
	}
	return t.tree.Len() < 2
}

// CheckInvariants can be used in testing builds to verify internal invariants.
func (t *T[B, P]) CheckInvariants() {
	t.tree.Descend(func(r region[B, P]) bool {
		var zeroProp P
		if !t.propEq(r.prop, zeroProp) {
			panic("last region must always have zero property")
		}
		return false
	})
}

func (t *T[B, P]) String(bFmt axisds.Formatter[B]) string {
	var b strings.Builder
	var lastBoundary B
	var lastProp, zeroProp P
	first := true
	t.tree.Ascend(func(r region[B, P]) bool {
		if first {
			first = false
			lastBoundary = r.start
			lastProp = r.prop
			return true
		}
		if t.propEq(lastProp, r.prop) {
			return true
		}
		if !t.propEq(lastProp, zeroProp) {
			fmt.Fprintf(&b, "%s = %v\n", bFmt.FormatInterval(lastBoundary, r.start), lastProp)
		}
		lastBoundary = r.start
		lastProp = r.prop
		return true
	})
	if b.Len() == 0 {
		return "<empty>"
	}
	return b.String()
}
