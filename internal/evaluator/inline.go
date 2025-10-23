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
	var result *sx.Pair
	for bn := range blocks.Pairs() {
		blk := bn.Head()
		if sym, isSymbol := sx.GetSymbol(blk.Car()); isSymbol {
			switch sym {
			case zsx.SymPara:
				inlines := zsx.GetPara(blk)
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymHeading:
				_, _, inlines, _, frag := zsx.GetHeading(blk)
				if frag == fragment {
					return firstInlinesToEmbed(bn.Tail())
				}
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymRegionBlock, zsx.SymRegionQuote, zsx.SymRegionVerse:
				_, _, regionBlocks, inlines := zsx.GetRegion(blk)
				if result = findFragmentInBlocks(regionBlocks, fragment); result == nil {
					result = findFragmentInInlines(inlines, fragment)
				}

			case zsx.SymListOrdered, zsx.SymListUnordered, zsx.SymListQuote:
				_, _, items := zsx.GetList(blk)
				for itemNode := range items.Pairs() {
					itemBlocks := zsx.GetBlock(itemNode.Head())
					if result = findFragmentInBlocks(itemBlocks, fragment); result != nil {
						return result
					}
				}

			case zsx.SymDescription:
				_, termVals := zsx.GetDescription(blk)
				for n := termVals; n != nil; n = n.Tail() {
					if result = findFragmentInInlines(n.Head(), fragment); result != nil {
						return result
					}
					if n = n.Tail(); n == nil {
						break
					}
					for valBlkNode := range zsx.GetBlock(n.Head()).Pairs() {
						valBlocks := zsx.GetBlock(valBlkNode.Head())
						if result = findFragmentInBlocks(valBlocks, fragment); result != nil {
							return result
						}
					}

				}

			case zsx.SymTable:
				_, headerRow, rows := zsx.GetTable(blk)
				if result = findFragmentInRow(headerRow, fragment); result != nil {
					return result
				}
				for row := range rows.Pairs() {
					if result = findFragmentInRow(row.Head(), fragment); result != nil {
						return result
					}
				}

			case zsx.SymBLOB:
				_, _, _, inlines := zsx.GetBLOBuncode(blk)
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymTransclude:
				_, _, inlines := zsx.GetTransclusion(blk)
				result = findFragmentInInlines(inlines, fragment)
			}
			if result != nil {
				return result
			}
		}
	}
	return nil
}

func findFragmentInRow(row *sx.Pair, fragment string) *sx.Pair {
	for cell := range row.Pairs() {
		_, inlines := zsx.GetCell(cell.Head())
		return findFragmentInInlines(inlines, fragment)
	}
	return nil
}

func findFragmentInInlines(ins *sx.Pair, fragment string) *sx.Pair {
	var result *sx.Pair
	for in := range ins.Pairs() {
		inl := in.Head()
		if sym, isSymbol := sx.GetSymbol(inl.Car()); isSymbol {
			switch sym {
			case zsx.SymLink:
				_, _, inlines := zsx.GetLink(inl)
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymEmbed:
				_, _, _, inlines := zsx.GetEmbed(inl)
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymEmbedBLOB:
				_, _, _, inlines := zsx.GetEmbedBLOBuncode(inl)
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymCite:
				_, _, inlines := zsx.GetCite(inl)
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymMark:
				_, _, frag, inlines := zsx.GetMark(inl)
				if frag == fragment {
					next := in.Tail()
					for ; next != nil; next = next.Tail() {
						car := next.Head().Car()
						if !zsx.SymSoft.IsEqual(car) && !zsx.SymHard.IsEqual(car) {
							break
						}
					}
					if next == nil { // Mark is last in inline list
						return inlines
					}

					var lb sx.ListBuilder
					if inlines != nil {
						lb.Collect(inlines.Values())
					}
					lb.Collect(next.Values())
					return lb.List()
				}
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymEndnote:
				_, inlines := zsx.GetEndnote(inl)
				result = findFragmentInInlines(inlines, fragment)

			case zsx.SymFormatDelete, zsx.SymFormatEmph, zsx.SymFormatInsert, zsx.SymFormatMark,
				zsx.SymFormatQuote, zsx.SymFormatSpan, zsx.SymFormatStrong, zsx.SymFormatSub,
				zsx.SymFormatSuper:
				_, _, inlines := zsx.GetFormat(inl)
				result = findFragmentInInlines(inlines, fragment)
			}
			if result != nil {
				return result
			}
		}
	}
	return nil
}

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
