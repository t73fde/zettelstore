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

package ast

// Visitor is a visitor for walking the AST.
type Visitor interface {
	Visit(node Node) Visitor
}

// Walk traverses the AST.
func Walk(v Visitor, node Node) {
	if v = v.Visit(node); v == nil {
		return
	}

	// Implementation note:
	// It is much faster to use interface dispatching than to use a switch statement.
	// On my "cpu: Intel(R) Core(TM) i7-6820HQ CPU @ 2.70GHz", a switch statement
	// implementation tooks approx 940-980 ns/op. Interface dispatching is in the
	// range of 900-930 ns/op.
	node.WalkChildren(v)
	v.Visit(nil)
}

// WalkItemSlice traverses an item slice.
func WalkItemSlice(v Visitor, ins ItemSlice) {
	for _, in := range ins {
		Walk(v, in)
	}
}

// WalkDescriptionSlice traverses an item slice.
func WalkDescriptionSlice(v Visitor, dns DescriptionSlice) {
	for _, dn := range dns {
		Walk(v, dn)
	}
}
