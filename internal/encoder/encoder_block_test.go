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

package encoder_test

import "t73f.de/r/zsc/domain/meta"

// func TestEncoderBlock(t *testing.T) {
// 	executeTestCases(t, tcsBlock)
// }

var tcsBlock = []zmkTestCase{
	{
		descr: "Empty Zettelmarkup should produce near nothing",
		zmk:   "",
		expect: expectMap{
			encoderHTML:  "",
			encoderMD:    "",
			encoderSz:    `(BLOCK)`,
			encoderSHTML: `()`,
			encoderText:  "",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple text: Hello, world",
		zmk:   "Hello, world",
		expect: expectMap{
			encoderHTML:  "<p>Hello, world</p>",
			encoderMD:    "Hello, world",
			encoderSz:    `(BLOCK (PARA (TEXT "Hello, world")))`,
			encoderSHTML: `((p "Hello, world"))`,
			encoderText:  "Hello, world",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple block comment",
		zmk:   "%%%\nNo\nrender\n%%%",
		expect: expectMap{
			encoderHTML:  ``,
			encoderMD:    "",
			encoderSz:    `(BLOCK (VERBATIM-COMMENT () "No\nrender"))`,
			encoderSHTML: `(())`,
			encoderText:  ``,
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Rendered block comment",
		zmk:   "%%%{-}\nRender\n%%%",
		expect: expectMap{
			encoderHTML:  "<!--\nRender\n-->\n",
			encoderMD:    "",
			encoderSz:    `(BLOCK (VERBATIM-COMMENT (("-" . "")) "Render"))`,
			encoderSHTML: "((@@@ \"Render\"))",
			encoderText:  ``,
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple Heading",
		zmk:   `=== Top Job`,
		expect: expectMap{
			encoderHTML:  "<h2 id=\"top-job\">Top Job</h2>",
			encoderMD:    "# Top Job",
			encoderSz:    `(BLOCK (HEADING 1 () "top-job" "top-job" (TEXT "Top Job")))`,
			encoderSHTML: `((h2 ((id . "top-job")) "Top Job"))`,
			encoderText:  `Top Job`,
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple List",
		zmk:   "* A\n* B\n* C",
		expect: expectMap{
			encoderHTML:  "<ul><li>A</li><li>B</li><li>C</li></ul>",
			encoderMD:    "* A\n* B\n* C",
			encoderSz:    `(BLOCK (UNORDERED () (BLOCK (PARA (TEXT "A"))) (BLOCK (PARA (TEXT "B"))) (BLOCK (PARA (TEXT "C")))))`,
			encoderSHTML: `((ul (li "A") (li "B") (li "C")))`,
			encoderText:  "A\nB\nC",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Nested List",
		zmk:   "* T1\n*# T2\n* T3\n** T4\n** T5\n* T6",
		expect: expectMap{
			encoderHTML:  `<ul><li><p>T1</p><ol><li>T2</li></ol></li><li><p>T3</p><ul><li>T4</li><li>T5</li></ul></li><li><p>T6</p></li></ul>`,
			encoderMD:    "* T1\n    1. T2\n* T3\n    * T4\n    * T5\n* T6",
			encoderSz:    `(BLOCK (UNORDERED () (BLOCK (PARA (TEXT "T1")) (ORDERED () (BLOCK (PARA (TEXT "T2"))))) (BLOCK (PARA (TEXT "T3")) (UNORDERED () (BLOCK (PARA (TEXT "T4"))) (BLOCK (PARA (TEXT "T5"))))) (BLOCK (PARA (TEXT "T6")))))`,
			encoderSHTML: `((ul (li (p "T1") (ol (li "T2"))) (li (p "T3") (ul (li "T4") (li "T5"))) (li (p "T6"))))`,
			encoderText:  "T1\nT2\nT3\nT4\nT5\nT6",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Sequence of two lists",
		zmk:   "* Item1.1\n* Item1.2\n* Item1.3\n\n* Item2.1\n* Item2.2",
		expect: expectMap{
			encoderHTML:  "<ul><li>Item1.1</li><li>Item1.2</li><li>Item1.3</li><li>Item2.1</li><li>Item2.2</li></ul>",
			encoderMD:    "* Item1.1\n* Item1.2\n* Item1.3\n* Item2.1\n* Item2.2",
			encoderSz:    `(BLOCK (UNORDERED () (BLOCK (PARA (TEXT "Item1.1"))) (BLOCK (PARA (TEXT "Item1.2"))) (BLOCK (PARA (TEXT "Item1.3"))) (BLOCK (PARA (TEXT "Item2.1"))) (BLOCK (PARA (TEXT "Item2.2")))))`,
			encoderSHTML: `((ul (li "Item1.1") (li "Item1.2") (li "Item1.3") (li "Item2.1") (li "Item2.2")))`,
			encoderText:  "Item1.1\nItem1.2\nItem1.3\nItem2.1\nItem2.2",
			encoderZmk:   "* Item1.1\n* Item1.2\n* Item1.3\n* Item2.1\n* Item2.2",
		},
	},
	{
		descr: "Simple horizontal rule",
		zmk:   `---`,
		expect: expectMap{
			encoderHTML:  "<hr>",
			encoderMD:    "---",
			encoderSz:    `(BLOCK (THEMATIC ()))`,
			encoderSHTML: `((hr))`,
			encoderText:  ``,
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Thematic break with attribute",
		zmk:   `---{lang="zmk"}`,
		expect: expectMap{
			encoderHTML:  `<hr lang="zmk">`,
			encoderMD:    "---",
			encoderSz:    `(BLOCK (THEMATIC (("lang" . "zmk"))))`,
			encoderSHTML: `((hr ((lang . "zmk"))))`,
			encoderText:  ``,
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "No list after paragraph",
		zmk:   "Text\n*abc",
		expect: expectMap{
			encoderHTML:  "<p>Text *abc</p>",
			encoderMD:    "Text\n*abc",
			encoderSz:    `(BLOCK (PARA (TEXT "Text") (SOFT) (TEXT "*abc")))`,
			encoderSHTML: `((p "Text" " " "*abc"))`,
			encoderText:  `Text *abc`,
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "A list after paragraph",
		zmk:   "Text\n# abc",
		expect: expectMap{
			encoderHTML:  "<p>Text</p><ol><li>abc</li></ol>",
			encoderMD:    "Text\n\n1. abc",
			encoderSz:    `(BLOCK (PARA (TEXT "Text")) (ORDERED () (BLOCK (PARA (TEXT "abc")))))`,
			encoderSHTML: `((p "Text") (ol (li "abc")))`,
			encoderText:  "Text\nabc",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple List Quote",
		zmk:   "> ToBeOrNotToBe",
		expect: expectMap{
			encoderHTML:  "<blockquote>ToBeOrNotToBe</blockquote>",
			encoderMD:    "> ToBeOrNotToBe",
			encoderSz:    `(BLOCK (QUOTATION () (BLOCK (PARA (TEXT "ToBeOrNotToBe")))))`,
			encoderSHTML: `((blockquote (@L "ToBeOrNotToBe")))`,
			encoderText:  "ToBeOrNotToBe",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Nested list quotes", // Based on CommonMark example 250
		zmk:   ">>> foo\n    bar",
		expect: expectMap{
			encoderHTML:  `<blockquote><blockquote><blockquote>foo bar</blockquote></blockquote></blockquote>`,
			encoderSz:    `(BLOCK (QUOTATION () (BLOCK (QUOTATION () (BLOCK (QUOTATION () (BLOCK (PARA (TEXT "foo") (SOFT) (TEXT "bar")))))))))`,
			encoderSHTML: `((blockquote (@L (blockquote (@L (blockquote (@L "foo" " " "bar")))))))`,
			encoderText:  "foo bar",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple Quote Block",
		zmk:   "<<<\nToBeOrNotToBe\n<<< Romeo Julia",
		expect: expectMap{
			encoderHTML:  "<blockquote><p>ToBeOrNotToBe</p><cite>Romeo Julia</cite></blockquote>",
			encoderMD:    "> ToBeOrNotToBe",
			encoderSz:    `(BLOCK (REGION-QUOTE () ((PARA (TEXT "ToBeOrNotToBe"))) (TEXT "Romeo Julia")))`,
			encoderSHTML: `((blockquote (p "ToBeOrNotToBe") (cite "Romeo Julia")))`,
			encoderText:  "ToBeOrNotToBe\nRomeo Julia",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Quote Block with multiple paragraphs",
		zmk:   "<<<\nToBeOr\n\nNotToBe\n<<< Romeo",
		expect: expectMap{
			encoderHTML:  "<blockquote><p>ToBeOr</p><p>NotToBe</p><cite>Romeo</cite></blockquote>",
			encoderMD:    "> ToBeOr\n>\n> NotToBe",
			encoderSz:    `(BLOCK (REGION-QUOTE () ((PARA (TEXT "ToBeOr")) (PARA (TEXT "NotToBe"))) (TEXT "Romeo")))`,
			encoderSHTML: `((blockquote (p "ToBeOr") (p "NotToBe") (cite "Romeo")))`,
			encoderText:  "ToBeOr\nNotToBe\nRomeo",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Verse block",
		zmk: `"""
A line
  another line
Back

Paragraph

    Spacy  Para
""" Author`,
		expect: expectMap{
			encoderHTML:  "<div><p>A\u00a0line<br>\u00a0\u00a0another\u00a0line<br>Back</p><p>Paragraph</p><p>\u00a0\u00a0\u00a0\u00a0Spacy\u00a0\u00a0Para</p><cite>Author</cite></div>",
			encoderMD:    "",
			encoderSz:    "(BLOCK (REGION-VERSE () ((PARA (TEXT \"A\u00a0line\") (HARD) (TEXT \"\u00a0\u00a0another\u00a0line\") (HARD) (TEXT \"Back\")) (PARA (TEXT \"Paragraph\")) (PARA (TEXT \"\u00a0\u00a0\u00a0\u00a0Spacy\u00a0\u00a0Para\"))) (TEXT \"Author\")))",
			encoderSHTML: "((div (p \"A\u00a0line\" (br) \"\u00a0\u00a0another\u00a0line\" (br) \"Back\") (p \"Paragraph\") (p \"\u00a0\u00a0\u00a0\u00a0Spacy\u00a0\u00a0Para\") (cite \"Author\")))",
			encoderText:  "A line\n another line\nBack\nParagraph\n Spacy Para\nAuthor",
			encoderZmk:   "\"\"\"\nA\u00a0line\\\n\u00a0\u00a0another\u00a0line\\\nBack\nParagraph\n\u00a0\u00a0\u00a0\u00a0Spacy\u00a0\u00a0Para\n\"\"\" Author",
		},
	},
	{
		descr: "Span Block",
		zmk: `:::
A simple
   span
and much more
:::`,
		expect: expectMap{
			encoderHTML:  "<div><p>A simple    span and much more</p></div>",
			encoderMD:    "",
			encoderSz:    `(BLOCK (REGION-BLOCK () ((PARA (TEXT "A simple") (SOFT) (TEXT "   span") (SOFT) (TEXT "and much more")))))`,
			encoderSHTML: `((div (p "A simple" " " "   span" " " "and much more")))`,
			encoderText:  `A simple  span and much more`,
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple Verbatim Code",
		zmk:   "```\nHello\nWorld\n```",
		expect: expectMap{
			encoderHTML:  "<pre><code>Hello\nWorld</code></pre>",
			encoderMD:    "    Hello\n    World",
			encoderSz:    `(BLOCK (VERBATIM-CODE () "Hello\nWorld"))`,
			encoderSHTML: `((pre (code "Hello\nWorld")))`,
			encoderText:  "Hello\nWorld",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple Verbatim Code with visible spaces",
		zmk:   "```{-}\nHello World\n```",
		expect: expectMap{
			encoderHTML:  "<pre><code>Hello\u2423World</code></pre>",
			encoderMD:    "    Hello World",
			encoderSz:    `(BLOCK (VERBATIM-CODE (("-" . "")) "Hello World"))`,
			encoderSHTML: "((pre (code \"Hello\u2423World\")))",
			encoderText:  "Hello World",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple Verbatim Eval",
		zmk:   "~~~\nHello\nWorld\n~~~",
		expect: expectMap{
			encoderHTML:  "<pre><code class=\"zs-eval\">Hello\nWorld</code></pre>",
			encoderMD:    "",
			encoderSz:    `(BLOCK (VERBATIM-EVAL () "Hello\nWorld"))`,
			encoderSHTML: "((pre (code ((class . \"zs-eval\")) \"Hello\\nWorld\")))",
			encoderText:  "Hello\nWorld",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple Verbatim Math",
		zmk:   "$$$\nHello\n\\LaTeX\n$$$",
		expect: expectMap{
			encoderHTML:  "<pre><code class=\"zs-math\">Hello\n\\LaTeX</code></pre>",
			encoderMD:    "",
			encoderSz:    `(BLOCK (VERBATIM-MATH () "Hello\n\\LaTeX"))`,
			encoderSHTML: "((pre (code ((class . \"zs-math\")) \"Hello\\n\\\\LaTeX\")))",
			encoderText:  "Hello\n\\LaTeX",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple Description List",
		zmk:   "; Zettel\n: Paper\n: Note\n; Zettelkasten\n: Slip box",
		expect: expectMap{
			encoderHTML:  "<dl><dt>Zettel</dt><dd><p>Paper</p></dd><dd><p>Note</p></dd><dt>Zettelkasten</dt><dd><p>Slip box</p></dd></dl>",
			encoderMD:    "",
			encoderSz:    `(BLOCK (DESCRIPTION () ((TEXT "Zettel")) (BLOCK (BLOCK (PARA (TEXT "Paper"))) (BLOCK (PARA (TEXT "Note")))) ((TEXT "Zettelkasten")) (BLOCK (BLOCK (PARA (TEXT "Slip box"))))))`,
			encoderSHTML: `((dl (dt "Zettel") (dd (p "Paper")) (dd (p "Note")) (dt "Zettelkasten") (dd (p "Slip box"))))`,
			encoderText:  "Zettel\nPaper\nNote\nZettelkasten\nSlip box",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Description List with paragraphs as item",
		zmk:   "; Zettel\n: Paper\n\n  Note\n; Zettelkasten\n: Slip box",
		expect: expectMap{
			encoderHTML:  "<dl><dt>Zettel</dt><dd><p>Paper</p><p>Note</p></dd><dt>Zettelkasten</dt><dd><p>Slip box</p></dd></dl>",
			encoderMD:    "",
			encoderSz:    `(BLOCK (DESCRIPTION () ((TEXT "Zettel")) (BLOCK (BLOCK (PARA (TEXT "Paper")) (PARA (TEXT "Note")))) ((TEXT "Zettelkasten")) (BLOCK (BLOCK (PARA (TEXT "Slip box"))))))`,
			encoderSHTML: `((dl (dt "Zettel") (dd (p "Paper") (p "Note")) (dt "Zettelkasten") (dd (p "Slip box"))))`,
			encoderText:  "Zettel\nPaper\nNote\nZettelkasten\nSlip box",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Description List with keys, but no descriptions",
		zmk:   "; K1\n: D11\n: D12\n; K2\n; K3\n: D31",
		expect: expectMap{
			encoderHTML:  "<dl><dt>K1</dt><dd><p>D11</p></dd><dd><p>D12</p></dd><dt>K2</dt><dt>K3</dt><dd><p>D31</p></dd></dl>",
			encoderMD:    "",
			encoderSz:    `(BLOCK (DESCRIPTION () ((TEXT "K1")) (BLOCK (BLOCK (PARA (TEXT "D11"))) (BLOCK (PARA (TEXT "D12")))) ((TEXT "K2")) (BLOCK) ((TEXT "K3")) (BLOCK (BLOCK (PARA (TEXT "D31"))))))`,
			encoderSHTML: `((dl (dt "K1") (dd (p "D11")) (dd (p "D12")) (dt "K2") (dt "K3") (dd (p "D31"))))`,
			encoderText:  "K1\nD11\nD12\nK2\nK3\nD31",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Simple Table",
		zmk:   "|c1|c2|c3\n|d1||d3",
		expect: expectMap{
			encoderHTML:  `<table><tbody><tr><td>c1</td><td>c2</td><td>c3</td></tr><tr><td>d1</td><td></td><td>d3</td></tr></tbody></table>`,
			encoderMD:    "",
			encoderSz:    `(BLOCK (TABLE () () ((CELL () (TEXT "c1")) (CELL () (TEXT "c2")) (CELL () (TEXT "c3"))) ((CELL () (TEXT "d1")) (CELL ()) (CELL () (TEXT "d3")))))`,
			encoderSHTML: `((table (tbody (tr (td "c1") (td "c2") (td "c3")) (tr (td "d1") (td) (td "d3")))))`,
			encoderText:  "c1 c2 c3\nd1  d3",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Table with alignment and comment",
		zmk: `|h1>|=h2|h3:|
|%--+---+---+
|<c1|c2|:c3|
|f1|f2|=f3`,
		expect: expectMap{
			encoderHTML:  `<table><thead><tr><th class="right">h1</th><th>h2</th><th class="center">h3</th></tr></thead><tbody><tr><td class="left">c1</td><td>c2</td><td class="center">c3</td></tr><tr><td class="right">f1</td><td>f2</td><td class="center">=f3</td></tr></tbody></table>`,
			encoderMD:    "",
			encoderSz:    `(BLOCK (TABLE () ((CELL ((align . "right")) (TEXT "h1")) (CELL () (TEXT "h2")) (CELL ((align . "center")) (TEXT "h3"))) ((CELL ((align . "left")) (TEXT "c1")) (CELL () (TEXT "c2")) (CELL ((align . "center")) (TEXT "c3"))) ((CELL ((align . "right")) (TEXT "f1")) (CELL () (TEXT "f2")) (CELL ((align . "center")) (TEXT "=f3")))))`,
			encoderSHTML: `((table (thead (tr (th ((class . "right")) "h1") (th "h2") (th ((class . "center")) "h3"))) (tbody (tr (td ((class . "left")) "c1") (td "c2") (td ((class . "center")) "c3")) (tr (td ((class . "right")) "f1") (td "f2") (td ((class . "center")) "=f3")))))`,
			encoderText:  "h1 h2 h3\nc1 c2 c3\nf1 f2 =f3",
			encoderZmk: /*`|=h1>|=h2|=h3:
			|<c1|c2|c3
			|f1|f2|=f3`,*/
			`|=>h1|=h2|=:h3
|<c1|c2|:c3
|>f1|f2|:=f3`,
		},
	},
	{
		descr: "Simple Endnote",
		zmk:   `Text[^Endnote]`,
		expect: expectMap{
			encoderHTML:  "<p>Text<sup id=\"fnref:1\"><a class=\"zs-noteref\" href=\"#fn:1\" role=\"doc-noteref\">1</a></sup></p><ol class=\"zs-endnotes\"><li class=\"zs-endnote\" id=\"fn:1\" role=\"doc-endnote\" value=\"1\">Endnote <a class=\"zs-endnote-backref\" href=\"#fnref:1\" role=\"doc-backlink\">\u21a9\ufe0e</a></li></ol>",
			encoderMD:    "Text",
			encoderSz:    `(BLOCK (PARA (TEXT "Text") (ENDNOTE () (TEXT "Endnote"))))`,
			encoderSHTML: "((p \"Text\" (sup ((id . \"fnref:1\")) (a ((class . \"zs-noteref\") (href . \"#fn:1\") (role . \"doc-noteref\")) \"1\"))))",
			encoderText:  "Text Endnote",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Nested Endnotes",
		zmk:   `Text[^Endnote[^Nested]]`,
		expect: expectMap{
			encoderHTML:  "<p>Text<sup id=\"fnref:1\"><a class=\"zs-noteref\" href=\"#fn:1\" role=\"doc-noteref\">1</a></sup></p><ol class=\"zs-endnotes\"><li class=\"zs-endnote\" id=\"fn:1\" role=\"doc-endnote\" value=\"1\">Endnote<sup id=\"fnref:2\"><a class=\"zs-noteref\" href=\"#fn:2\" role=\"doc-noteref\">2</a></sup> <a class=\"zs-endnote-backref\" href=\"#fnref:1\" role=\"doc-backlink\">\u21a9\ufe0e</a></li><li class=\"zs-endnote\" id=\"fn:2\" role=\"doc-endnote\" value=\"2\">Nested <a class=\"zs-endnote-backref\" href=\"#fnref:2\" role=\"doc-backlink\">\u21a9\ufe0e</a></li></ol>",
			encoderMD:    "Text",
			encoderSz:    `(BLOCK (PARA (TEXT "Text") (ENDNOTE () (TEXT "Endnote") (ENDNOTE () (TEXT "Nested")))))`,
			encoderSHTML: "((p \"Text\" (sup ((id . \"fnref:1\")) (a ((class . \"zs-noteref\") (href . \"#fn:1\") (role . \"doc-noteref\")) \"1\"))))",
			encoderText:  "Text Endnote Nested",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "Transclusion",
		zmk:   `{{{http://example.com/image}}}{width="100px"}`,
		expect: expectMap{
			encoderHTML:  `<p><img class="external" src="http://example.com/image" width="100px"></p>`,
			encoderMD:    "",
			encoderSz:    `(BLOCK (TRANSCLUDE (("width" . "100px")) (EXTERNAL "http://example.com/image")))`,
			encoderSHTML: `((p (img ((class . "external") (src . "http://example.com/image") (width . "100px")))))`,
			encoderText:  "",
			encoderZmk:   useZmk,
		},
	},
	{
		descr: "A paragraph with an inline comment only should be empty in HTML",
		zmk:   `%% Comment`,
		expect: expectMap{
			// encoderHTML:  ``,
			encoderSz: `(BLOCK (PARA (LITERAL-COMMENT () "Comment")))`,
			// encoderSHTML: ``,
			encoderText: "",
			encoderZmk:  useZmk,
		},
	},
	{
		descr:  "Zettel with disallowed syntax HTML",
		zmk:    "<h1>Hello</h1>\nWorld\n",
		syntax: meta.ValueSyntaxHTML,
		expect: expectMap{
			encoderHTML:  "<pre><code class=\"language-html\">&lt;h1&gt;Hello&lt;/h1&gt;\nWorld</code></pre>",
			encoderSz:    `(BLOCK (VERBATIM-CODE (("" . "html")) "<h1>Hello</h1>\nWorld"))`,
			encoderSHTML: `((pre (code ((class . "language-html")) "<h1>Hello</h1>\nWorld")))`,
			encoderText:  "<h1>Hello</h1>\nWorld",
			encoderZmk:   "```{=\"html\"}\n<h1>Hello</h1>\nWorld\n```",
		},
	},
	{
		descr:     "Zettel with allowed syntax HTML",
		zmk:       "<h1>Hello</h1>\nWorld\n",
		syntax:    meta.ValueSyntaxHTML,
		allowHTML: true,
		expect: expectMap{
			encoderHTML:  "<h1>Hello</h1>\nWorld",
			encoderSz:    `(BLOCK (VERBATIM-HTML (("" . "html")) "<h1>Hello</h1>\nWorld"))`,
			encoderSHTML: `((@H "<h1>Hello</h1>\nWorld"))`,
			encoderText:  "<h1>Hello</h1>\nWorld",
			encoderZmk:   "@@@{=\"html\"}\n<h1>Hello</h1>\nWorld\n@@@",
		},
	},
	{
		descr: "",
		zmk:   ``,
		expect: expectMap{
			encoderHTML:  ``,
			encoderSz:    `(BLOCK)`,
			encoderSHTML: `()`,
			encoderText:  "",
			encoderZmk:   useZmk,
		},
	},
}
