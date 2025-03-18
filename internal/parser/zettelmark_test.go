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

package parser_test

import (
	"fmt"
	"strings"
	"testing"

	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/ast"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/parser"
)

type TestCase struct{ source, want string }
type TestCases []TestCase

func replace(s string, tcs TestCases) TestCases {
	var testCases TestCases

	for _, tc := range tcs {
		source := strings.ReplaceAll(tc.source, "$", s)
		want := strings.ReplaceAll(tc.want, "$", s)
		testCases = append(testCases, TestCase{source, want})
	}
	return testCases
}

func checkTcs(t *testing.T, tcs TestCases) {
	t.Helper()

	for tcn, tc := range tcs {
		t.Run(fmt.Sprintf("TC=%02d,src=%q", tcn, tc.source), func(st *testing.T) {
			st.Helper()
			inp := input.NewInput([]byte(tc.source))
			bns := parser.Parse(inp, nil, meta.ValueSyntaxZmk, config.NoHTML)
			var tv TestVisitor
			ast.Walk(&tv, &bns)
			got := tv.String()
			if tc.want != got {
				st.Errorf("\nwant=%q\n got=%q", tc.want, got)
			}
		})
	}
}

func TestEOL(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"", ""},
		{"\n", ""},
		{"\r", ""},
		{"\r\n", ""},
		{"\n\n", ""},
	})
}

func TestText(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"abcd", "(PARA abcd)"},
		{"ab cd", "(PARA ab cd)"},
		{"abcd ", "(PARA abcd)"},
		{" abcd", "(PARA abcd)"},
		{"\\", "(PARA \\)"},
		{"\\\n", ""},
		{"\\\ndef", "(PARA HB def)"},
		{"\\\r", ""},
		{"\\\rdef", "(PARA HB def)"},
		{"\\\r\n", ""},
		{"\\\r\ndef", "(PARA HB def)"},
		{"\\a", "(PARA a)"},
		{"\\aa", "(PARA aa)"},
		{"a\\a", "(PARA aa)"},
		{"\\+", "(PARA +)"},
		{"\\ ", "(PARA \u00a0)"},
		{"http://a, http://b", "(PARA http://a, http://b)"},
	})
}

func TestSpace(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{" ", ""},
		{"\t", ""},
		{"  ", ""},
	})
}

func TestSoftBreak(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"x\ny", "(PARA x SB y)"},
		{"z\n", "(PARA z)"},
		{" \n ", ""},
		{" \n", ""},
	})
}

func TestHardBreak(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"x  \ny", "(PARA x HB y)"},
		{"z  \n", "(PARA z)"},
		{"   \n ", ""},
		{"   \n", ""},
	})
}

func TestLink(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"[", "(PARA [)"},
		{"[[", "(PARA [[)"},
		{"[[|", "(PARA [[|)"},
		{"[[]", "(PARA [[])"},
		{"[[|]", "(PARA [[|])"},
		{"[[]]", "(PARA [[]])"},
		{"[[|]]", "(PARA [[|]])"},
		{"[[ ]]", "(PARA [[ ]])"},
		{"[[\n]]", "(PARA [[ SB ]])"},
		{"[[ a]]", "(PARA (LINK a))"},
		{"[[a ]]", "(PARA [[a ]])"},
		{"[[a\n]]", "(PARA [[a SB ]])"},
		{"[[a]]", "(PARA (LINK a))"},
		{"[[12345678901234]]", "(PARA (LINK 12345678901234))"},
		{"[[a]", "(PARA [[a])"},
		{"[[|a]]", "(PARA [[|a]])"},
		{"[[b|]]", "(PARA [[b|]])"},
		{"[[b|a]]", "(PARA (LINK a b))"},
		{"[[b| a]]", "(PARA (LINK a b))"},
		{"[[b%c|a]]", "(PARA (LINK a b%c))"},
		{"[[b%%c|a]]", "(PARA [[b {% c|a]]})"},
		{"[[b|a]", "(PARA [[b|a])"},
		{"[[b\nc|a]]", "(PARA (LINK a b SB c))"},
		{"[[b c|a#n]]", "(PARA (LINK a#n b c))"},
		{"[[a]]go", "(PARA (LINK a) go)"},
		{"[[b|a]]{go}", "(PARA (LINK a b)[ATTR go])"},
		{"[[[[a]]|b]]", "(PARA [[ (LINK a) |b]])"},
		{"[[a[b]c|d]]", "(PARA (LINK d a[b]c))"},
		{"[[[b]c|d]]", "(PARA [ (LINK d b]c))"},
		{"[[a[]c|d]]", "(PARA (LINK d a[]c))"},
		{"[[a[b]|d]]", "(PARA (LINK d a[b]))"},
		{"[[\\|]]", "(PARA (LINK %5C%7C))"},
		{"[[\\||a]]", "(PARA (LINK a |))"},
		{"[[b\\||a]]", "(PARA (LINK a b|))"},
		{"[[b\\|c|a]]", "(PARA (LINK a b|c))"},
		{"[[\\]]]", "(PARA (LINK %5C%5D))"},
		{"[[\\]|a]]", "(PARA (LINK a ]))"},
		{"[[b\\]|a]]", "(PARA (LINK a b]))"},
		{"[[\\]\\||a]]", "(PARA (LINK a ]|))"},
		{"[[http://a]]", "(PARA (LINK http://a))"},
		{"[[http://a|http://a]]", "(PARA (LINK http://a http://a))"},
		{"[[[[a]]]]", "(PARA [[ (LINK a) ]])"},
		{"[[query:title]]", "(PARA (LINK query:title))"},
		{"[[query:title syntax]]", "(PARA (LINK query:title syntax))"},
		{"[[query:title | action]]", "(PARA (LINK query:title | action))"},
		{"[[Text|query:title]]", "(PARA (LINK query:title Text))"},
		{"[[Text|query:title syntax]]", "(PARA (LINK query:title syntax Text))"},
		{"[[Text|query:title | action]]", "(PARA (LINK query:title | action Text))"},
	})
}

func TestCite(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"[@", "(PARA [@)"},
		{"[@]", "(PARA [@])"},
		{"[@a]", "(PARA (CITE a))"},
		{"[@ a]", "(PARA [@ a])"},
		{"[@a ]", "(PARA (CITE a))"},
		{"[@a\n]", "(PARA (CITE a))"},
		{"[@a\nx]", "(PARA (CITE a SB x))"},
		{"[@a\n\n]", "(PARA [@a)(PARA ])"},
		{"[@a,\n]", "(PARA (CITE a))"},
		{"[@a,n]", "(PARA (CITE a n))"},
		{"[@a| n]", "(PARA (CITE a n))"},
		{"[@a|n ]", "(PARA (CITE a n))"},
		{"[@a,[@b]]", "(PARA (CITE a (CITE b)))"},
		{"[@a]{color=green}", "(PARA (CITE a)[ATTR color=green])"},
	})
}

func TestFootnote(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"[^", "(PARA [^)"},
		{"[^]", "(PARA (FN))"},
		{"[^abc]", "(PARA (FN abc))"},
		{"[^abc ]", "(PARA (FN abc))"},
		{"[^abc\ndef]", "(PARA (FN abc SB def))"},
		{"[^abc\n\ndef]", "(PARA [^abc)(PARA def])"},
		{"[^abc[^def]]", "(PARA (FN abc (FN def)))"},
		{"[^abc]{-}", "(PARA (FN abc)[ATTR -])"},
	})
}

func TestEmbed(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"{", "(PARA {)"},
		{"{{", "(PARA {{)"},
		{"{{|", "(PARA {{|)"},
		{"{{}", "(PARA {{})"},
		{"{{|}", "(PARA {{|})"},
		{"{{}}", "(PARA {{}})"},
		{"{{|}}", "(PARA {{|}})"},
		{"{{ }}", "(PARA {{ }})"},
		{"{{\n}}", "(PARA {{ SB }})"},
		{"{{a }}", "(PARA {{a }})"},
		{"{{a\n}}", "(PARA {{a SB }})"},
		{"{{a}}", "(PARA (EMBED a))"},
		{"{{12345678901234}}", "(PARA (EMBED 12345678901234))"},
		{"{{ a}}", "(PARA (EMBED a))"},
		{"{{a}", "(PARA {{a})"},
		{"{{|a}}", "(PARA {{|a}})"},
		{"{{b|}}", "(PARA {{b|}})"},
		{"{{b|a}}", "(PARA (EMBED a b))"},
		{"{{b| a}}", "(PARA (EMBED a b))"},
		{"{{b|a}", "(PARA {{b|a})"},
		{"{{b\nc|a}}", "(PARA (EMBED a b SB c))"},
		{"{{b c|a#n}}", "(PARA (EMBED a#n b c))"},
		{"{{a}}{go}", "(PARA (EMBED a)[ATTR go])"},
		{"{{{{a}}|b}}", "(PARA {{ (EMBED a) |b}})"},
		{"{{\\|}}", "(PARA (EMBED %5C%7C))"},
		{"{{\\||a}}", "(PARA (EMBED a |))"},
		{"{{b\\||a}}", "(PARA (EMBED a b|))"},
		{"{{b\\|c|a}}", "(PARA (EMBED a b|c))"},
		{"{{\\}}}", "(PARA (EMBED %5C%7D))"},
		{"{{\\}|a}}", "(PARA (EMBED a }))"},
		{"{{b\\}|a}}", "(PARA (EMBED a b}))"},
		{"{{\\}\\||a}}", "(PARA (EMBED a }|))"},
		{"{{http://a}}", "(PARA (EMBED http://a))"},
		{"{{http://a|http://a}}", "(PARA (EMBED http://a http://a))"},
		{"{{{{a}}}}", "(PARA {{ (EMBED a) }})"},
	})
}

func TestMark(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"[!", "(PARA [!)"},
		{"[!\n", "(PARA [!)"},
		{"[!]", "(PARA (MARK #*))"},
		{"[!][!]", "(PARA (MARK #*) (MARK #*-1))"},
		{"[! ]", "(PARA [! ])"},
		{"[!a]", "(PARA (MARK \"a\" #a))"},
		{"[!a][!a]", "(PARA (MARK \"a\" #a) (MARK \"a\" #a-1))"},
		{"[!a ]", "(PARA [!a ])"},
		{"[!a_]", "(PARA (MARK \"a_\" #a))"},
		{"[!a_][!a]", "(PARA (MARK \"a_\" #a) (MARK \"a\" #a-1))"},
		{"[!a-b]", "(PARA (MARK \"a-b\" #a-b))"},
		{"[!a|b]", "(PARA (MARK \"a\" #a b))"},
		{"[!a|]", "(PARA (MARK \"a\" #a))"},
		{"[!|b]", "(PARA (MARK #* b))"},
		{"[!|b c]", "(PARA (MARK #* b c))"},
	})
}

func TestComment(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"%", "(PARA %)"},
		{"%%", "(PARA {%})"},
		{"%\n", "(PARA %)"},
		{"%%\n", "(PARA {%})"},
		{"%%a", "(PARA {% a})"},
		{"%%%a", "(PARA {% a})"},
		{"%% a", "(PARA {% a})"},
		{"%%%  a", "(PARA {% a})"},
		{"%% % a", "(PARA {% % a})"},
		{"%%a", "(PARA {% a})"},
		{"a%%b", "(PARA a {% b})"},
		{"a %%b", "(PARA a  {% b})" /*"(PARA a {% b})"*/},
		{" %%b", "(PARA {% b})"},
		{"%%b ", "(PARA {% b })"},
		{"100%", "(PARA 100%)"},
	})
}

func TestFormat(t *testing.T) {
	t.Parallel()
	// Not for Insert / '>', because collision with quoted list
	for _, ch := range []string{"_", "*", "~", "^", ",", "\"", "#", ":"} {
		checkTcs(t, replace(ch, TestCases{
			{"$", "(PARA $)"},
			{"$$", "(PARA $$)"},
			{"$$$", "(PARA $$$)"},
			{"$$$$", "(PARA {$})"},
		}))
	}
	for _, ch := range []string{"_", "*", ">", "~", "^", ",", "\"", "#", ":"} {
		checkTcs(t, replace(ch, TestCases{
			{"$$a$$", "(PARA {$ a})"},
			{"$$a$$$", "(PARA {$ a} $)"},
			{"$$$a$$", "(PARA {$ $a})"},
			{"$$$a$$$", "(PARA {$ $a} $)"},
			{"$\\$", "(PARA $$)"},
			{"$\\$$", "(PARA $$$)"},
			{"$$\\$", "(PARA $$$)"},
			{"$$a\\$$", "(PARA $$a$$)"},
			{"$$a$\\$", "(PARA $$a$$)"},
			{"$$a\\$$$", "(PARA {$ a$})"},
			{"$$a\na$$", "(PARA {$ a SB a})"},
			{"$$a\n\na$$", "(PARA $$a)(PARA a$$)"},
			{"$$a$${go}", "(PARA {$ a}[ATTR go])"},
		}))
	}
	checkTcs(t, TestCases{
		{"__****__", "(PARA {_ {*}})"},
		{"__**a**__", "(PARA {_ {* a}})"},
		{"__**__**", "(PARA __ {* __})"},
	})
}

func TestLiteral(t *testing.T) {
	t.Parallel()
	for _, ch := range []string{"`", "'", "="} {
		checkTcs(t, replace(ch, TestCases{
			{"$", "(PARA $)"},
			{"$$", "(PARA $$)"},
			{"$$$", "(PARA $$$)"},
			{"$$$$", "(PARA {$})"},
			{"$$a$$", "(PARA {$ a})"},
			{"$$a$$$", "(PARA {$ a} $)"},
			{"$$$a$$", "(PARA {$ $a})"},
			{"$$$a$$$", "(PARA {$ $a} $)"},
			{"$\\$", "(PARA $$)"},
			{"$\\$$", "(PARA $$$)"},
			{"$$\\$", "(PARA $$$)"},
			{"$$a\\$$", "(PARA $$a$$)"},
			{"$$a$\\$", "(PARA $$a$$)"},
			{"$$a\\$$$", "(PARA {$ a$})"},
			{"$$a$${go}", "(PARA {$ a}[ATTR go])"},
		}))
	}
	checkTcs(t, TestCases{
		{"``<script `` abc", "(PARA {` <script }  abc)"},
		{"''````''", "(PARA {' ````})"},
		{"''``a``''", "(PARA {' ``a``})"},
		{"''``''``", "(PARA {' ``} ``)"},
		{"''\\'''", "(PARA {' '})"},
	})
}

func TestLiteralMath(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"$", "(PARA $)"},
		{"$$", "(PARA $$)"},
		{"$$$", "(PARA $$$)"},
		{"$$$$", "(PARA {$})"},
		{"$$a$$", "(PARA {$ a})"},
		{"$$a$$$", "(PARA {$ a} $)"},
		{"$$$a$$", "(PARA {$ $a})"},
		{"$$$a$$$", "(PARA {$ $a} $)"},
		{`$\$`, "(PARA $$)"},
		{`$\$$`, "(PARA $$$)"},
		{`$$\$`, "(PARA $$$)"},
		{`$$a\$$`, `(PARA {$ a\})`},
		{`$$a$\$`, "(PARA $$a$$)"},
		{`$$a\$$$`, `(PARA {$ a\} $)`},
		{"$$a$${go}", "(PARA {$ a}[ATTR go])"},
	})
}

func TestMixFormatCode(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"__abc__\n**def**", "(PARA {_ abc} SB {* def})"},
		{"''abc''\n==def==", "(PARA {' abc} SB {= def})"},
		{"__abc__\n==def==", "(PARA {_ abc} SB {= def})"},
		{"__abc__\n``def``", "(PARA {_ abc} SB {` def})"},
		{"\"\"ghi\"\"\n::abc::\n``def``\n", "(PARA {\" ghi} SB {: abc} SB {` def})"},
	})
}

func TestNDash(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"--", "(PARA \u2013)"},
		{"a--b", "(PARA a\u2013b)"},
	})
}

func TestEntity(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"&", "(PARA &)"},
		{"&;", "(PARA &;)"},
		{"&#;", "(PARA &#;)"},
		{"&#1a;", "(PARA &#1a;)"},
		{"&#x;", "(PARA &#x;)"},
		{"&#x0z;", "(PARA &#x0z;)"},
		{"&1;", "(PARA &1;)"},
		{"&#9;", "(PARA &#9;)"}, // No numeric entities below space are not allowed.
		{"&#x1f;", "(PARA &#x1f;)"},

		// Good cases
		{"&lt;", "(PARA <)"},
		{"&#48;", "(PARA 0)"},
		{"&#x4A;", "(PARA J)"},
		{"&#X4a;", "(PARA J)"},
		{"&hellip;", "(PARA \u2026)"},
		{"&nbsp;", "(PARA \u00a0)"},
		{"E: &amp;,&#63;;&#x63;.", "(PARA E: &,?;c.)"},
	})
}

func TestVerbatimZettel(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		// {"@@@\n@@@", "(ZETTEL)"},
		{"@@@\nabc\n@@@", "(ZETTEL\nabc)"},
		{"@@@@def\nabc\n@@@@", "(ZETTEL\nabc)[ATTR =def]"},
	})
}

func TestVerbatimCode(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		// {"```\n```", "(PROG)"},
		{"```\nabc\n```", "(PROG\nabc)"},
		{"```\nabc\n````", "(PROG\nabc)"},
		{"````\nabc\n````", "(PROG\nabc)"},
		{"````\nabc\n```\n````", "(PROG\nabc\n```)"},
		{"````go\nabc\n````", "(PROG\nabc)[ATTR =go]"},
	})
}

func TestVerbatimEval(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		// {"~~~\n~~~", "(EVAL)"},
		{"~~~\nabc\n~~~", "(EVAL\nabc)"},
		{"~~~\nabc\n~~~~", "(EVAL\nabc)"},
		{"~~~~\nabc\n~~~~", "(EVAL\nabc)"},
		{"~~~~\nabc\n~~~\n~~~~", "(EVAL\nabc\n~~~)"},
		{"~~~~go\nabc\n~~~~", "(EVAL\nabc)[ATTR =go]"},
	})
}

func TestVerbatimMath(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		// {"$$$\n$$$", "(MATH)"},
		{"$$$\nabc\n$$$", "(MATH\nabc)"},
		{"$$$\nabc\n$$$$", "(MATH\nabc)"},
		{"$$$$\nabc\n$$$$", "(MATH\nabc)"},
		{"$$$$\nabc\n$$$\n$$$$", "(MATH\nabc\n$$$)"},
		{"$$$$go\nabc\n$$$$", "(MATH\nabc)[ATTR =go]"},
	})
}

func TestVerbatimComment(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		// {"%%%\n%%%", "(COMMENT)"},
		{"%%%\nabc\n%%%", "(COMMENT\nabc)"},
		{"%%%%go\nabc\n%%%%", "(COMMENT\nabc)[ATTR =go]"},
	})
}

func TestPara(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"a\n\nb", "(PARA a)(PARA b)"},
		{"a\n \nb", "(PARA a)(PARA b)"},
	})
}

func TestSpanRegion(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		// {":::\n:::", "(SPAN)"},
		{":::\nabc\n:::", "(SPAN (PARA abc))"},
		{":::\nabc\n::::", "(SPAN (PARA abc))"},
		{"::::\nabc\n::::", "(SPAN (PARA abc))"},
		{"::::\nabc\n:::\ndef\n:::\n::::", "(SPAN (PARA abc)(SPAN (PARA def)))"},
		{":::{go}\na\n:::", "(SPAN (PARA a))[ATTR go]"},
		{":::\nabc\n::: def ", "(SPAN (PARA abc) (LINE def))"},
	})
}

func TestQuoteRegion(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		// {"<<<\n<<<", "(QUOTE)"},
		{"<<<\nabc\n<<<", "(QUOTE (PARA abc))"},
		{"<<<\nabc\n<<<<", "(QUOTE (PARA abc))"},
		{"<<<<\nabc\n<<<<", "(QUOTE (PARA abc))"},
		{"<<<<\nabc\n<<<\ndef\n<<<\n<<<<", "(QUOTE (PARA abc)(QUOTE (PARA def)))"},
		{"<<<go\na\n<<<", "(QUOTE (PARA a))[ATTR =go]"},
		{"<<<\nabc\n<<< def ", "(QUOTE (PARA abc) (LINE def))"},
	})
}

func TestVerseRegion(t *testing.T) {
	t.Parallel()
	checkTcs(t, replace("\"", TestCases{
		// {"$$$\n$$$", "(VERSE)"},
		{"$$$\nabc\n$$$", "(VERSE (PARA abc))"},
		{"$$$\nabc\n$$$$", "(VERSE (PARA abc))"},
		{"$$$$\nabc\n$$$$", "(VERSE (PARA abc))"},
		{"$$$\nabc\ndef\n$$$", "(VERSE (PARA abc HB def))"},
		{"$$$$\nabc\n$$$\ndef\n$$$\n$$$$", "(VERSE (PARA abc)(VERSE (PARA def)))"},
		{"$$$go\na\n$$$", "(VERSE (PARA a))[ATTR =go]"},
		{"$$$\nabc\n$$$ def ", "(VERSE (PARA abc) (LINE def))"},
	}))
}

func TestHeading(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"=h", "(PARA =h)"},
		{"= h", "(PARA = h)"},
		{"==h", "(PARA ==h)"},
		{"== h", "(PARA == h)"},
		{"===h", "(PARA ===h)"},
		{"=== h", "(H1 h #h)"},
		{"===  h", "(H1 h #h)"},
		{"==== h", "(H2 h #h)"},
		{"===== h", "(H3 h #h)"},
		{"====== h", "(H4 h #h)"},
		{"======= h", "(H5 h #h)"},
		{"======== h", "(H5 h #h)"},
		{"=", "(PARA =)"},
		{"=== h=__=a__", "(H1 h= {_ =a} #h-a)"},
		{"=\n", "(PARA =)"},
		{"a=", "(PARA a=)"},
		{" =", "(PARA =)"},
		{"=== h\na", "(H1 h #h)(PARA a)"},
		{"=== h i {-}", "(H1 h i #h-i)[ATTR -]"},
		{"=== h {{a}}", "(H1 h  (EMBED a) #h)"},
		{"=== h{{a}}", "(H1 h (EMBED a) #h)"},
		{"=== {{a}}", "(H1 (EMBED a))"},
		{"=== h {{a}}{-}", "(H1 h  (EMBED a)[ATTR -] #h)"},
		{"=== h {{a}} {-}", "(H1 h  (EMBED a) #h)[ATTR -]"},
		{"=== h {-}{{a}}", "(H1 h #h)[ATTR -]"},
		{"=== h{id=abc}", "(H1 h #h)[ATTR id=abc]"},
		{"=== h\n=== h", "(H1 h #h)(H1 h #h-1)"},
	})
}

func TestHRule(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"-", "(PARA -)"},
		{"---", "(HR)"},
		{"----", "(HR)"},
		{"---A", "(HR)[ATTR =A]"},
		{"---A-", "(HR)[ATTR =A-]"},
		{"-1", "(PARA -1)"},
		{"2-1", "(PARA 2-1)"},
		{"---  {  go  }  ", "(HR)[ATTR go]"},
		{"---  {  .go  }  ", "(HR)[ATTR class=go]"},
	})
}

func TestList(t *testing.T) {
	t.Parallel()
	// No ">" in the following, because quotation lists may have empty items.
	for _, ch := range []string{"*", "#"} {
		checkTcs(t, replace(ch, TestCases{
			{"$", "(PARA $)"},
			{"$$", "(PARA $$)"},
			{"$$$", "(PARA $$$)"},
			{"$ ", "(PARA $)"},
			{"$$ ", "(PARA $$)"},
			{"$$$ ", "(PARA $$$)"},
		}))
	}
	checkTcs(t, TestCases{
		{"* abc", "(UL {(PARA abc)})"},
		{"** abc", "(UL {(UL {(PARA abc)})})"},
		{"*** abc", "(UL {(UL {(UL {(PARA abc)})})})"},
		{"**** abc", "(UL {(UL {(UL {(UL {(PARA abc)})})})})"},
		{"** abc\n**** def", "(UL {(UL {(PARA abc)(UL {(UL {(PARA def)})})})})"},
		{"* abc\ndef", "(UL {(PARA abc)})(PARA def)"},
		{"* abc\n def", "(UL {(PARA abc)})(PARA def)"},
		{"* abc\n* def", "(UL {(PARA abc)} {(PARA def)})"},
		{"* abc\n  def", "(UL {(PARA abc SB def)})"},
		{"* abc\n   def", "(UL {(PARA abc SB def)})"},
		{"* abc\n\ndef", "(UL {(PARA abc)})(PARA def)"},
		{"* abc\n\n def", "(UL {(PARA abc)})(PARA def)"},
		{"* abc\n\n  def", "(UL {(PARA abc)(PARA def)})"},
		{"* abc\n\n   def", "(UL {(PARA abc)(PARA def)})"},
		{"* abc\n** def", "(UL {(PARA abc)(UL {(PARA def)})})"},
		{"* abc\n** def\n* ghi", "(UL {(PARA abc)(UL {(PARA def)})} {(PARA ghi)})"},
		{"* abc\n\n  def\n* ghi", "(UL {(PARA abc)(PARA def)} {(PARA ghi)})"},
		{"* abc\n** def\n   ghi\n  jkl", "(UL {(PARA abc)(UL {(PARA def SB ghi)})(PARA jkl)})"},

		// A list does not last beyond a region
		{":::\n# abc\n:::\n# def", "(SPAN (OL {(PARA abc)}))(OL {(PARA def)})"},

		// A HRule creates a new list
		{"* abc\n---\n* def", "(UL {(PARA abc)})(HR)(UL {(PARA def)})"},

		// Changing list type adds a new list
		{"* abc\n# def", "(UL {(PARA abc)})(OL {(PARA def)})"},

		// Quotation lists may have empty items
		{">", "(QL {})"},
	})
}

func TestQuoteList(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"> w1 w2", "(QL {(PARA w1 w2)})"},
		{"> w1\n> w2", "(QL {(PARA w1 SB w2)})"},
		{"> w1\n>\n>w2", "(QL {(PARA w1)} {})(PARA >w2)"},
	})
}

func TestEnumAfterPara(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"abc\n* def", "(PARA abc)(UL {(PARA def)})"},
		{"abc\n*def", "(PARA abc SB *def)"},
	})
}

func TestDefinition(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{";", "(PARA ;)"},
		{"; ", "(PARA ;)"},
		{"; abc", "(DL (DT abc))"},
		{"; abc\ndef", "(DL (DT abc))(PARA def)"},
		{"; abc\n def", "(DL (DT abc))(PARA def)"},
		{"; abc\n  def", "(DL (DT abc SB def))"},
		{":", "(PARA :)"},
		{": ", "(PARA :)"},
		{": abc", "(PARA : abc)"},
		{"; abc\n: def", "(DL (DT abc) (DD (PARA def)))"},
		{"; abc\n: def\nghi", "(DL (DT abc) (DD (PARA def)))(PARA ghi)"},
		{"; abc\n: def\n ghi", "(DL (DT abc) (DD (PARA def)))(PARA ghi)"},
		{"; abc\n: def\n  ghi", "(DL (DT abc) (DD (PARA def SB ghi)))"},
		{"; abc\n: def\n\n  ghi", "(DL (DT abc) (DD (PARA def)(PARA ghi)))"},
		{"; abc\n:", "(DL (DT abc))(PARA :)"},
		{"; abc\n: def\n: ghi", "(DL (DT abc) (DD (PARA def)) (DD (PARA ghi)))"},
		{"; abc\n: def\n; ghi\n: jkl", "(DL (DT abc) (DD (PARA def)) (DT ghi) (DD (PARA jkl)))"},
	})
}

func TestTable(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		// {"|", "(TAB (TR))"},
		{"||", "(TAB (TR (TD)))"},
		{"| |", "(TAB (TR (TD)))"},
		{"|a", "(TAB (TR (TD a)))"},
		{"|a|", "(TAB (TR (TD a)))"},
		{"|a| ", "(TAB (TR (TD a)(TD)))"},
		{"|a|b", "(TAB (TR (TD a)(TD b)))"},
		{"|a|b\n|c|d", "(TAB (TR (TD a)(TD b))(TR (TD c)(TD d)))"},
		{"|%", ""},
		{"|a|b\n|%---\n|c|d", "(TAB (TR (TD a)(TD b))(TR (TD c)(TD d)))"},
		{"|a|b\n|c", "(TAB (TR (TD a)(TD b))(TR (TD c)(TD)))"},
	})
}

func TestTransclude(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"{{{a}}}", "(TRANSCLUDE a)"},
		{"{{{a}}}b", "(TRANSCLUDE a)[ATTR =b]"},
		{"{{{a}}}}", "(TRANSCLUDE a)"},
		{"{{{a\\}}}}", "(TRANSCLUDE a%5C%7D)"},
		{"{{{a\\}}}}b", "(TRANSCLUDE a%5C%7D)[ATTR =b]"},
		{"{{{a}}", "(PARA { (EMBED a))"},
		{"{{{a}}}{go=b}", "(TRANSCLUDE a)[ATTR go=b]"},
	})
}

func TestBlockAttr(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{":::go\na\n:::", "(SPAN (PARA a))[ATTR =go]"},
		{":::go=\na\n:::", "(SPAN (PARA a))[ATTR =go]"},
		{":::{}\na\n:::", "(SPAN (PARA a))"},
		{":::{ }\na\n:::", "(SPAN (PARA a))"},
		{":::{.go}\na\n:::", "(SPAN (PARA a))[ATTR class=go]"},
		{":::{=go}\na\n:::", "(SPAN (PARA a))[ATTR =go]"},
		{":::{go}\na\n:::", "(SPAN (PARA a))[ATTR go]"},
		{":::{go=py}\na\n:::", "(SPAN (PARA a))[ATTR go=py]"},
		{":::{.go=py}\na\n:::", "(SPAN (PARA a))"},
		{":::{go=}\na\n:::", "(SPAN (PARA a))[ATTR go]"},
		{":::{.go=}\na\n:::", "(SPAN (PARA a))"},
		{":::{go py}\na\n:::", "(SPAN (PARA a))[ATTR go py]"},
		{":::{go\npy}\na\n:::", "(SPAN (PARA a))[ATTR go py]"},
		{":::{.go py}\na\n:::", "(SPAN (PARA a))[ATTR class=go py]"},
		{":::{go .py}\na\n:::", "(SPAN (PARA a))[ATTR class=py go]"},
		{":::{.go py=3}\na\n:::", "(SPAN (PARA a))[ATTR class=go py=3]"},
		{":::  {  go  }  \na\n:::", "(SPAN (PARA a))[ATTR go]"},
		{":::  {  .go  }  \na\n:::", "(SPAN (PARA a))[ATTR class=go]"},
	})
	checkTcs(t, replace("\"", TestCases{
		{":::{py=3}\na\n:::", "(SPAN (PARA a))[ATTR py=3]"},
		{":::{py=$2 3$}\na\n:::", "(SPAN (PARA a))[ATTR py=$2 3$]"},
		{":::{py=$2\\$3$}\na\n:::", "(SPAN (PARA a))[ATTR py=2$3]"},
		{":::{py=2$3}\na\n:::", "(SPAN (PARA a))[ATTR py=2$3]"},
		{":::{py=$2\n3$}\na\n:::", "(SPAN (PARA a))[ATTR py=$2\n3$]"},
		{":::{py=$2 3}\na\n:::", "(SPAN (PARA a))"},
		{":::{py=2 py=3}\na\n:::", "(SPAN (PARA a))[ATTR py=$2 3$]"},
		{":::{.go .py}\na\n:::", "(SPAN (PARA a))[ATTR class=$go py$]"},
		{":::{go go}\na\n:::", "(SPAN (PARA a))[ATTR go]"},
		{":::{=py =go}\na\n:::", "(SPAN (PARA a))[ATTR =go]"},
	}))
}

func TestInlineAttr(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"::a::{}", "(PARA {: a})"},
		{"::a::{ }", "(PARA {: a})"},
		{"::a::{.go}", "(PARA {: a}[ATTR class=go])"},
		{"::a::{=go}", "(PARA {: a}[ATTR =go])"},
		{"::a::{go}", "(PARA {: a}[ATTR go])"},
		{"::a::{go=py}", "(PARA {: a}[ATTR go=py])"},
		{"::a::{.go=py}", "(PARA {: a} {.go=py})"},
		{"::a::{go=}", "(PARA {: a}[ATTR go])"},
		{"::a::{.go=}", "(PARA {: a} {.go=})"},
		{"::a::{go py}", "(PARA {: a}[ATTR go py])"},
		{"::a::{go\npy}", "(PARA {: a}[ATTR go py])"},
		{"::a::{.go py}", "(PARA {: a}[ATTR class=go py])"},
		{"::a::{go .py}", "(PARA {: a}[ATTR class=py go])"},
		{"::a::{  \n go \n .py\n  \n}", "(PARA {: a}[ATTR class=py go])"},
		{"::a::{  \n go \n .py\n\n}", "(PARA {: a}[ATTR class=py go])"},
		{"::a::{\ngo\n}", "(PARA {: a}[ATTR go])"},
	})
	checkTcs(t, replace("\"", TestCases{
		{"::a::{py=3}", "(PARA {: a}[ATTR py=3])"},
		{"::a::{py=$2 3$}", "(PARA {: a}[ATTR py=$2 3$])"},
		{"::a::{py=$2\\$3$}", "(PARA {: a}[ATTR py=2$3])"},
		{"::a::{py=2$3}", "(PARA {: a}[ATTR py=2$3])"},
		{"::a::{py=$2\n3$}", "(PARA {: a}[ATTR py=$2\n3$])"},
		{"::a::{py=$2 3}", "(PARA {: a} {py=$2 3})"},

		{"::a::{py=2 py=3}", "(PARA {: a}[ATTR py=$2 3$])"},
		{"::a::{.go .py}", "(PARA {: a}[ATTR class=$go py$])"},
	}))
}

func TestTemp(t *testing.T) {
	t.Parallel()
	checkTcs(t, TestCases{
		{"", ""},
	})
}

// --------------------------------------------------------------------------

// TestVisitor serializes the abstract syntax tree to a string.
type TestVisitor struct {
	sb strings.Builder
}

func (tv *TestVisitor) String() string { return tv.sb.String() }

func (tv *TestVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.InlineSlice:
		tv.visitInlineSlice(n)
	case *ast.ParaNode:
		tv.sb.WriteString("(PARA")
		ast.Walk(tv, &n.Inlines)
		tv.sb.WriteByte(')')
	case *ast.VerbatimNode:
		code, ok := mapVerbatimKind[n.Kind]
		if !ok {
			panic(fmt.Sprintf("Unknown verbatim code %v", n.Kind))
		}
		tv.sb.WriteString(code)
		if len(n.Content) > 0 {
			tv.sb.WriteByte('\n')
			tv.sb.Write(n.Content)
		}
		tv.sb.WriteByte(')')
		tv.visitAttributes(n.Attrs)
	case *ast.RegionNode:
		code, ok := mapRegionKind[n.Kind]
		if !ok {
			panic(fmt.Sprintf("Unknown region code %v", n.Kind))
		}
		tv.sb.WriteString(code)
		if len(n.Blocks) > 0 {
			tv.sb.WriteByte(' ')
			ast.Walk(tv, &n.Blocks)
		}
		if len(n.Inlines) > 0 {
			tv.sb.WriteString(" (LINE")
			ast.Walk(tv, &n.Inlines)
			tv.sb.WriteByte(')')
		}
		tv.sb.WriteByte(')')
		tv.visitAttributes(n.Attrs)
	case *ast.HeadingNode:
		fmt.Fprintf(&tv.sb, "(H%d", n.Level)
		ast.Walk(tv, &n.Inlines)
		if n.Fragment != "" {
			tv.sb.WriteString(" #")
			tv.sb.WriteString(n.Fragment)
		}
		tv.sb.WriteByte(')')
		tv.visitAttributes(n.Attrs)
	case *ast.HRuleNode:
		tv.sb.WriteString("(HR)")
		tv.visitAttributes(n.Attrs)
	case *ast.NestedListNode:
		tv.sb.WriteString(mapNestedListKind[n.Kind])
		for _, item := range n.Items {
			tv.sb.WriteString(" {")
			ast.WalkItemSlice(tv, item)
			tv.sb.WriteByte('}')
		}
		tv.sb.WriteByte(')')
	case *ast.DescriptionListNode:
		tv.sb.WriteString("(DL")
		for _, def := range n.Descriptions {
			tv.sb.WriteString(" (DT")
			ast.Walk(tv, &def.Term)
			tv.sb.WriteByte(')')
			for _, b := range def.Descriptions {
				tv.sb.WriteString(" (DD ")
				ast.WalkDescriptionSlice(tv, b)
				tv.sb.WriteByte(')')
			}
		}
		tv.sb.WriteByte(')')
	case *ast.TableNode:
		tv.sb.WriteString("(TAB")
		if len(n.Header) > 0 {
			tv.sb.WriteString(" (TR")
			for _, cell := range n.Header {
				tv.sb.WriteString(" (TH")
				tv.sb.WriteString(alignString[cell.Align])
				ast.Walk(tv, &cell.Inlines)
				tv.sb.WriteString(")")
			}
			tv.sb.WriteString(")")
		}
		if len(n.Rows) > 0 {
			tv.sb.WriteString(" ")
			for _, row := range n.Rows {
				tv.sb.WriteString("(TR")
				for i, cell := range row {
					if i == 0 {
						tv.sb.WriteString(" ")
					}
					tv.sb.WriteString("(TD")
					tv.sb.WriteString(alignString[cell.Align])
					ast.Walk(tv, &cell.Inlines)
					tv.sb.WriteString(")")
				}
				tv.sb.WriteString(")")
			}
		}
		tv.sb.WriteString(")")
	case *ast.TranscludeNode:
		fmt.Fprintf(&tv.sb, "(TRANSCLUDE %v)", n.Ref) // FIXME n.Inlines
		tv.visitAttributes(n.Attrs)
	case *ast.BLOBNode:
		tv.sb.WriteString("(BLOB ")
		tv.sb.WriteString(n.Syntax)
		tv.sb.WriteString(")")
	case *ast.TextNode:
		tv.sb.WriteString(n.Text)
	case *ast.BreakNode:
		if n.Hard {
			tv.sb.WriteString("HB")
		} else {
			tv.sb.WriteString("SB")
		}
	case *ast.LinkNode:
		fmt.Fprintf(&tv.sb, "(LINK %v", n.Ref)
		ast.Walk(tv, &n.Inlines)
		tv.sb.WriteByte(')')
		tv.visitAttributes(n.Attrs)
	case *ast.EmbedRefNode:
		fmt.Fprintf(&tv.sb, "(EMBED %v", n.Ref)
		if len(n.Inlines) > 0 {
			ast.Walk(tv, &n.Inlines)
		}
		tv.sb.WriteByte(')')
		tv.visitAttributes(n.Attrs)
	case *ast.EmbedBLOBNode:
		panic("TODO: zmktest blob")
	case *ast.CiteNode:
		fmt.Fprintf(&tv.sb, "(CITE %s", n.Key)
		if len(n.Inlines) > 0 {
			ast.Walk(tv, &n.Inlines)
		}
		tv.sb.WriteByte(')')
		tv.visitAttributes(n.Attrs)
	case *ast.FootnoteNode:
		tv.sb.WriteString("(FN")
		ast.Walk(tv, &n.Inlines)
		tv.sb.WriteByte(')')
		tv.visitAttributes(n.Attrs)
	case *ast.MarkNode:
		tv.sb.WriteString("(MARK")
		if n.Mark != "" {
			tv.sb.WriteString(" \"")
			tv.sb.WriteString(n.Mark)
			tv.sb.WriteByte('"')
		}
		if n.Fragment != "" {
			tv.sb.WriteString(" #")
			tv.sb.WriteString(n.Fragment)
		}
		if len(n.Inlines) > 0 {
			ast.Walk(tv, &n.Inlines)
		}
		tv.sb.WriteByte(')')
	case *ast.FormatNode:
		fmt.Fprintf(&tv.sb, "{%c", mapFormatKind[n.Kind])
		ast.Walk(tv, &n.Inlines)
		tv.sb.WriteByte('}')
		tv.visitAttributes(n.Attrs)
	case *ast.LiteralNode:
		code, ok := mapLiteralKind[n.Kind]
		if !ok {
			panic(fmt.Sprintf("No element for code %v", n.Kind))
		}
		tv.sb.WriteByte('{')
		tv.sb.WriteRune(code)
		if len(n.Content) > 0 {
			tv.sb.WriteByte(' ')
			tv.sb.Write(n.Content)
		}
		tv.sb.WriteByte('}')
		tv.visitAttributes(n.Attrs)
	default:
		return tv
	}
	return nil
}

var mapVerbatimKind = map[ast.VerbatimKind]string{
	ast.VerbatimZettel:  "(ZETTEL",
	ast.VerbatimCode:    "(PROG",
	ast.VerbatimEval:    "(EVAL",
	ast.VerbatimMath:    "(MATH",
	ast.VerbatimComment: "(COMMENT",
}

var mapRegionKind = map[ast.RegionKind]string{
	ast.RegionSpan:  "(SPAN",
	ast.RegionQuote: "(QUOTE",
	ast.RegionVerse: "(VERSE",
}

var mapNestedListKind = map[ast.NestedListKind]string{
	ast.NestedListOrdered:   "(OL",
	ast.NestedListUnordered: "(UL",
	ast.NestedListQuote:     "(QL",
}

var alignString = map[ast.Alignment]string{
	ast.AlignDefault: "",
	ast.AlignLeft:    "l",
	ast.AlignCenter:  "c",
	ast.AlignRight:   "r",
}

var mapFormatKind = map[ast.FormatKind]rune{
	ast.FormatEmph:   '_',
	ast.FormatStrong: '*',
	ast.FormatInsert: '>',
	ast.FormatDelete: '~',
	ast.FormatSuper:  '^',
	ast.FormatSub:    ',',
	ast.FormatQuote:  '"',
	ast.FormatMark:   '#',
	ast.FormatSpan:   ':',
}

var mapLiteralKind = map[ast.LiteralKind]rune{
	ast.LiteralCode:    '`',
	ast.LiteralInput:   '\'',
	ast.LiteralOutput:  '=',
	ast.LiteralComment: '%',
	ast.LiteralMath:    '$',
}

func (tv *TestVisitor) visitInlineSlice(is *ast.InlineSlice) {
	for _, in := range *is {
		tv.sb.WriteByte(' ')
		ast.Walk(tv, in)
	}
}

func (tv *TestVisitor) visitAttributes(a zsx.Attributes) {
	if a.IsEmpty() {
		return
	}
	tv.sb.WriteString("[ATTR")

	for _, k := range a.Keys() {
		tv.sb.WriteByte(' ')
		tv.sb.WriteString(k)
		v := a[k]
		if len(v) > 0 {
			tv.sb.WriteByte('=')
			if quoteString(v) {
				tv.sb.WriteByte('"')
				tv.sb.WriteString(v)
				tv.sb.WriteByte('"')
			} else {
				tv.sb.WriteString(v)
			}
		}
	}

	tv.sb.WriteByte(']')
}

func quoteString(s string) bool {
	for _, ch := range s {
		if ch <= ' ' {
			return true
		}
	}
	return false
}
