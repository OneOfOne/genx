// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build genx
// +build !genx_kt_builtin

package sort

import (
	U "github.com/OneOfOne/genx/seeds/sort/utils"
)

type KT interface{}

// KTSlice sorts the provided slice given the provided less function.
// The sort is not guaranteed to be stable. For a stable sort, use SliceStable.
// For reverse sort, return j < i.
func KTSlice(s []KT, less func(i, j int) bool) {
	swap := func(i, j int) { s[i], s[j] = s[j], s[i] }
	U.QuickSort(U.LessSwap{Less: less, Swap: swap}, 0, len(s), U.MaxDepth(len(s)))
}

// StableKT sorts the provided slice given the provided less
// function while keeping the original order of equal elements.
// For reverse sort, return j < i.
func StableKT(s []KT, less func(i, j int) bool) {
	swap := func(i, j int) { s[i], s[j] = s[j], s[i] }
	U.Stable(U.LessSwap{Less: less, Swap: swap}, len(s))
}

// KTIsSorted tests whether a slice is sorted.
// For reverse sort, return j < i.
func KTIsSorted(s []KT, less func(i, j int) bool) bool {
	for i := len(s) - 1; i > 0; i-- {
		if less(i, i-1) {
			return false
		}
	}
	return true
}
