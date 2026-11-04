//-----------------------------------------------------------------------------
// Copyright (c) 2026-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2026-present Detlef Stern
//-----------------------------------------------------------------------------

package impl

import "t73f.de/r/zsc/domain/id"

// powersOf10[i] is 10^i.
var powersOf10 = [15]uint64{
	1,
	10,
	100,
	1_000,
	10_000,
	100_000,
	1_000_000,
	10_000_000,
	100_000_000,
	1_000_000_000,
	10_000_000_000,
	100_000_000_000,
	1_000_000_000_000,
	10_000_000_000_000,
	100_000_000_000_000,
}

// minZidFragmentLen is the minimum length of a search string that is also
// treated as a fragment of a zid.
const minZidFragmentLen = 4

// parseZidFragment converts s into the number it represents, if s may be a
// part of a zid: it must consist of decimal digits only and its length must be
// in the range minZidFragmentLen..id.LengthZid. Leading zeros are significant
// and are covered by len(s).
func parseZidFragment(s string) (uint64, bool) {
	if len(s) < minZidFragmentLen || len(s) > id.LengthZid {
		return 0, false
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	return n, true
}

// The following functions treat number as a zid, i.e. as a sequence of
// id.LengthZid decimal digits, padded with leading zeros. pattern is a
// sequence of patternLen digits, 1 <= patternLen <= id.LengthZid.

func zidHasPrefix(number, pattern uint64, patternLen int) bool {
	return number/powersOf10[id.LengthZid-patternLen] == pattern
}

func zidHasSuffix(number, pattern uint64, patternLen int) bool {
	return number%powersOf10[patternLen] == pattern
}

func zidContains(number, pattern uint64, patternLen int) bool {
	width := powersOf10[patternLen]
	for range id.LengthZid - patternLen + 1 {
		if number%width == pattern {
			return true
		}
		number /= 10
	}
	return false
}
