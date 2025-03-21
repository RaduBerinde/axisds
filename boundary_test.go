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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatters(t *testing.T) {
	bFmt := MakeBasicFormatter[int]()
	require.Equal(t, "[1, 5)", bFmt.FormatInterval(1, 5))

	eFmt := MakeEndpointFormatter[int](bFmt)
	var x, y Endpoint[int]
	x, y = MakeEndpoints(1, Inclusive, 5, Inclusive)
	require.Equal(t, "[1, 5]", eFmt.FormatInterval(x, y))
	x, y = MakeEndpoints(1, Inclusive, 5, Exclusive)
	require.Equal(t, "[1, 5)", eFmt.FormatInterval(x, y))
	x, y = MakeEndpoints(1, Exclusive, 5, Inclusive)
	require.Equal(t, "(1, 5]", eFmt.FormatInterval(x, y))
	x, y = MakeEndpoints(1, Exclusive, 5, Exclusive)
	require.Equal(t, "(1, 5)", eFmt.FormatInterval(x, y))

	require.Equal(t, "1+", eFmt.FormatBoundary(x))
	require.Equal(t, "5", eFmt.FormatBoundary(y))
}
