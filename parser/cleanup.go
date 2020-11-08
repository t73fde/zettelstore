//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// Zettelstore is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with Zettelstore. If not, see <http://www.gnu.org/licenses/>.
//-----------------------------------------------------------------------------

// Package parser provides a generic interface to a range of different parsers.
package parser

import (
	"strings"

	"zettelstore.de/z/ast"
	"zettelstore.de/z/encoder"
	"zettelstore.de/z/strfun"

	// Ensure that the text encoder is available
	_ "zettelstore.de/z/encoder/textenc"
)

func cleanupBlockSlice(bs ast.BlockSlice) {
	cv := &cleanupVisitor{
		textEnc: encoder.Create("text"),
	}
	ast.NewTopDownTraverser(cv).VisitBlockSlice(bs)
}

type cleanupVisitor struct {
	textEnc encoder.Encoder
}

// VisitZettel does nothing.
func (cv *cleanupVisitor) VisitZettel(z *ast.Zettel) {}

// VisitVerbatim does nothing.
func (cv *cleanupVisitor) VisitVerbatim(vn *ast.VerbatimNode) {}

// VisitRegion does nothing.
func (cv *cleanupVisitor) VisitRegion(rn *ast.RegionNode) {}

// VisitHeading does nothing.
func (cv *cleanupVisitor) VisitHeading(hn *ast.HeadingNode) {
	if hn == nil || hn.Inlines == nil {
		return
	}
	var sb strings.Builder
	_, err := cv.textEnc.WriteInlines(&sb, hn.Inlines)
	if err != nil {
		return
	}
	hn.Slug = strfun.Slugify(sb.String())
}

// VisitHRule does nothing.
func (cv *cleanupVisitor) VisitHRule(hn *ast.HRuleNode) {}

// VisitList does nothing.
func (cv *cleanupVisitor) VisitNestedList(ln *ast.NestedListNode) {}

// VisitDescriptionList does nothing.
func (cv *cleanupVisitor) VisitDescriptionList(dn *ast.DescriptionListNode) {}

// VisitPara does nothing.
func (cv *cleanupVisitor) VisitPara(pn *ast.ParaNode) {}

// VisitTable does nothing.
func (cv *cleanupVisitor) VisitTable(tn *ast.TableNode) {}

// VisitBLOB does nothing.
func (cv *cleanupVisitor) VisitBLOB(bn *ast.BLOBNode) {}

// VisitText does nothing.
func (cv *cleanupVisitor) VisitText(tn *ast.TextNode) {}

// VisitTag does nothing.
func (cv *cleanupVisitor) VisitTag(tn *ast.TagNode) {}

// VisitSpace does nothing.
func (cv *cleanupVisitor) VisitSpace(sn *ast.SpaceNode) {}

// VisitBreak does nothing.
func (cv *cleanupVisitor) VisitBreak(bn *ast.BreakNode) {}

// VisitLink collects the given link as a reference.
func (cv *cleanupVisitor) VisitLink(ln *ast.LinkNode) {}

// VisitImage collects the image links as a reference.
func (cv *cleanupVisitor) VisitImage(in *ast.ImageNode) {}

// VisitCite does nothing.
func (cv *cleanupVisitor) VisitCite(cn *ast.CiteNode) {}

// VisitFootnote does nothing.
func (cv *cleanupVisitor) VisitFootnote(fn *ast.FootnoteNode) {}

// VisitMark does nothing.
func (cv *cleanupVisitor) VisitMark(mn *ast.MarkNode) {}

// VisitFormat does nothing.
func (cv *cleanupVisitor) VisitFormat(fn *ast.FormatNode) {}

// VisitLiteral does nothing.
func (cv *cleanupVisitor) VisitLiteral(ln *ast.LiteralNode) {}
