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

import (
	"reflect"
	"testing"
)

func TestBasicParser(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		p := MakeBasicParser[int]()
		testParse(t, p, "[1, 2)", 1, 2, "")
		testParse(t, p, "[1, 2) ", 1, 2, "")
		testParse(t, p, "[1, 2) foo", 1, 2, "foo")
		testParse(t, p, "[1, 2) foo bar", 1, 2, "foo bar")
		testParse(t, p, "[1, 2)    foo bar", 1, 2, "foo bar")

		testParseErr(t, p, "(1, 2)")
		testParseErr(t, p, "[1, 2]")
		testParseErr(t, p, "[1, 2")
		testParseErr(t, p, "1, 2)")
		testParseErr(t, p, "[1,2)")
	})
	t.Run("string", func(t *testing.T) {
		p := MakeBasicParser[string]()
		testParse(t, p, "[abc, de)", "abc", "de", "")
		testParse(t, p, "[abc, de) ", "abc", "de", "")
		testParse(t, p, "[abc, de) foo", "abc", "de", "foo")
		testParse(t, p, "[abc, de) foo bar", "abc", "de", "foo bar")
		testParse(t, p, "[abc, de)    foo bar", "abc", "de", "foo bar")

		testParseErr(t, p, "(abc, de)")
		testParseErr(t, p, "[abc, de]")
		testParseErr(t, p, "[abc, de")
		testParseErr(t, p, "abc, de)")
		testParseErr(t, p, "[abc,de)")
	})
}

func TestEndpointParser(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		p := MakeEndpointParser(MakeBasicParser[int]())
		at := func(x int) Endpoint[int] { return Endpoint[int]{B: x} }
		after := func(x int) Endpoint[int] { return Endpoint[int]{B: x, PlusEpsilon: true} }
		testParse(t, p, "[1, 2)", at(1), at(2), "")
		testParse(t, p, "[1, 2]", at(1), after(2), "")
		testParse(t, p, "(1, 2)", after(1), at(2), "")
		testParse(t, p, "(1, 2]", after(1), after(2), "")

		testParse(t, p, "[1, 2) ", at(1), at(2), "")
		testParse(t, p, "(1, 2] foo", after(1), after(2), "foo")
		testParse(t, p, "[1, 2] foo bar", at(1), after(2), "foo bar")
		testParse(t, p, "[1, 2]    foo bar", at(1), after(2), "foo bar")

		testParseErr(t, p, "]1, 2)")
		testParseErr(t, p, "[1, 2(")
		testParseErr(t, p, "1, 2)")
		testParseErr(t, p, "[1,2)")
	})

	t.Run("string", func(t *testing.T) {
		p := MakeEndpointParser(MakeBasicParser[string]())
		at := func(x string) Endpoint[string] { return Endpoint[string]{B: x} }
		after := func(x string) Endpoint[string] { return Endpoint[string]{B: x, PlusEpsilon: true} }
		testParse(t, p, "[abc, de)", at("abc"), at("de"), "")
		testParse(t, p, "(abc, de]", after("abc"), after("de"), "")
		testParse(t, p, "(abc, de) ", after("abc"), at("de"), "")
		testParse(t, p, "[abc, de] foo", at("abc"), after("de"), "foo")
		testParse(t, p, "[abc, de) foo bar", at("abc"), at("de"), "foo bar")
		testParse(t, p, "(abc, de]    foo bar", after("abc"), after("de"), "foo bar")

		testParseErr(t, p, "]abc, de)")
		testParseErr(t, p, "[abc, de(")
		testParseErr(t, p, "[abc, de")
		testParseErr(t, p, "abc, de)")
		testParseErr(t, p, "[abc,de)")
	})
}

func testParseErr[B Boundary](t *testing.T, p Parser[B], input string) {
	_, _, _, err := p.ParseInterval(input)
	if err == nil {
		t.Helper()
		t.Fatalf("%q: expected error", input)
	}
}

func testParse[B Boundary](
	t *testing.T, p Parser[B], input string, expectedStart, expectedEnd B, expectedRemainder string,
) {
	t.Helper()
	start, end, rem, err := p.ParseInterval(input)
	if err != nil {
		t.Fatalf("%q: unexpected error: %v", input, err)
	}
	if !reflect.DeepEqual(start, expectedStart) || !reflect.DeepEqual(end, expectedEnd) || rem != expectedRemainder {
		t.Fatalf("expected %v %v %q, got %v %v %q", expectedStart, expectedEnd, expectedRemainder, start, end, rem)
	}
}

func TestFormatParseRoundtrip(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		f := MakeBasicFormatter[int]()
		p := MakeBasicParser[int]()
		testRoundtrip(t, f, p, 1, 2)
	})
	t.Run("string", func(t *testing.T) {
		f := MakeBasicFormatter[string]()
		p := MakeBasicParser[string]()
		testRoundtrip(t, f, p, "a", "bc")
	})
	t.Run("endpoints-int", func(t *testing.T) {
		f := MakeEndpointFormatter(MakeBasicFormatter[int]())
		p := MakeEndpointParser(MakeBasicParser[int]())
		testRoundtrip(t, f, p, MakeStartEndpoint(1, Inclusive), MakeEndEndpoint(2, Exclusive))
		testRoundtrip(t, f, p, MakeStartEndpoint(1, Exclusive), MakeEndEndpoint(2, Exclusive))
		testRoundtrip(t, f, p, MakeStartEndpoint(1, Inclusive), MakeEndEndpoint(2, Inclusive))
		testRoundtrip(t, f, p, MakeStartEndpoint(1, Exclusive), MakeEndEndpoint(2, Inclusive))
	})
	t.Run("endpoints-string", func(t *testing.T) {
		f := MakeEndpointFormatter(MakeBasicFormatter[string]())
		p := MakeEndpointParser(MakeBasicParser[string]())
		testRoundtrip(t, f, p, MakeStartEndpoint("a", Inclusive), MakeEndEndpoint("b", Exclusive))
		testRoundtrip(t, f, p, MakeStartEndpoint("ab", Exclusive), MakeEndEndpoint("ac", Exclusive))
		testRoundtrip(t, f, p, MakeStartEndpoint("a", Inclusive), MakeEndEndpoint("fgh", Inclusive))
		testRoundtrip(t, f, p, MakeStartEndpoint("a", Exclusive), MakeEndEndpoint("z", Inclusive))
	})
}

func testRoundtrip[B Boundary](t *testing.T, f Formatter[B], p Parser[B], start, end B) {
	str := f.FormatInterval(start, end)
	x, y := MustParseInterval(p, str)
	if !reflect.DeepEqual(x, start) || !reflect.DeepEqual(y, end) {
		t.Fatalf("roundtrip %v %v failed: %v %v\n", start, end, x, y)
	}
}
