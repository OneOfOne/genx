// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !genx_t_builtin

package sort

import (
	U "github.com/OneOfOne/genx/seeds/sort/utils"
)

type T interface{}

// SortTs sorts the provided slice given the provided less function.
// The sort is not guaranteed to be stable. For a stable sort, use StableSortTs.
// For reverse sort, return j < i.
func SortTs(s []T, less func(i, j int) bool) {
	swap := func(i, j int) { s[i], s[j] = s[j], s[i] }
	U.QuickSort(U.LessSwap{Less: less, Swap: swap}, 0, len(s), U.MaxDepth(len(s)))
}

// StableSortTs sorts the provided slice given the provided less
// function while keeping the original order of equal elements.
// For reverse sort, return j < i.
func StableSortTs(s []T, less func(i, j int) bool) {
	swap := func(i, j int) { s[i], s[j] = s[j], s[i] }
	U.Stable(U.LessSwap{Less: less, Swap: swap}, len(s))
}

// TsAreSorted tests whether a slice is sorted.
// For reverse sort, return j < i.
func TsAreSorted(s []T, less func(i, j int) bool) bool {
	for i := len(s) - 1; i > 0; i-- {
		if less(i, i-1) {
			return false
		}
	}
	return true
}
