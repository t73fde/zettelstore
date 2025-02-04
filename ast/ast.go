//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

// Package ast provides the abstract syntax tree for parsed zettel content.
package ast

import (
	"net/url"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"zettelstore.de/z/zettel"
)

// ZettelNode is the root node of the abstract syntax tree.
// It is *not* part of the visitor pattern.
type ZettelNode struct {
	Meta    *meta.Meta     // Original metadata
	Content zettel.Content // Original content
	Zid     id.Zid         // Zettel identification.
	InhMeta *meta.Meta     // Metadata of the zettel, with inherited values.
	Ast     BlockSlice     // Zettel abstract syntax tree is a sequence of block nodes.
	Syntax  string         // Syntax / parser that produced the Ast
}

// Node is the interface, all nodes must implement.
type Node interface {
	WalkChildren(v Visitor)
}

// BlockNode is the interface that all block nodes must implement.
type BlockNode interface {
	Node
	blockNode()
}

// ItemNode is a node that can occur as a list item.
type ItemNode interface {
	BlockNode
	itemNode()
}

// ItemSlice is a slice of ItemNodes.
type ItemSlice []ItemNode

// DescriptionNode is a node that contains just textual description.
type DescriptionNode interface {
	ItemNode
	descriptionNode()
}

// DescriptionSlice is a slice of DescriptionNodes.
type DescriptionSlice []DescriptionNode

// InlineNode is the interface that all inline nodes must implement.
type InlineNode interface {
	Node
	inlineNode()
}

// Reference is a reference to external or internal material.
type Reference struct {
	URL   *url.URL
	Value string
	State RefState
}

// RefState indicates the state of the reference.
type RefState int

// Constants for RefState
const (
	RefStateInvalid  RefState = iota // Invalid Reference
	RefStateZettel                   // Reference to an internal zettel
	RefStateSelf                     // Reference to same zettel with a fragment
	RefStateFound                    // Reference to an existing internal zettel, URL is ajusted
	RefStateBroken                   // Reference to a non-existing internal zettel
	RefStateHosted                   // Reference to local hosted non-Zettel, without URL change
	RefStateBased                    // Reference to local non-Zettel, to be prefixed
	RefStateQuery                    // Reference to a zettel query
	RefStateExternal                 // Reference to external material
)
