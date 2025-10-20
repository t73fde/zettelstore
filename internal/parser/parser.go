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
	"log"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/ast/sztrans"
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
	Parse         func(*input.Input, *meta.Meta, string) *sx.Pair
}

var registry = map[string]*Info{}

// register the parser (info) for later retrieval.
func register(pi *Info) {
	if _, ok := registry[pi.Name]; ok {
		panic(fmt.Sprintf("Parser %q already registered", pi.Name))
	}
	registry[pi.Name] = pi
	for _, alt := range pi.AltNames {
		if _, ok := registry[alt]; ok {
			panic(fmt.Sprintf("Parser %q already registered", alt))
		}
		registry[alt] = pi
	}
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
func Parse(inp *input.Input, m *meta.Meta, syntax string) (*sx.Pair, ast.BlockSlice) {
	if obj := Get(syntax).Parse(inp, m, syntax); obj != nil {
		bs, err := sztrans.GetBlockSlice(obj)
		if err == nil {
			return obj, bs
		}
		log.Printf("sztrans error: %v, for %v\n", err, obj)
	}
	return nil, nil
}

// ParseDescriptionAST returns a suitable description stored in the metadata as an inline slice.
// This is done for an image in most cases.
func ParseDescriptionAST(m *meta.Meta) ast.InlineSlice {
	if m == nil {
		return nil
	}
	if summary, found := m.Get(meta.KeySummary); found {
		return ast.ParseSpacedTextAST(string(summary))
	}
	if title, found := m.Get(meta.KeyTitle); found {
		return ast.ParseSpacedTextAST(string(title))
	}
	return ast.InlineSlice{&ast.TextNode{Text: "Zettel without title/summary: " + m.Zid.String()}}
}

// ParseDescription returns a suitable description stored in the metadata as an inline list.
// This is done for an image in most cases.
func ParseDescription(m *meta.Meta) *sx.Pair {
	if m == nil {
		return nil
	}
	if summary, found := m.Get(meta.KeySummary); found {
		return sx.Cons(zsx.MakeText(ast.NormalizedSpacedText(string(summary))), sx.Nil())
	}
	if title, found := m.Get(meta.KeyTitle); found {
		return sx.Cons(zsx.MakeText(ast.NormalizedSpacedText(string(title))), sx.Nil())
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
	parseMeta := inhMeta
	if syntax == meta.ValueSyntaxNone {
		parseMeta = m
	}

	rootNode, bs := Parse(input.NewInput(zettel.Content.AsBytes()), parseMeta, syntax)
	return &ast.Zettel{
		Meta:      m,
		Content:   zettel.Content,
		Zid:       m.Zid,
		InhMeta:   inhMeta,
		Blocks:    rootNode,
		BlocksAST: bs,
		Syntax:    syntax,
	}
}
