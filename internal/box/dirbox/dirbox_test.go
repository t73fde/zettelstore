//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package dirbox

import "testing"

func TestIsPrime(t *testing.T) {
	testcases := []struct {
		n   uint32
		exp bool
	}{
		{0, false}, {1, true}, {2, true}, {3, true}, {4, false}, {5, true},
		{6, false}, {7, true}, {8, false}, {9, false}, {10, false},
		{11, true}, {12, false}, {13, true}, {14, false}, {15, false},
		{17, true}, {19, true}, {21, false}, {23, true}, {25, false},
		{27, false}, {29, true}, {31, true}, {33, false}, {35, false},
	}
	for _, tc := range testcases {
		got := isPrime(tc.n)
		if got != tc.exp {
			t.Errorf("isPrime(%d)=%v, but got %v", tc.n, tc.exp, got)
		}
	}
}

func TestMakePrime(t *testing.T) {
	for i := range uint32(1500) {
		np := makePrime(i)
		if np < i {
			t.Errorf("makePrime(%d) < %d", i, np)
			continue
		}
		if !isPrime(np) {
			t.Errorf("makePrime(%d) == %d is not prime", i, np)
			continue
		}
		if isPrime(i) && i != np {
			t.Errorf("%d is already prime, but got %d as next prime", i, np)
			continue
		}
	}
}
