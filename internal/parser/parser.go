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

// Package parser provides a generic interface to a range of different parsers.
package parser

import (
	"context"
	"fmt"
	"maps"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsc/sz/zmk"
	"t73f.de/r/zsx"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/zettel"
)

// Info describes a single parser.
//
// Before Parse() is called, ensure the input stream to be valid. This can be
// achieved on calling inp.Next() after the input stream was created.
type Info struct {
	Name          string
	AltNames      []string
	IsASTParser   bool
	IsTextFormat  bool
	IsImageFormat bool

	// Parse the input, with the given metadata, the given syntax, and the given config.
	Parse func(*input.Input, *meta.Meta, string, *sx.Pair) *sx.Pair
}

var registry map[string]*Info

func init() {
	localRegistry := map[string]*Info{
		meta.ValueSyntaxCSS: {
			IsASTParser:   false,
			IsTextFormat:  true,
			IsImageFormat: false,
			Parse:         parsePlain,
		},
		meta.ValueSyntaxDraw: {
			IsASTParser:   true,
			IsTextFormat:  true,
			IsImageFormat: false,
			Parse:         parseDraw,
		},
		meta.ValueSyntaxGif: {
			IsASTParser:   false,
			IsTextFormat:  false,
			IsImageFormat: true,
			Parse:         parseBlob,
		},
		meta.ValueSyntaxHTML: {
			IsASTParser:   false,
			IsTextFormat:  true,
			IsImageFormat: false,
			Parse:         parsePlain,
		},
		meta.ValueSyntaxJPEG: {
			AltNames:      []string{meta.ValueSyntaxJPG},
			IsASTParser:   false,
			IsTextFormat:  false,
			IsImageFormat: true,
			Parse:         parseBlob,
		},
		meta.ValueSyntaxMarkdown: {
			AltNames:      []string{meta.ValueSyntaxMD},
			IsASTParser:   true,
			IsTextFormat:  true,
			IsImageFormat: false,
			Parse:         parseMarkdown,
		},
		meta.ValueSyntaxNone: {
			IsASTParser:   false,
			IsTextFormat:  false,
			IsImageFormat: false,
			Parse: func(inp *input.Input, _ *meta.Meta, _ string, _ *sx.Pair) *sx.Pair {
				return sz.ParseNoneBlocks(inp)
			},
		},
		meta.ValueSyntaxPNG: {
			IsASTParser:   false,
			IsTextFormat:  false,
			IsImageFormat: true,
			Parse:         parseBlob,
		},
		meta.ValueSyntaxSVG: {
			IsASTParser:   false,
			IsTextFormat:  true,
			IsImageFormat: true,
			Parse:         parsePlainSVG,
		},
		meta.ValueSyntaxSxn: {
			IsASTParser:   false,
			IsTextFormat:  true,
			IsImageFormat: false,
			Parse:         parsePlainSxn,
		},
		meta.ValueSyntaxTxt: {
			AltNames:      []string{meta.ValueSyntaxPlain, meta.ValueSyntaxText},
			IsASTParser:   false,
			IsTextFormat:  true,
			IsImageFormat: false,
			Parse:         parsePlain,
		},
		meta.ValueSyntaxWebp: {
			IsASTParser:   false,
			IsTextFormat:  false,
			IsImageFormat: true,
			Parse:         parseBlob,
		},
		meta.ValueSyntaxZmk: {
			IsASTParser:   true,
			IsTextFormat:  true,
			IsImageFormat: false,
			Parse: func(inp *input.Input, _ *meta.Meta, _ string, _ *sx.Pair) *sx.Pair {
				var zmkParser zmk.Parser
				zmkParser.Initialize(inp) // TODO: add alst
				return zmkParser.Parse()
			},
		},
	}

	registry = maps.Clone(localRegistry)
	for k, i := range localRegistry {
		i.Name = k
		for _, alt := range i.AltNames {
			if other, found := registry[alt]; found && other != i {
				panic(fmt.Sprintf("Parser %q already registered", alt))
			}
			registry[alt] = i
		}
	}
}

func parseBlob(inp *input.Input, m *meta.Meta, syntax string, _ *sx.Pair) *sx.Pair {
	if p := Get(syntax); p != nil {
		syntax = p.Name
	}
	return zsx.MakeBlock(zsx.MakeBLOB(nil, syntax, inp.Src, ParseDescription(m)))
}

// GetSyntaxes returns a list of syntaxes implemented by all registered parsers.
func GetSyntaxes() []string {
	result := make([]string, 0, len(registry))
	for syntax := range registry {
		result = append(result, syntax)
	}
	return result
}

// Get the parser (info) by name. If name not found, use a default parser.
func Get(name string) *Info {
	if pi := registry[name]; pi != nil {
		return pi
	}
	if pi := registry["plain"]; pi != nil {
		return pi
	}
	panic(fmt.Sprintf("No parser for %q found", name))
}

// IsASTParser returns whether the given syntax parses text into an AST or not.
func IsASTParser(syntax string) bool {
	pi, ok := registry[syntax]
	if !ok {
		return false
	}
	return pi.IsASTParser
}

// IsImageFormat returns whether the given syntax is known to be an image format.
func IsImageFormat(syntax string) bool {
	pi, ok := registry[syntax]
	if !ok {
		return false
	}
	return pi.IsImageFormat
}

// Parse parses some input and returns both a Sx.Object and a slice of block nodes.
func Parse(inp *input.Input, m *meta.Meta, syntax string, alst *sx.Pair) *sx.Pair {
	return Get(syntax).Parse(inp, m, syntax, alst)
}

// SymAllowHTML signals a parser to allow HTML content during parsing.
var SymAllowHTML = sx.MakeSymbol("ALLOW-HTML")

// ParseDescription returns a suitable description stored in the metadata as an inline list.
// This is done for an image in most cases.
func ParseDescription(m *meta.Meta) *sx.Pair {
	if m == nil {
		return nil
	}
	if summary, found := m.Get(meta.KeySummary); found {
		return sx.Cons(zsx.MakeText(sz.NormalizedSpacedText(string(summary))), sx.Nil())
	}
	if title, found := m.Get(meta.KeyTitle); found {
		return sx.Cons(zsx.MakeText(sz.NormalizedSpacedText(string(title))), sx.Nil())
	}
	return sx.Cons(zsx.MakeText("Zettel without title/summary: "+m.Zid.String()), sx.Nil())
}

// ParseZettel parses the zettel based on the syntax.
func ParseZettel(ctx context.Context, zettel zettel.Zettel, syntax string, rtConfig config.Config) *ast.Zettel {
	m := zettel.Meta
	inhMeta := m
	if rtConfig != nil {
		inhMeta = rtConfig.AddDefaultValues(ctx, inhMeta)
	}
	if syntax == "" {
		syntax = string(inhMeta.GetDefault(meta.KeySyntax, meta.DefaultSyntax))
	}
	var alst *sx.Pair
	if rtConfig != nil && rtConfig.GetHTMLInsecurity().AllowHTML(syntax) {
		alst = alst.Cons(sx.Cons(SymAllowHTML, nil))
	}

	parseMeta := inhMeta
	if syntax == meta.ValueSyntaxNone {
		parseMeta = m
	}

	rootNode := Parse(input.NewInput(zettel.Content.AsBytes()), parseMeta, syntax, alst)
	return &ast.Zettel{
		Meta:    m,
		Content: zettel.Content,
		Zid:     m.Zid,
		InhMeta: inhMeta,
		Blocks:  rootNode,
		Syntax:  syntax,
	}
}
