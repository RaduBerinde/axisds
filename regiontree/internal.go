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

// ensureBoundary makes sure the tree contains the given boundary, inserting it
// if necessary.
func (t *T[B, P]) ensureBoundary(b B) {
	var exists bool
	var p P
	t.tree.DescendLessOrEqual(region[B, P]{start: b}, func(r region[B, P]) bool {
		if t.cmp(b, r.start) == 0 {
			// Boundary exists, nothing to do.
			exists = true
		} else {
			// We will split the region; both splits will have the same property.
			p = r.prop
		}
		return false
	})
	// If there was no boundary <= b then p is empty, which is what we want.
	if !exists {
		t.tree.ReplaceOrInsert(region[B, P]{start: b, prop: p})
	}
}

// optimizeRange removes any unnecessary boundaries in the given range.
func (t *T[B, P]) optimizeRange(start, end B) {
	var toDelete []region[B, P]
	var last P
	first := true
	t.tree.AscendGreaterOrEqual(region[B, P]{start: start}, func(r region[B, P]) bool {
		if first {
			first = false
		} else if t.propEq(last, r.prop) {
			// This boundary is not necessary; we can merge the regions.
			toDelete = append(toDelete, r)
			// Keep going even if we go past the end.
			return true
		}
		last = r.prop
		return t.cmp(r.start, end) <= 0
	})
	for _, b := range toDelete {
		t.tree.Delete(b)
	}
}
