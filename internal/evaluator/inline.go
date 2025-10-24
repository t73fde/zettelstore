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

// Package evaluator interprets and evaluates the AST.
package evaluator

import (
	"errors"
	"path"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"

	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/parser"
)

func (e *evaluator) evalLink(node *sx.Pair) *sx.Pair {
	attrs, ref, inlines := zsx.GetLink(node)
	refState, refVal := zsx.GetReference(ref)
	newInlines := inlines
	if inlines == nil {
		newInlines = sx.MakeList(zsx.MakeText(refVal))
	}
	if !sz.SymRefStateZettel.IsEqualSymbol(refState) {
		if newInlines != inlines {
			return zsx.MakeLink(attrs, ref, newInlines)
		}
		return node
	}

	zid := mustParseZid(ref, refVal)
	_, err := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
	if errors.Is(err, &box.ErrNotAllowed{}) {
		return zsx.MakeFormat(zsx.SymFormatSpan, attrs, newInlines)
	}
	if err != nil {
		return zsx.MakeLink(attrs, zsx.MakeReference(sz.SymRefStateBroken, refVal), newInlines)
	}

	if newInlines != inlines {
		return zsx.MakeLink(attrs, ref, newInlines)
	}
	return node
}

func (e *evaluator) evalEmbed(en *sx.Pair) *sx.Pair {
	attrs, ref, _, inlines := zsx.GetEmbed(en)
	refSym, refVal := zsx.GetReference(ref)

	// To prevent e.embedCount from counting
	if errText := e.checkMaxTransclusions(ref); errText != nil {
		return errText
	}

	if !sz.SymRefStateZettel.IsEqualSymbol(refSym) {
		switch refSym {
		case zsx.SymRefStateInvalid, sz.SymRefStateBroken:
			e.transcludeCount++
			return createInlineErrorImage(attrs, inlines)
		case zsx.SymRefStateSelf:
			e.transcludeCount++
			return createInlineErrorText(ref, "Self embed reference")
		case sz.SymRefStateFound, zsx.SymRefStateExternal:
			return en
		case zsx.SymRefStateHosted, sz.SymRefStateBased:
			if n := createLocalEmbedded(attrs, ref, refVal, inlines); n != nil {
				return n
			}
			return en
		case sz.SymRefStateQuery:
			return createInlineErrorText(ref, "Query reference not allowed here")
		default:
			return createInlineErrorText(ref, "Illegal inline state "+refSym.GetValue())
		}
	}

	zid := mustParseZid(ref, refVal)
	zettel, err := e.port.GetZettel(box.NoEnrichContext(e.ctx), zid)
	if err != nil {
		if errors.Is(err, &box.ErrNotAllowed{}) {
			return nil
		}
		e.transcludeCount++
		return createInlineErrorImage(attrs, inlines)
	}

	if syntax := string(zettel.Meta.GetDefault(meta.KeySyntax, meta.DefaultSyntax)); parser.IsImageFormat(syntax) {
		return e.updateImageRefNode(attrs, ref, inlines, zettel.Meta, syntax)
	} else if !parser.IsASTParser(syntax) {
		// Not embeddable.
		e.transcludeCount++
		return createInlineErrorText(ref, "Not embeddable (syntax="+syntax+")")
	}

	cost, ok := e.costMap[zid]
	zn := cost.zn
	if zn == e.marker {
		e.transcludeCount++
		return createInlineErrorText(ref, "Recursive transclusion")
	}
	if !ok {
		ec := e.transcludeCount
		e.costMap[zid] = transcludeCost{zn: e.marker, ec: ec}
		zn = e.evaluateEmbeddedZettel(zettel)
		e.costMap[zid] = transcludeCost{zn: zn, ec: e.transcludeCount - ec}
		e.transcludeCount = 0 // No stack needed, because embedding is done left-recursive, depth-first.
	}
	e.transcludeCount++

	result, ok := e.embedMap[refVal]
	if !ok {
		// Search for text to be embedded.
		_, fragment := sz.SplitFragment(refVal)
		blocks := zsx.GetBlock(zn.Blocks)
		if fragment == "" {
			result = firstInlinesToEmbed(blocks)
		} else {
			result = findFragmentInBlocks(blocks, fragment)
		}
		e.embedMap[refVal] = result
	}
	if result == nil {
		return zsx.MakeLiteral(zsx.SymLiteralComment,
			sx.MakeList(sx.Cons(sx.MakeString("-"), sx.MakeString(""))),
			"Nothing to transclude: "+sz.ReferenceString(ref),
		)
	}

	if ec := cost.ec; ec > 0 {
		e.transcludeCount += cost.ec
	}
	if result.Tail() == nil {
		return result.Head()
	}
	return result.Cons(zsx.SymSpecialSplice)
}

func (e *evaluator) updateImageRefNode(
	attrs *sx.Pair, ref *sx.Pair, inlines *sx.Pair, m *meta.Meta, syntax string,
) *sx.Pair {
	if inlines != nil {
		if is := parser.ParseDescription(m); is != nil {
			if is = mustPair(zsx.Walk(e, is, nil)); is != nil {
				inlines = is
			}
		}
	}
	return zsx.MakeEmbed(attrs, ref, syntax, inlines)
}

func findFragmentInBlocks(blocks *sx.Pair, fragment string) *sx.Pair {
	fs := fragmentSearcher{fragment: fragment}
	zsx.WalkItList(&fs, blocks, 0, nil)
	return fs.result
}

type fragmentSearcher struct {
	result   *sx.Pair
	fragment string
}

func (fs *fragmentSearcher) VisitItBefore(node *sx.Pair, alst *sx.Pair) bool {
	if fs.result != nil {
		return true
	}
	if sym, isSymbol := sx.GetSymbol(node.Car()); isSymbol {
		switch sym {
		case zsx.SymHeading:
			_, _, _, _, frag := zsx.GetHeading(node)
			if frag == fs.fragment {
				bn := zsx.GetWalkList(alst)
				fs.result = firstInlinesToEmbed(bn.Tail())
				return true
			}

		case zsx.SymMark:
			_, _, frag, inlines := zsx.GetMark(node)
			if frag == fs.fragment {
				next := zsx.GetWalkList(alst).Tail()
				for ; next != nil; next = next.Tail() {
					car := next.Head().Car()
					if !zsx.SymSoft.IsEqual(car) && !zsx.SymHard.IsEqual(car) {
						break
					}
				}
				if next == nil { // Mark is last in inline list
					fs.result = inlines
					return true
				}

				var lb sx.ListBuilder
				if inlines != nil {
					lb.Collect(inlines.Values())
				}
				lb.Collect(next.Values())
				fs.result = lb.List()
				return true
			}

		}
	}
	return false
}
func (fs *fragmentSearcher) VisitItAfter(*sx.Pair, *sx.Pair) {}

func firstInlinesToEmbed(blocks *sx.Pair) *sx.Pair {
	if blocks != nil {
		if ins := firstParagraphInlines(blocks); ins != nil {
			return ins
		}

		blk := blocks.Head()
		if sym, isSymbol := sx.GetSymbol(blk.Car()); isSymbol && zsx.SymBLOB.IsEqualSymbol(sym) {
			attrs, syntax, content, inlines := zsx.GetBLOBuncode(blk)
			return sx.MakeList(zsx.MakeEmbedBLOBuncode(attrs, syntax, content, inlines))
		}
	}
	return nil
}

// firstParagraphInlines returns the inline list of the first paragraph that
// contains a inline list.
func firstParagraphInlines(blocks *sx.Pair) *sx.Pair {
	for blockObj := range blocks.Values() {
		if block, isPair := sx.GetPair(blockObj); isPair {
			if sym, isSymbol := sx.GetSymbol(block.Car()); isSymbol && zsx.SymPara.IsEqualSymbol(sym) {
				if inlines := zsx.GetPara(block); inlines != nil {
					return inlines
				}
			}
		}
	}
	return nil
}

func createLocalEmbedded(attrs *sx.Pair, ref *sx.Pair, refValue string, inlines *sx.Pair) *sx.Pair {
	ext := path.Ext(refValue)
	if ext != "" && ext[0] == '.' {
		ext = ext[1:]
	}
	pinfo := parser.Get(ext)
	if pinfo == nil || !pinfo.IsImageFormat {
		return nil
	}
	return zsx.MakeEmbed(attrs, ref, ext, inlines)
}
