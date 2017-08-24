// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build genx
// +build genx_t_builtin

package sort

import (
	U "github.com/OneOfOne/genx/seeds/sort/utils"
)

type T int

// SortTs sorts the provided slice given the provided less function.
// The sort is not guaranteed to be stable. For a stable sort, use SliceStable.
func SortTs(s []T, reverse bool) {
	var less func(i, j int) bool
	if reverse {
		less = func(i, j int) bool { return s[j] < s[i] }
	} else {
		less = func(i, j int) bool { return s[i] < s[j] }
	}
	swap := func(i, j int) { s[i], s[j] = s[j], s[i] }
	U.QuickSort(U.LessSwap{Less: less, Swap: swap}, 0, len(s), U.MaxDepth(len(s)))
}

// StableSortTs sorts the provided slice given the provided less
// function while keeping the original order of equal elements.
func StableSortTs(s []T, reverse bool) {
	var less func(i, j int) bool
	if reverse {
		less = func(i, j int) bool { return s[j] < s[i] }
	} else {
		less = func(i, j int) bool { return s[i] < s[j] }
	}
	swap := func(i, j int) { s[i], s[j] = s[j], s[i] }
	U.Stable(U.LessSwap{Less: less, Swap: swap}, len(s))
}

// TsAreSorted tests whether a slice is sorted.
// For reverse sort, return j < i from less.
func TsAreSorted(s []T, reverse bool) bool {
	var less func(i, j int) bool
	if reverse {
		less = func(i, j int) bool { return s[j] < s[i] }
	} else {
		less = func(i, j int) bool { return s[i] < s[j] }
	}
	for i := len(s) - 1; i > 0; i-- {
		if less(i, i-1) {
			return false
		}
	}
	return true
}
