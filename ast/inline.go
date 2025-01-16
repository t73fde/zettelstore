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

package ast

import (
	"t73f.de/r/zsc/attrs"
)

// Definitions of inline nodes.

// InlineSlice is a list of BlockNodes.
type InlineSlice []InlineNode

func (*InlineSlice) inlineNode() { /* Just a marker */ }

// WalkChildren walks down to the list.
func (is InlineSlice) WalkChildren(v Visitor) {
	for _, in := range is {
		Walk(v, in)
	}
}

// --------------------------------------------------------------------------

// TextNode just contains some text.
type TextNode struct {
	Text string // The text itself.
}

func (*TextNode) inlineNode() { /* Just a marker */ }

// WalkChildren does nothing.
func (*TextNode) WalkChildren(Visitor) { /* No children*/ }

// --------------------------------------------------------------------------

// BreakNode signals a new line that must / should be interpreted as a new line break.
type BreakNode struct {
	Hard bool // Hard line break?
}

func (*BreakNode) inlineNode() { /* Just a marker */ }

// WalkChildren does nothing.
func (*BreakNode) WalkChildren(Visitor) { /* No children*/ }

// --------------------------------------------------------------------------

// LinkNode contains the specified link.
type LinkNode struct {
	Attrs   attrs.Attributes // Optional attributes
	Ref     *Reference
	Inlines InlineSlice // The text associated with the link.
}

func (*LinkNode) inlineNode() { /* Just a marker */ }

// WalkChildren walks to the link text.
func (ln *LinkNode) WalkChildren(v Visitor) {
	if len(ln.Inlines) > 0 {
		Walk(v, &ln.Inlines)
	}
}

// --------------------------------------------------------------------------

// EmbedRefNode contains the specified embedded reference material.
type EmbedRefNode struct {
	Attrs   attrs.Attributes // Optional attributes
	Ref     *Reference       // The reference to be embedded.
	Syntax  string           // Syntax of referenced material, if known
	Inlines InlineSlice      // Optional text associated with the image.
}

func (*EmbedRefNode) inlineNode() { /* Just a marker */ }

// WalkChildren walks to the text that describes the embedded material.
func (en *EmbedRefNode) WalkChildren(v Visitor) { Walk(v, &en.Inlines) }

// --------------------------------------------------------------------------

// EmbedBLOBNode contains the specified embedded BLOB material.
type EmbedBLOBNode struct {
	Attrs   attrs.Attributes // Optional attributes
	Syntax  string           // Syntax of Blob
	Blob    []byte           // BLOB data itself.
	Inlines InlineSlice      // Optional text associated with the image.
}

func (*EmbedBLOBNode) inlineNode() { /* Just a marker */ }

// WalkChildren walks to the text that describes the embedded material.
func (en *EmbedBLOBNode) WalkChildren(v Visitor) { Walk(v, &en.Inlines) }

// --------------------------------------------------------------------------

// CiteNode contains the specified citation.
type CiteNode struct {
	Attrs   attrs.Attributes // Optional attributes
	Key     string           // The citation key
	Inlines InlineSlice      // Optional text associated with the citation.
}

func (*CiteNode) inlineNode() { /* Just a marker */ }

// WalkChildren walks to the cite text.
func (cn *CiteNode) WalkChildren(v Visitor) { Walk(v, &cn.Inlines) }

// --------------------------------------------------------------------------

// MarkNode contains the specified merked position.
// It is a BlockNode too, because although it is typically parsed during inline
// mode, it is moved into block mode afterwards.
type MarkNode struct {
	Mark     string      // The mark text itself
	Slug     string      // Slugified form of Mark
	Fragment string      // Unique form of Slug
	Inlines  InlineSlice // Marked inline content
}

func (*MarkNode) inlineNode() { /* Just a marker */ }

// WalkChildren does nothing.
func (mn *MarkNode) WalkChildren(v Visitor) {
	if len(mn.Inlines) > 0 {
		Walk(v, &mn.Inlines)
	}
}

// --------------------------------------------------------------------------

// FootnoteNode contains the specified footnote.
type FootnoteNode struct {
	Attrs   attrs.Attributes // Optional attributes
	Inlines InlineSlice      // The footnote text.
}

func (*FootnoteNode) inlineNode() { /* Just a marker */ }

// WalkChildren walks to the footnote text.
func (fn *FootnoteNode) WalkChildren(v Visitor) { Walk(v, &fn.Inlines) }

// --------------------------------------------------------------------------

// FormatNode specifies some inline formatting.
type FormatNode struct {
	Kind    FormatKind
	Attrs   attrs.Attributes // Optional attributes.
	Inlines InlineSlice
}

// FormatKind specifies the format that is applied to the inline nodes.
type FormatKind int

// Constants for FormatCode
const (
	_            FormatKind = iota
	FormatEmph              // Emphasized text
	FormatStrong            // Strongly emphasized text
	FormatInsert            // Inserted text
	FormatDelete            // Deleted text
	FormatSuper             // Superscripted text
	FormatSub               // SubscriptedText
	FormatQuote             // Quoted text
	FormatMark              // Marked text
	FormatSpan              // Generic inline container
)

func (*FormatNode) inlineNode() { /* Just a marker */ }

// WalkChildren walks to the formatted text.
func (fn *FormatNode) WalkChildren(v Visitor) { Walk(v, &fn.Inlines) }

// --------------------------------------------------------------------------

// LiteralNode specifies some uninterpreted text.
type LiteralNode struct {
	Kind    LiteralKind
	Attrs   attrs.Attributes // Optional attributes.
	Content []byte
}

// LiteralKind specifies the format that is applied to code inline nodes.
type LiteralKind int

// Constants for LiteralCode
const (
	_              LiteralKind = iota
	LiteralProg                // Inline program code
	LiteralInput               // Computer input, e.g. Keyboard strokes
	LiteralOutput              // Computer output
	LiteralComment             // Inline comment
	LiteralMath                // Inline math mode
)

func (*LiteralNode) inlineNode() { /* Just a marker */ }

// WalkChildren does nothing.
func (*LiteralNode) WalkChildren(Visitor) { /* No children*/ }
