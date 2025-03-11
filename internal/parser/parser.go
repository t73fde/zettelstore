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

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/input"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/parser/cleaner"
	"zettelstore.de/z/internal/zettel"
)

// Info describes a single parser.
//
// Before ParseBlocks() or ParseInlines() is called, ensure the input stream to
// be valid. This can ce achieved on calling inp.Next() after the input stream
// was created.
type Info struct {
	Name          string
	AltNames      []string
	IsASTParser   bool
	IsTextFormat  bool
	IsImageFormat bool
	Parse         func(*input.Input, *meta.Meta, string) ast.BlockSlice
}

var registry = map[string]*Info{}

// Register the parser (info) for later retrieval.
func Register(pi *Info) {
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

// ParseBlocks parses some input and returns a slice of block nodes.
func ParseBlocks(inp *input.Input, m *meta.Meta, syntax string, hi config.HTMLInsecurity) ast.BlockSlice {
	bs := Get(syntax).Parse(inp, m, syntax)
	cleaner.CleanBlockSlice(&bs, hi.AllowHTML(syntax))
	return bs
}

// ParseDescription returns a suitable description stored in the metadata as an inline slice.
// This is done for an image in most cases.
func ParseDescription(m *meta.Meta) ast.InlineSlice {
	if m == nil {
		return nil
	}
	if summary, found := m.Get(meta.KeySummary); found {
		return ast.ParseSpacedText(string(summary))
	}
	if title, found := m.Get(meta.KeyTitle); found {
		return ast.ParseSpacedText(string(title))
	}
	return ast.InlineSlice{&ast.TextNode{Text: "Zettel without title/summary: " + m.Zid.String()}}
}

// ParseZettel parses the zettel based on the syntax.
func ParseZettel(ctx context.Context, zettel zettel.Zettel, syntax string, rtConfig config.Config) *ast.ZettelNode {
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

	hi := config.NoHTML
	if rtConfig != nil {
		hi = rtConfig.GetHTMLInsecurity()
	}
	bs := ParseBlocks(input.NewInput(zettel.Content.AsBytes()), parseMeta, syntax, hi)
	return &ast.ZettelNode{
		Meta:      m,
		Content:   zettel.Content,
		Zid:       m.Zid,
		InhMeta:   inhMeta,
		BlocksAST: bs,
		Syntax:    syntax,
	}
}
