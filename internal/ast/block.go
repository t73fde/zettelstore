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

import "t73f.de/r/zsx"

// Definition of Block nodes.

// BlockSlice is a slice of BlockNodes.
type BlockSlice []BlockNode

func (*BlockSlice) blockNode() { /* Just a marker */ }

// WalkChildren walks down to the descriptions.
func (bs *BlockSlice) WalkChildren(v Visitor) {
	if bs != nil {
		for _, bn := range *bs {
			Walk(v, bn)
		}
	}
}

// FirstParagraphInlines returns the inline list of the first paragraph that
// contains a inline list.
func (bs BlockSlice) FirstParagraphInlines() InlineSlice {
	for _, bn := range bs {
		pn, ok := bn.(*ParaNode)
		if !ok {
			continue
		}
		if inl := pn.Inlines; len(inl) > 0 {
			return inl
		}
	}
	return nil
}

//--------------------------------------------------------------------------

// ParaNode contains just a sequence of inline elements.
// Another name is "paragraph".
type ParaNode struct {
	Inlines InlineSlice
}

func (*ParaNode) blockNode()       { /* Just a marker */ }
func (*ParaNode) itemNode()        { /* Just a marker */ }
func (*ParaNode) descriptionNode() { /* Just a marker */ }

// CreateParaNode creates a parameter block from inline nodes.
func CreateParaNode(nodes ...InlineNode) *ParaNode { return &ParaNode{Inlines: nodes} }

// WalkChildren walks down the inline elements.
func (pn *ParaNode) WalkChildren(v Visitor) { Walk(v, &pn.Inlines) }

//--------------------------------------------------------------------------

// VerbatimNode contains uninterpreted text
type VerbatimNode struct {
	Kind    VerbatimKind
	Attrs   zsx.Attributes
	Content []byte
}

// VerbatimKind specifies the format that is applied to code inline nodes.
type VerbatimKind int

// Constants for VerbatimCode
const (
	_               VerbatimKind = iota
	VerbatimZettel               // Zettel content
	VerbatimCode                 // Program code
	VerbatimEval                 // Code to be externally interpreted. Syntax is stored in default attribute.
	VerbatimComment              // Block comment
	VerbatimHTML                 // Block HTML, e.g. for Markdown
	VerbatimMath                 // Block math mode
)

func (*VerbatimNode) blockNode() { /* Just a marker */ }
func (*VerbatimNode) itemNode()  { /* Just a marker */ }

// WalkChildren does nothing.
func (*VerbatimNode) WalkChildren(Visitor) { /* No children*/ }

//--------------------------------------------------------------------------

// RegionNode encapsulates a region of block nodes.
type RegionNode struct {
	Kind    RegionKind
	Attrs   zsx.Attributes
	Blocks  BlockSlice
	Inlines InlineSlice // Optional text at the end of the region
}

// RegionKind specifies the actual region type.
type RegionKind int

// Values for RegionCode
const (
	_           RegionKind = iota
	RegionSpan             // Just a span of blocks
	RegionQuote            // A longer quotation
	RegionVerse            // Line breaks matter
)

func (*RegionNode) blockNode() { /* Just a marker */ }
func (*RegionNode) itemNode()  { /* Just a marker */ }

// WalkChildren walks down the blocks and the text.
func (rn *RegionNode) WalkChildren(v Visitor) {
	Walk(v, &rn.Blocks)
	Walk(v, &rn.Inlines)
}

//--------------------------------------------------------------------------

// HeadingNode stores the heading text and level.
type HeadingNode struct {
	Level    int
	Attrs    zsx.Attributes
	Slug     string      // Heading text, normalized
	Fragment string      // Heading text, suitable to be used as an unique URL fragment
	Inlines  InlineSlice // Heading text, possibly formatted
}

func (*HeadingNode) blockNode() { /* Just a marker */ }
func (*HeadingNode) itemNode()  { /* Just a marker */ }

// WalkChildren walks the heading text.
func (hn *HeadingNode) WalkChildren(v Visitor) { Walk(v, &hn.Inlines) }

//--------------------------------------------------------------------------

// HRuleNode specifies a horizontal rule.
type HRuleNode struct {
	Attrs zsx.Attributes
}

func (*HRuleNode) blockNode() { /* Just a marker */ }
func (*HRuleNode) itemNode()  { /* Just a marker */ }

// WalkChildren does nothing.
func (*HRuleNode) WalkChildren(Visitor) { /* No children*/ }

//--------------------------------------------------------------------------

// NestedListNode specifies a nestable list, either ordered or unordered.
type NestedListNode struct {
	Kind  NestedListKind
	Attrs zsx.Attributes
	Items []ItemSlice
}

// NestedListKind specifies the actual list type.
type NestedListKind uint8

// Values for ListCode
const (
	_                   NestedListKind = iota
	NestedListOrdered                  // Ordered list.
	NestedListUnordered                // Unordered list.
	NestedListQuote                    // Quote list.
)

func (*NestedListNode) blockNode() { /* Just a marker */ }
func (*NestedListNode) itemNode()  { /* Just a marker */ }

// WalkChildren walks down the items.
func (ln *NestedListNode) WalkChildren(v Visitor) {
	if items := ln.Items; items != nil {
		for _, item := range items {
			WalkItemSlice(v, item)
		}
	}
}

//--------------------------------------------------------------------------

// DescriptionListNode specifies a description list.
type DescriptionListNode struct {
	Attrs        zsx.Attributes
	Descriptions []Description
}

// Description is one element of a description list.
type Description struct {
	Term         InlineSlice
	Descriptions []DescriptionSlice
}

func (*DescriptionListNode) blockNode() { /* Just a marker */ }

// WalkChildren walks down to the descriptions.
func (dn *DescriptionListNode) WalkChildren(v Visitor) {
	if descrs := dn.Descriptions; descrs != nil {
		for i, desc := range descrs {
			if len(desc.Term) > 0 {
				Walk(v, &descrs[i].Term) // Otherwise, changes in desc.Term will not go back into AST
			}
			if dss := desc.Descriptions; dss != nil {
				for _, dns := range dss {
					WalkDescriptionSlice(v, dns)
				}
			}
		}
	}
}

//--------------------------------------------------------------------------

// TableNode specifies a full table
type TableNode struct {
	Header TableRow    // The header row
	Align  []Alignment // Default column alignment
	Rows   []TableRow  // The slice of cell rows
}

// TableCell contains the data for one table cell
type TableCell struct {
	Align   Alignment   // Cell alignment
	Inlines InlineSlice // Cell content
}

// TableRow is a slice of cells.
type TableRow []*TableCell

// Alignment specifies text alignment.
// Currently only for tables.
type Alignment int

// Constants for Alignment.
const (
	_            Alignment = iota
	AlignDefault           // Default alignment, inherited
	AlignLeft              // Left alignment
	AlignCenter            // Center the content
	AlignRight             // Right alignment
)

func (*TableNode) blockNode() { /* Just a marker */ }

// WalkChildren walks down to the cells.
func (tn *TableNode) WalkChildren(v Visitor) {
	if header := tn.Header; header != nil {
		for i := range header {
			Walk(v, header[i]) // Otherwise changes will not go back
		}
	}
	if rows := tn.Rows; rows != nil {
		for _, row := range rows {
			for i := range row {
				Walk(v, &row[i].Inlines) // Otherwise changes will not go back
			}
		}
	}
}

// WalkChildren walks the list of inline elements.
func (cell *TableCell) WalkChildren(v Visitor) {
	Walk(v, &cell.Inlines) // Otherwise changes will not go back
}

//--------------------------------------------------------------------------

// TranscludeNode specifies block content from other zettel to embedded in
// current zettel
type TranscludeNode struct {
	Attrs   zsx.Attributes
	Ref     *Reference
	Inlines InlineSlice // Optional text.
}

func (*TranscludeNode) blockNode() { /* Just a marker */ }

// WalkChildren walks the associated text.
func (tn *TranscludeNode) WalkChildren(v Visitor) { Walk(v, &tn.Inlines) }

//--------------------------------------------------------------------------

// BLOBNode contains just binary data that must be interpreted according to
// a syntax.
type BLOBNode struct {
	Attrs       zsx.Attributes
	Description InlineSlice
	Syntax      string
	Blob        []byte
}

func (*BLOBNode) blockNode() { /* Just a marker */ }

// WalkChildren does nothing.
func (bn *BLOBNode) WalkChildren(v Visitor) { Walk(v, &bn.Description) }
