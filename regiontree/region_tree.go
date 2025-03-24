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

// PropertyEqualFn is a function used to compare properties of two regions. If
// it returns true, the two property values can be used interchangeably.
//
// Note that it is allowed for the function to "evolve" over time, with values
// that were not equal becoming equal (but not the opposite: once two values are
// equal, they must stay equal forever).
//
// A zero property value is any value that is equal to the zero P value.
type PropertyEqualFn[P Property] func(a, b P) bool

// T is a tree of regions which fragment a one-dimensional space. Regions have
// boundaries of type B and each region maintains a property P. Neighboring
// regions with equal properties are automatically merged.
//
// T supports lazy (copy-on-write) cloning via Clone().
type T[B Boundary, P Property] struct {
	cmp    axisds.CompareFn[B]
	propEq PropertyEqualFn[P]
	tree   *btree.BTreeG[region[B, P]]
}

// region is a fragment of the one-dimensional space with a property.
// The region ends at the next region's start boundary.
type region[B Boundary, P Property] struct {
	start B
	prop  P
}

// Make creates a new region tree with the given boundary and property
// comparison functions.
func Make[B Boundary, P Property](cmp axisds.CompareFn[B], propEq PropertyEqualFn[P]) T[B, P] {
	t := T[B, P]{
		cmp:    cmp,
		propEq: propEq,
	}
	lessFn := func(a, b region[B, P]) bool {
		return cmp(a.start, b.start) < 0
	}
	t.tree = btree.NewG[region[B, P]](4, lessFn)
	return t
}

// Update the property for the given range. The updateProp function is called
// for all the regions within the range to calculate the new property.
//
// The runtime complexity is O(log N + K) where K is the number of regions we
// are updating. Note that if the ranges we update are mostly non-overlapping,
// this will be O(log N) on average.
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

// Enumerate all regions in the range [start, end) with non-zero property.
//
// Two consecutive regions can "touch" but not overlap; if they touch, their
// properties are not equal.
//
// Enumerate stops once emit() returns false.
func (t *T[B, P]) Enumerate(start, end B, emit func(start, end B, prop P) bool) {
	if t.tree.Len() < 2 || t.cmp(start, end) >= 0 {
		return
	}
	var eh enumerateHelper[B, P]
	// Handle the case where we don't have a boundary equal to start; we have to
	// find the region that contains it.
	t.tree.DescendLessOrEqual(region[B, P]{start: start}, func(r region[B, P]) bool {
		if t.cmp(r.start, start) < 0 {
			// This is the first addRegion call, so we won't emit anything,.
			eh.addRegion(start, r.prop, t.propEq, nil)
		}
		return false
	})
	var toDelete []region[B, P]
	t.tree.AscendRange(region[B, P]{start: start}, region[B, P]{start: end}, func(r region[B, P]) bool {
		eh.addRegion(r.start, r.prop, t.propEq, emit)
		if eh.canDeleteLastBoundary {
			toDelete = append(toDelete, r)
		}
		return !eh.stopEmitting
	})
	eh.finish(end, t.propEq, emit)
	for _, b := range toDelete {
		t.tree.Delete(b)
	}
}

// EnumerateAll emits all regions with non-zero property.
//
// Two consecutive regions can "touch" but not overlap; if they touch, their
// properties are not equal.
//
// EnumerateAll stops once emit() returns false.
func (t *T[B, P]) EnumerateAll(emit func(start, end B, prop P) bool) {
	var eh enumerateHelper[B, P]
	var toDelete []region[B, P]
	t.tree.Ascend(func(r region[B, P]) bool {
		eh.addRegion(r.start, r.prop, t.propEq, emit)
		if eh.canDeleteLastBoundary {
			toDelete = append(toDelete, r)
		}
		return !eh.stopEmitting
	})
	for _, b := range toDelete {
		t.tree.Delete(b)
	}
}

type enumerateHelper[B Boundary, P Property] struct {
	lastBoundary B
	lastProp     P
	initialized  bool
	stopEmitting bool
	// canDeleteLastBoundary is set by addRegion when the two last regions had
	// equal properties.
	canDeleteLastBoundary bool
}

func (eh *enumerateHelper[B, P]) addRegion(
	boundary B, prop P, propEq PropertyEqualFn[P], emitFn func(start, end B, prop P) bool,
) {
	if !eh.initialized {
		eh.lastBoundary = boundary
		eh.lastProp = prop
		eh.initialized = true
		return
	}
	eh.canDeleteLastBoundary = propEq(eh.lastProp, prop)
	if eh.canDeleteLastBoundary || eh.stopEmitting {
		return
	}
	var zeroProp P
	if !propEq(zeroProp, eh.lastProp) && !emitFn(eh.lastBoundary, boundary, eh.lastProp) {
		eh.stopEmitting = true
	}
	eh.lastBoundary = boundary
	eh.lastProp = prop
}

func (eh *enumerateHelper[B, P]) finish(
	end B, propEq PropertyEqualFn[P], emitFn func(start, end B, prop P) bool,
) {
	var zeroProp P
	if eh.initialized && !eh.stopEmitting && !propEq(zeroProp, eh.lastProp) {
		emitFn(eh.lastBoundary, end, eh.lastProp)
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

// Clone creates a lazy clone of T with the same properties and regions. The new
// tree can be modified independently.
//
// This operation is constant time; it can cause some minor slowdown of future
// updates because of copy-on-write logic.
func (t *T[B, P]) Clone() T[B, P] {
	return T[B, P]{
		cmp:    t.cmp,
		propEq: t.propEq,
		tree:   t.tree.Clone(),
	}
}

// String formats all regions, one per line.
func (t *T[B, P]) String(iFmt axisds.IntervalFormatter[B]) string {
	var b strings.Builder
	// We don't use EnumerateAll because we don't want String() to modify the
	// structure (it is typically used for testing or debugging).
	var eh enumerateHelper[B, P]
	t.tree.Ascend(func(r region[B, P]) bool {
		eh.addRegion(r.start, r.prop, t.propEq, func(start, end B, prop P) bool {
			fmt.Fprintf(&b, "%s = %v\n", iFmt(start, end), prop)
			return true
		})
		return true
	})
	if b.Len() == 0 {
		return "<empty>"
	}
	return b.String()
}

// CheckInvariants can be used in testing builds to verify internal invariants.
func (t *T[B, P]) CheckInvariants() {
	var lastBoundary B
	var lastProp P
	lastBoundarySet := false
	t.tree.Ascend(func(r region[B, P]) bool {
		if lastBoundarySet {
			if t.cmp(lastBoundary, r.start) >= 0 {
				panic("region boundaries not increasing")
			}
		}
		if !t.propEq(r.prop, r.prop) {
			panic("region property is not equal to itself")
		}
		lastBoundary = r.start
		lastBoundarySet = true
		lastProp = r.prop
		return true
	})

	// Last region should have the zero property.
	if lastBoundarySet {
		var zeroProp P
		if !t.propEq(lastProp, zeroProp) {
			panic("last region must always have zero property")
		}
	}
}
