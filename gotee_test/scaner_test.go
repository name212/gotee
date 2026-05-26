// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/name212/gotee"
	"github.com/name212/gotee/scan"
)

const smallMaxTokenSize = 256 // Much smaller for more efficient testing.

func TestScanByte(t *testing.T) {
	newBytesTest := func(t *testing.T, tst scanTest) *scanTest {
		tst.split = bufio.ScanBytes
		return newScanTest(t, tst)
	}

	assertBytes := func(t *testing.T, tst *scanTest) {
		assertScannerNoError(t, tst)

		fullLen := 0
		for _, s := range tst.scans {
			fullLen += len(s)
		}
		require.Len(t, tst.handler.handled, fullLen, "should have correct handles")

		i := 0
		for _, s := range tst.scans {
			for j := 0; j < len(s); j++ {
				gotBytes := tst.handler.handled[i]
				require.Len(t, gotBytes, 1, "%d: should handle one byte: %s", i, gotBytes)
				require.Equal(t, s[j], gotBytes[0], "%d: should handle correct byte", i)
				i++
			}
		}

		require.Len(t, tst.scanner.Unhandled(), 0, "should not have unhandled bytes")
	}

	tests := []*scanTest{
		newBytesTest(t, scanTest{
			name:   "empty string",
			scans:  [][]byte{[]byte("")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "empty string multiple",
			scans:  [][]byte{[]byte(""), []byte("")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "ascii one",
			scans:  [][]byte{[]byte("a")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "ascii one multiple",
			scans:  [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "ascii multiple",
			scans:  [][]byte{[]byte("abcdefgh")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "ascii hight",
			scans:  [][]byte{[]byte("¼")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "ascii with spaces",
			scans:  [][]byte{[]byte("abc def\n\t\tgh    ")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "utf incorrect",
			scans:  [][]byte{[]byte("\x81")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "utf incorrect 2",
			scans:  [][]byte{[]byte("\uFFFD")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name:   "utf with ascii",
			scans:  [][]byte{[]byte("abc¼☹\x81\uFFFD日本語\x82abc")},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name: "utf with ascii multiple write 2",
			scans: [][]byte{
				[]byte("abc¼☹\x81\uFFFD日本語\x82abc"),
				[]byte("abc¼☹\x81\uFFFD日本語\x82abc"),
			},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name: "utf with ascii multiple write 3",
			scans: [][]byte{
				[]byte("abc¼☹\x81\uFFFD日本語\x82abc"),
				[]byte("abc¼☹日本語\x82abc"),
				[]byte("日本語\x82abc"),
			},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name: "utf with ascii multiple write with empty",
			scans: [][]byte{
				[]byte("abc¼☹\x81\uFFFD日本語\x82abc"),
				[]byte(""),
				[]byte("abc¼☹日本語\x82abc"),
				[]byte("日本語\x82abc"),
				[]byte(""),
			},
			assert: assertBytes,
		}),

		newBytesTest(t, scanTest{
			name: "utf with ascii multiple write with one and empty",
			scans: [][]byte{
				[]byte("abc¼☹\x81\uFFFD日本語\x82abc"),
				[]byte("a"),
				[]byte("abc¼☹日本語\x82abc"),
				[]byte(""),
				[]byte("日本語\x82abc"),
				[]byte("b"),
			},
			assert: assertBytes,
		}),
	}

	doAllScannerTests(t, tests)
}

func TestScanRune(t *testing.T) {
	newRunesTest := func(t *testing.T, tst scanTest) *scanTest {
		tst.split = bufio.ScanRunes
		return newScanTest(t, tst)
	}

	assertCorrectRunesWithInCorrect := func(incorrectIndexes []int) assertScanner {
		return func(t *testing.T, tst *scanTest) {
			assertScannerNoError(t, tst)

			fullLen := 0
			for _, s := range tst.scans {
				fullLen += utf8.RuneCount(s)
			}
			require.Len(t, tst.handler.handled, fullLen, "should have correct handles")

			i := 0
			for _, s := range tst.scans {
				ss := string(s)
				for _, r := range ss {
					gotBytes := tst.handler.handled[i]
					gotRune, size := utf8.DecodeRune(gotBytes)
					require.Equal(t, size, utf8.RuneLen(rune(r)), "%d: should handle correct rune size: %d", i, size)
					if slices.Contains(incorrectIndexes, i) {
						require.Equal(t, utf8.RuneError, gotRune, "%d: should handle incorrect rune", i)
					} else {
						require.Equal(t, r, gotRune, "%d: should handle correct rune", i)
					}
					i++
				}
			}
		}
	}

	assertCorrectRunes := func(t *testing.T, tst *scanTest) {
		assertCorrectRunesWithInCorrect([]int{})
	}

	assertUtf8IncorrectRunes := func(t *testing.T, tst *scanTest) {
		assertScannerNoError(t, tst)

		require.Len(t, tst.handler.handled, 1, "should have correct handles")
		require.Len(t, tst.scanner.Unhandled(), 0, "should not have unhandled runes")
		r, _ := utf8.DecodeRune([]byte(tst.handler.handled[0]))
		require.Equal(t, utf8.RuneError, r, "should have Rune error at handle %d", 0)
	}

	tests := []*scanTest{
		newRunesTest(t, scanTest{
			name:   "empty string",
			scans:  [][]byte{[]byte("")},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "empty string multiple",
			scans:  [][]byte{[]byte(""), []byte("")},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "ascii one",
			scans:  [][]byte{[]byte("a")},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "ascii one multiple",
			scans:  [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "ascii multiple",
			scans:  [][]byte{[]byte("abcdefgh")},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "ascii hight",
			scans:  [][]byte{[]byte("¼")},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "ascii with spaces",
			scans:  [][]byte{[]byte("abc def\n\t\tgh    ")},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "utf incorrect",
			scans:  [][]byte{[]byte("\x81")},
			assert: assertUtf8IncorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "utf incorrect 2",
			scans:  [][]byte{[]byte("\uFFFD")},
			assert: assertUtf8IncorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name:   "utf with ascii all correct",
			scans:  [][]byte{[]byte("abc¼☹日本語abc")},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name: "utf with ascii multiple write 2",
			scans: [][]byte{
				[]byte("abc¼☹日本語abc"),
				[]byte("¼☹日本語def"),
			},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name: "utf with ascii multiple write 3",
			scans: [][]byte{
				[]byte("abc¼☹日本語abc"),
				[]byte("abc¼☹日本abc"),
				[]byte("日本語abc"),
			},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name: "utf with ascii incorrect multiple write with empty",
			scans: [][]byte{
				[]byte("abc¼☹日本語abc"),
				[]byte(""),
				[]byte("fh2abc¼☹日本語\x82abc"),
				[]byte("日本語abc"),
				[]byte(""),
			},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name: "utf with ascii multiple write with one and empty",
			scans: [][]byte{
				[]byte("abc¼☹\x81\uFFFD日本語\x82abc"),
				[]byte(""),
				[]byte("abc¼☹日本語\x82abc"),
				[]byte("語"),
				[]byte("日本語\x82abc"),
				[]byte("b"),
			},
			assert: assertCorrectRunes,
		}),

		newRunesTest(t, scanTest{
			name: "utf with ascii with incorrect multiple write",
			scans: [][]byte{
				[]byte("abc¼☹\x81\uFFFD日本語\x82abc"),
				[]byte("abc¼☹\x81日本語\x82abc"),
			},
			assert: assertCorrectRunesWithInCorrect([]int{
				5, 6, 10, 19, 23,
			}),
		}),

		newRunesTest(t, scanTest{
			name: "utf with ascii incorrect multiple write with one and empty",
			scans: [][]byte{
				[]byte("abc¼☹\x81\uFFFD日本語\x82abc"),
				[]byte(""),
				[]byte("abc¼☹日本語\x82abc"),
				[]byte("\uFFFD"),
				[]byte("日本語\x82abc"),
				[]byte("本"),
				[]byte("\x82"),
			},
			assert: assertCorrectRunesWithInCorrect([]int{
				5, 6, 10, 22, 26, 30, 36,
			}),
		}),
	}

	doAllScannerTests(t, tests)
}

// Test that the word splitter returns the same data as strings.Fields.
func TestScanWords(t *testing.T) {
	newWordsTest := func(t *testing.T, tst scanTest) *scanTest {
		tst.split = bufio.ScanWords
		return newScanTest(t, tst)
	}

	newWordsExcessiveWhiteSpaceTest := func(t *testing.T, tst scanTest) *scanTest {
		tst.split = bufio.ScanWords
		const word = "ipsum"
		s := strings.Repeat(" ", 4*smallMaxTokenSize) + word
		tst.scans = [][]byte{
			[]byte(s),
		}
		res := newScanTest(t, tst)
		res.scanner.PrivateMaxTokenSize(smallMaxTokenSize)
		return res
	}

	assertWords := func(unhandled int) assertScanner {
		return func(t *testing.T, tst *scanTest) {
			joinedScans := bytes.Join(tst.scans, []byte(""))
			words := strings.Fields(string(joinedScans))

			assertHandledStrings(unhandled, words)(t, tst)
		}
	}

	tests := []*scanTest{
		newWordsTest(t, scanTest{
			name:   "empty string",
			scans:  [][]byte{[]byte("")},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name:   "empty string multiple",
			scans:  [][]byte{[]byte(""), []byte(""), []byte("")},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name:   "only space",
			scans:  [][]byte{[]byte(" ")},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name:   "only space multiple",
			scans:  [][]byte{[]byte(" "), []byte("  ")},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name:   "only tab",
			scans:  [][]byte{[]byte("\t")},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name:   "only new line",
			scans:  [][]byte{[]byte("\n")},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name:   "only spaces",
			scans:  [][]byte{[]byte("\n \t  \n")},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "only spaces multiple",
			scans: [][]byte{
				[]byte("\n \t  \n"),
				[]byte("\n"),
				[]byte("\t"),
				[]byte("   "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "only spaces multiple with empty",
			scans: [][]byte{
				[]byte("\n \t  \n"),
				[]byte("\n"),
				[]byte(""),
				[]byte("\t"),
				[]byte("   "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "one word one symbol unhandled",
			scans: [][]byte{
				[]byte("a"),
			},
			assert: assertWords(1),
		}),

		newWordsTest(t, scanTest{
			name: "one word one symbol with spaces in begin unhandled",
			scans: [][]byte{
				[]byte("  a"),
			},
			assert: assertWords(1),
		}),

		newWordsTest(t, scanTest{
			name: "one word one symbol multiple",
			scans: [][]byte{
				[]byte("a"),
				[]byte("b"),
			},
			assert: assertWords(2),
		}),

		newWordsTest(t, scanTest{
			name: "one word one symbol multiple with empty",
			scans: [][]byte{
				[]byte("a"),
				[]byte(""),
				[]byte("b"),
				[]byte(""),
			},
			assert: assertWords(2),
		}),

		newWordsTest(t, scanTest{
			name: "one word with multiple symbols",
			scans: [][]byte{
				[]byte("abc"),
			},
			assert: assertWords(3),
		}),

		newWordsTest(t, scanTest{
			name: "one word with multiple symbols with spaces in begin",
			scans: [][]byte{
				[]byte("\t  \n abc"),
			},
			assert: assertWords(3),
		}),

		newWordsTest(t, scanTest{
			name: "one word with multiple symbols multiple",
			scans: [][]byte{
				[]byte("abc"),
				[]byte("def"),
				[]byte("g"),
			},
			assert: assertWords(7),
		}),

		newWordsTest(t, scanTest{
			name: "one word with multiple symbols multiple with empty",
			scans: [][]byte{
				[]byte("abc"),
				[]byte("def"),
				[]byte(""),
				[]byte("g"),
			},
			assert: assertWords(7),
		}),

		newWordsTest(t, scanTest{
			name: "one word",
			scans: [][]byte{
				[]byte("a "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "one word with empty",
			scans: [][]byte{
				[]byte(""),
				[]byte("a "),
				[]byte(""),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "one word multiple spaces",
			scans: [][]byte{
				[]byte(" \t"),
				[]byte("a "),
				[]byte(" "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "one word with spaces in begin",
			scans: [][]byte{
				[]byte("  \n\t b "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "two word one unhandled",
			scans: [][]byte{
				[]byte("d abc"),
			},
			assert: assertWords(3),
		}),

		newWordsTest(t, scanTest{
			name: "two word one unhandled with separate write",
			scans: [][]byte{
				[]byte("d"),
				[]byte(" "),
				[]byte("abcd"),
			},
			assert: assertWords(4),
		}),

		newWordsTest(t, scanTest{
			name: "two word with separate write",
			scans: [][]byte{
				[]byte("d"),
				[]byte(" "),
				[]byte("abcd "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "two word in one write",
			scans: [][]byte{
				[]byte("d abcde "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "multiple words in one write separated by spaces",
			scans: [][]byte{
				[]byte("d   abcde\nbbbb\tjase\t\n qqq "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "multiple words in one write separated by spaces multiple write",
			scans: [][]byte{
				[]byte("d   abcde\nbbbb\tjase\t\n qqq "),
				[]byte("c   abcde\nbbbb\tjase\t\nhgt"),
				[]byte(""),
				[]byte(" mmtf "),
			},
			assert: assertWords(0),
		}),

		newWordsTest(t, scanTest{
			name: "case with utf8",
			scans: [][]byte{
				[]byte(" abc\tdef\nghi\rjkl\fmno\vpqr\u0085stu\u00a0\n"),
			},
			assert: assertWords(0),
		}),

		newWordsExcessiveWhiteSpaceTest(t, scanTest{
			name:   "excessive white space",
			assert: assertWords(5),
		}),
	}

	doAllScannerTests(t, tests)
}

// Test the line splitter, including some carriage returns but no long lines.
func TestScanLongLines(t *testing.T) {
	lineNum := 0
	j := 0

	const allTokens = 2 * smallMaxTokenSize

	scans := make([][]byte, 0, 10*smallMaxTokenSize)
	for i := 0; i < allTokens; i++ {
		s := genLineForScanner(lineNum, j, true)
		if j < smallMaxTokenSize {
			j++
		} else {
			j--
		}

		lineNum++

		scans = append(scans, s)
	}

	assertLongLines := func(t *testing.T, tst *scanTest) {
		assertScannerNoError(t, tst)

		require.Len(t, tst.handler.handled, allTokens, "should have correct handled tokens")

		j := 0
		for indx, handled := range tst.handler.handled {
			expected := genLineForScanner(indx, j, false)

			if j < smallMaxTokenSize {
				j++
			} else {
				j--
			}

			assertLongLineBytes(t, indx, expected, handled)
		}
	}

	newLongLinesTest(t, scanTest{
		name:   "scan long lines",
		scans:  scans,
		assert: assertLongLines,
	}).Run(t)
}

func TestScanLineTooLong(t *testing.T) {
	lineNum := 0
	j := 0

	const scansLen = 3

	scans := make([][]byte, 0, 10*smallMaxTokenSize)

	for i := 0; i < 2*smallMaxTokenSize; i++ {
		s := genLineForScanner(lineNum, j, true)

		j++
		lineNum++

		toAdd := gotee.CopyBytes(s)

		if len(toAdd) == 0 || len(toAdd) <= scansLen-1 {
			scans = append(scans, toAdd)
			continue
		}

		ll := len(toAdd)
		for ; ll > 0; ll = len(toAdd) {
			toCopy := scansLen
			if ll < scansLen {
				toCopy = ll
			}
			b := gotee.CopyBytes(toAdd[0:toCopy])
			scans = append(scans, b)
			toAdd = toAdd[toCopy:]
		}
	}

	assertLineToLong := func(t *testing.T, tst *scanTest) {
		require.False(t, tst.handler.receiveLast, "should not receive last")
		require.Error(t, tst.scanError, "scan should return error")
		require.ErrorIs(t, tst.scanError, bufio.ErrTooLong, "scan should return correct error")

		require.Len(t, tst.handler.handled, smallMaxTokenSize+1, "len %d", len(tst.handler.handled))

		j := 0
		for indx, handled := range tst.handler.handled {
			expected := genLineForScanner(indx, j, false)
			j++
			assertLongLineBytes(t, indx, expected, handled)
		}

		indx := len(tst.handler.handled)
		expectedUnhandled := genLineForScanner(indx, j, false)
		unhandled := tst.scanner.Unhandled()
		assertLongLineBytes(t, indx, expectedUnhandled, unhandled)
	}

	newLongLinesTest(t, scanTest{
		name:   "scan line too long",
		scans:  scans,
		assert: assertLineToLong,
	}).Run(t)
}

func TestScanLines(t *testing.T) {
	newLinesTest := func(t *testing.T, tst scanTest) *scanTest {
		tst.split = bufio.ScanLines
		return newScanTest(t, tst)
	}

	longLine := strings.Repeat("a", 1024)

	const hugeNewLinesCount = 5000
	hugeNewLines := bytes.Repeat([]byte("\n"), hugeNewLinesCount)
	hugeNewLinesExpected := make([]string, hugeNewLinesCount)

	tests := []*scanTest{
		newLinesTest(t, scanTest{
			name: "empty string",
			scans: [][]byte{
				[]byte(""),
			},
			assert: assertHandledStrings(0, nil),
		}),

		newLinesTest(t, scanTest{
			name: "empty string multiple",
			scans: [][]byte{
				[]byte(""),
				[]byte(""),
				[]byte(""),
			},
			assert: assertHandledStrings(0, nil),
		}),

		newLinesTest(t, scanTest{
			name: "empty strings huge",
			scans: [][]byte{
				hugeNewLines,
			},
			assert: assertHandledStrings(0, hugeNewLinesExpected),
		}),

		newLinesTest(t, scanTest{
			name: "only space",
			scans: [][]byte{
				[]byte(" "),
			},
			assert: assertHandledStrings(1, []string{" "}),
		}),

		newLinesTest(t, scanTest{
			name: "only space multiple",
			scans: [][]byte{
				[]byte(" "),
				[]byte("  "),
			},
			assert: assertHandledStrings(3, []string{"   "}),
		}),

		newLinesTest(t, scanTest{
			name:   "only tab",
			scans:  [][]byte{[]byte("\t")},
			assert: assertHandledStrings(1, []string{"\t"}),
		}),

		newLinesTest(t, scanTest{
			name: "only new line",
			scans: [][]byte{
				[]byte("\n"),
			},
			assert: assertHandledStrings(0,
				[]string{""},
			),
		}),

		newLinesTest(t, scanTest{
			name: "multiple new lines",
			scans: [][]byte{
				[]byte("\n\n\n\n"),
			},
			assert: assertHandledStrings(0, []string{
				"", "", "", "",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "multiple new lines in multiple writes with empties",
			scans: [][]byte{
				[]byte("\n\n"),
				[]byte(""),
				[]byte("\n"),
				[]byte("\n\n\n"),
				[]byte(""),
			},
			assert: assertHandledStrings(0, []string{
				"", "", "", "", "", "",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "only spaces with tho new lines",
			scans: [][]byte{
				[]byte("\n \t  \n"),
			},
			assert: assertHandledStrings(0, []string{
				"", " \t  ",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "no new lines",
			scans: [][]byte{
				[]byte("abcde"),
			},
			assert: assertHandledStrings(5, []string{
				"abcde",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "no new lines in multiple with empty",
			scans: [][]byte{
				[]byte("abcde"),
				[]byte("a"),
				[]byte(""),
				[]byte("ccc"),
				[]byte(""),
				[]byte("cc"),
			},
			assert: assertHandledStrings(11, []string{
				"abcdeaccccc",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "multiple lines",
			scans: [][]byte{
				[]byte("ab\nc de\n  vb\t\n\t\tups\n"),
			},
			assert: assertHandledStrings(0, []string{
				"ab",
				"c de",
				"  vb\t",
				"\t\tups",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "multiple lines multiple writes",
			scans: [][]byte{
				[]byte("a\n"),
				[]byte("a b\n"),
				[]byte("\tde\n"),
				[]byte("fgh1\n"),
				[]byte("fgh1  \n"),
			},
			assert: assertHandledStrings(0, []string{
				"a",
				"a b",
				"\tde",
				"fgh1",
				"fgh1  ",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "multiple lines multiple writes with unhandled",
			scans: [][]byte{
				[]byte("a\n"),
				[]byte("a b\n"),
				[]byte("\tde\n"),
				[]byte("fgh1\n"),
				[]byte("fgh1  \n"),
				[]byte("\n"),
				[]byte("dddd"),
			},
			assert: assertHandledStrings(4, []string{
				"a",
				"a b",
				"\tde",
				"fgh1",
				"fgh1  ",
				"",
				"dddd",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "multiple lines multiple writes",
			scans: [][]byte{
				[]byte("a"),
				[]byte("a b\n"),
				[]byte("tde"),
				[]byte("\n"),
				[]byte("fg"),
				[]byte("h1  "),
				[]byte("\n"),
			},

			assert: assertHandledStrings(0, []string{
				"aa b",
				"tde",
				"fgh1  ",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "multiple new lines in end",
			scans: [][]byte{
				[]byte("a\na b\t\ntde\n\n\n"),
			},
			assert: assertHandledStrings(0, []string{
				"a",
				"a b\t",
				"tde",
				"",
				"",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "multiple new lines in middle with unhandled",
			scans: [][]byte{
				[]byte("a\n"),
				[]byte("a b"),
				[]byte("\n"),
				[]byte("\n"),
				[]byte("\n"),
				[]byte("\n"),
				[]byte("de"),
			},
			assert: assertHandledStrings(2, []string{
				"a",
				"a b",
				"",
				"",
				"",
				"de",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "multiple new lines with long with unhandled",
			scans: [][]byte{
				[]byte(longLine),
				[]byte("\n"),
				[]byte("bc"),
				[]byte("\n"),
				[]byte("de"),
			},
			assert: assertHandledStrings(2, []string{
				longLine,
				"bc",
				"de",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "long line as unhandled",
			scans: [][]byte{
				[]byte("\n"),
				[]byte("bc"),
				[]byte("\n"),
				[]byte("de"),
				[]byte("\n"),
				[]byte(longLine),
			},
			assert: assertHandledStrings(len(longLine), []string{
				"",
				"bc",
				"de",
				longLine,
			}),
		}),

		newLinesTest(t, scanTest{
			name: "with windows new lines only drop \\r",
			scans: [][]byte{
				[]byte("abc\r\nghi\r\njkl\r\n"),
			},
			assert: assertHandledStrings(0, []string{
				"abc",
				"ghi",
				"jkl",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "with windows new lines in multiple writes drop \\r",
			scans: [][]byte{
				[]byte("abc\r\n"),
				[]byte("ghi\r"),
				[]byte("\njkl\r"),
				[]byte("\n"),
			},
			assert: assertHandledStrings(0, []string{
				"abc",
				"ghi",
				"jkl",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "with windows new lines an unix in multiple writes drop \\r",
			scans: [][]byte{
				[]byte("abc\r\n"),
				[]byte("ghi\n"),
				[]byte("\njkl\r"),
				[]byte("\n"),
				[]byte("0&"),
			},
			assert: assertHandledStrings(2, []string{
				"abc",
				"ghi",
				"",
				"jkl",
				"0&",
			}),
		}),

		newLinesTest(t, scanTest{
			name: "with utf invalid and wyth \\r",
			scans: [][]byte{
				[]byte(" abc\nghi\rjkl\fmno\vpqr\u0085stu\u00a0\n"),
			},
			assert: assertHandledStrings(0, []string{
				" abc",
				"ghi\rjkl\fmno\vpqr\u0085stu\u00a0",
			}),
		}),
	}

	doAllScannerTests(t, tests)
}

// Test the correct error is returned when the split function errors out.
func TestScannerSplitError(t *testing.T) {
	testError := fmt.Errorf("testError")

	newSplitErrorTest := func(t *testing.T, tst scanTest, okCount int) *scanTest {
		numSplits := 0

		errorSplit := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if numSplits >= okCount {
				return 0, nil, testError
			}
			numSplits++
			return 1, data[0:1], nil
		}

		tst.split = errorSplit
		return newScanTest(t, tst)
	}

	assertScanErrors := func(expected [][]byte, unhandled []byte) assertScanner {
		return func(t *testing.T, tst *scanTest) {
			require.False(t, tst.handler.receiveLast, "should not receive last")
			require.Error(t, tst.scanError, "scanner should return error")
			require.ErrorIs(t, tst.scanError, testError, "should correct error")

			require.Len(t, tst.handler.handled, len(expected), "should correct handled len")
			for indx, handled := range tst.handler.handled {
				require.Equal(t, expected[indx], handled, "should correct handled")
			}

			unhandledRes := tst.scanner.Unhandled()
			lenExpectedUnhandled := len(unhandled)
			require.Len(t, unhandledRes, lenExpectedUnhandled, "should correct unhandled len")
			if lenExpectedUnhandled > 0 {
				require.Equal(t, unhandled, unhandledRes, "should correct unhandled")
			}
		}
	}

	tests := []*scanTest{
		newSplitErrorTest(t, scanTest{
			name: "at first",
			scans: [][]byte{
				[]byte("abc"),
			},
			assert: assertScanErrors(
				nil,
				[]byte("abc"),
			),
		}, 0),

		newSplitErrorTest(t, scanTest{
			name: "at middle",
			scans: [][]byte{
				[]byte("abc"),
			},
			assert: assertScanErrors(
				[][]byte{
					[]byte("a"),
				},

				[]byte("bc"),
			),
		}, 1),

		newSplitErrorTest(t, scanTest{
			name: "at middle multiple writes",
			scans: [][]byte{
				[]byte("ab"),
				[]byte("c"),
				[]byte("de"),
			},
			assert: assertScanErrors(
				[][]byte{
					[]byte("a"),
					[]byte("b"),
				},

				[]byte("c"),
			),
		}, 2),

		newSplitErrorTest(t, scanTest{
			name: "at middle multiple writes 2",
			scans: [][]byte{
				[]byte("ab"),
				[]byte("c"),
				[]byte("de"),
			},
			assert: assertScanErrors(
				[][]byte{
					[]byte("a"),
					[]byte("b"),
					[]byte("c"),
				},

				[]byte("de"),
			),
		}, 3),

		newSplitErrorTest(t, scanTest{
			name: "at end",
			scans: [][]byte{
				[]byte("abcd"),
			},
			assert: assertScanErrors(
				[][]byte{
					[]byte("a"),
					[]byte("b"),
					[]byte("c"),
				},

				[]byte("d"),
			),
		}, 3),

		newSplitErrorTest(t, scanTest{
			name: "at end multiple writes",
			scans: [][]byte{
				[]byte("ab"),
				[]byte("c"),
				[]byte("de"),
			},
			assert: assertScanErrors(
				[][]byte{
					[]byte("a"),
					[]byte("b"),
					[]byte("c"),
					[]byte("d"),
				},

				[]byte("e"),
			),
		}, 4),

		newSplitErrorTest(t, scanTest{
			name: "at end multiple writes last write",
			scans: [][]byte{
				[]byte("ab"),
				[]byte("cd"),
				[]byte("e"),
			},
			assert: assertScanErrors(
				[][]byte{
					[]byte("a"),
					[]byte("b"),
					[]byte("c"),
					[]byte("d"),
				},

				[]byte("e"),
			),
		}, 4),
	}

	doAllScannerTests(t, tests)
}

func TestScannerEndToken(t *testing.T) {
	endToken := []byte("\n")

	newSplitFinalTokenTest := func(t *testing.T, tst scanTest) *scanTest {
		errorSplit := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if data[0] == endToken[0] {
				return 0, data[0:1], bufio.ErrFinalToken
			}

			return 1, data[0:1], nil
		}

		tst.split = errorSplit
		return newScanTest(t, tst)
	}

	endWordToken := []byte("EOF")

	newSplitFinalTokenMultipleTest := func(t *testing.T, tst scanTest) *scanTest {
		errorSplit := func(data []byte, atEOF bool) (int, []byte, error) {
			advance, token, err := bufio.ScanWords(data, atEOF)
			if err != nil {
				return 0, nil, err
			}

			indx := bytes.Index(data, endWordToken)
			if indx < 0 {
				return advance, token, nil
			}

			if token == nil {
				indxWithFinalToken := indx + len(endWordToken)
				advance = indxWithFinalToken
				return advance, data[0:indxWithFinalToken], bufio.ErrFinalToken
			}

			indxInToken := bytes.Index(token, endWordToken)
			if indxInToken < 0 {
				return advance, token, nil
			}

			indxWithFinalToken := indx + len(endWordToken)

			return advance - indxWithFinalToken, token[0:indxWithFinalToken], bufio.ErrFinalToken
		}

		tst.split = errorSplit
		return newScanTest(t, tst)
	}

	assertScanToken := func(expected [][]byte, unhandled []byte) assertScanner {
		return func(t *testing.T, tst *scanTest) {
			require.True(t, tst.handler.receiveLast, "should receive last")
			require.NoError(t, tst.scanError, "scanner should not return error")

			require.Len(t, tst.handler.handled, len(expected), "should correct handled len")
			for indx, handled := range tst.handler.handled {
				require.Equal(t, expected[indx], handled, "should correct handled")
			}

			unhandledRes := tst.scanner.Unhandled()
			lenExpectedUnhandled := len(unhandled)
			require.Len(t, unhandledRes, lenExpectedUnhandled, "should correct unhandled len")
			if lenExpectedUnhandled > 0 {
				require.Equal(t, unhandled, unhandledRes, "should correct unhandled")
			}
		}
	}

	tests := []*scanTest{
		newSplitFinalTokenTest(t, scanTest{
			name: "at first and single",
			scans: [][]byte{
				endToken,
			},
			assert: assertScanToken(
				[][]byte{
					endToken,
				},
				nil,
			),
		}),

		newSplitFinalTokenTest(t, scanTest{
			name: "at first with unhandled",
			scans: [][]byte{
				[]byte("\nunh\n"),
			},
			assert: assertScanToken(
				[][]byte{
					endToken,
				},
				[]byte("unh\n"),
			),
		}),

		newSplitFinalTokenTest(t, scanTest{
			name: "at middle",
			scans: [][]byte{
				[]byte("unh\nend"),
			},

			assert: assertScanToken(
				[][]byte{
					[]byte("u"),
					[]byte("n"),
					[]byte("h"),
					endToken,
				},
				[]byte("end"),
			),
		}),

		newSplitFinalTokenTest(t, scanTest{
			name: "at middle",
			scans: [][]byte{
				[]byte("unh\nend"),
			},

			assert: assertScanToken(
				[][]byte{
					[]byte("u"),
					[]byte("n"),
					[]byte("h"),
					endToken,
				},
				[]byte("end"),
			),
		}),

		newSplitFinalTokenTest(t, scanTest{
			name: "at end",
			scans: [][]byte{
				[]byte("unhbc\n"),
			},

			assert: assertScanToken(
				[][]byte{
					[]byte("u"),
					[]byte("n"),
					[]byte("h"),
					[]byte("b"),
					[]byte("c"),
					endToken,
				},
				nil,
			),
		}),

		newSplitFinalTokenTest(t, scanTest{
			name: "at end multiple writes",
			scans: [][]byte{
				[]byte("abc"),
				[]byte("un\n"),
			},

			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("b"),
					[]byte("c"),
					[]byte("u"),
					[]byte("n"),
					endToken,
				},
				nil,
			),
		}),

		newSplitFinalTokenTest(t, scanTest{
			name: "at end multiple writes last",
			scans: [][]byte{
				[]byte("abc"),
				[]byte("un"),
				[]byte("\n"),
			},

			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("b"),
					[]byte("c"),
					[]byte("u"),
					[]byte("n"),
					endToken,
				},
				nil,
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: at first and single",
			scans: [][]byte{
				endWordToken,
			},
			assert: assertScanToken(
				[][]byte{
					endWordToken,
				},
				nil,
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: at first and single multiple writes",
			scans: [][]byte{
				[]byte("E"),
				[]byte("OF"),
			},
			assert: assertScanToken(
				[][]byte{
					endWordToken,
				},
				nil,
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: at first with unhandled",
			scans: [][]byte{
				[]byte("EOFun hd "),
			},
			assert: assertScanToken(
				[][]byte{
					endWordToken,
				},
				[]byte("un hd "),
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in middle",
			scans: [][]byte{
				[]byte("a ab EOF abc a "),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("ab"),
					endWordToken,
				},
				[]byte(" abc a "),
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in middle no token last",
			scans: [][]byte{
				[]byte("a abEOFabc a "),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("abEOF"),
				},
				[]byte("abc a "),
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in middle multiple writes",
			scans: [][]byte{
				[]byte("a "),
				[]byte("bc "),
				[]byte("E"),
				[]byte("O"),
				[]byte("F abc a"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("bc"),
					endWordToken,
				},
				[]byte(" abc a"),
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in middle multiple writes no token last",
			scans: [][]byte{
				[]byte("a "),
				[]byte("bc"),
				[]byte("EO"),
				[]byte("F abc a"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("bcEOF"),
				},
				[]byte(" abc a"),
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in end",
			scans: [][]byte{
				[]byte("a bc EOF"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("bc"),
					endWordToken,
				},
				nil,
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in end multiple writes",
			scans: [][]byte{
				[]byte("a"),
				[]byte(" "),
				[]byte("bc "),
				[]byte("EO"),
				[]byte("F"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("bc"),
					endWordToken,
				},
				nil,
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in end no last token",
			scans: [][]byte{
				[]byte("a bcEOF"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("bcEOF"),
				},
				nil,
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in end with unhandled no last token",
			scans: [][]byte{
				[]byte("a bcEOFad "),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("bcEOF"),
				},
				[]byte("ad "),
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in end multiple writes no last token",
			scans: [][]byte{
				[]byte("a "),
				[]byte("bcEO"),
				[]byte("F"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("bcEOF"),
				},
				nil,
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in end with unhandled",
			scans: [][]byte{
				[]byte("a bc EOF ad\x81"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("a"),
					[]byte("bc"),
					endWordToken,
				},
				[]byte(" ad\x81"),
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in end multiple writes with unhandled",
			scans: [][]byte{
				[]byte("ab"),
				[]byte(" a "),
				[]byte("EO"),
				[]byte("F ad\x81"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("ab"),
					[]byte("a"),
					endWordToken,
				},
				[]byte(" ad\x81"),
			),
		}),

		newSplitFinalTokenMultipleTest(t, scanTest{
			name: "final multiple: in end multiple writes with unhandled no last",
			scans: [][]byte{
				[]byte("ab"),
				[]byte(" a"),
				[]byte("EO"),
				[]byte("F ad\x81"),
			},
			assert: assertScanToken(
				[][]byte{
					[]byte("ab"),
					[]byte("aEOF"),
				},
				[]byte(" ad\x81"),
			),
		}),
	}

	doAllScannerTests(t, tests)
}

func TestScannerNoProgress(t *testing.T) {
	newNoProgressTest := func(t *testing.T, tst scanTest) *scanTest {
		tst.split = func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			return 0, nil, nil
		}

		return newScanTest(t, tst)
	}

	scans := make([][]byte, 1000)
	for i := 0; i < len(scans); i++ {
		scans[i] = []byte(" ")
	}

	assertNoProgress := func(t *testing.T, tst *scanTest) {
		require.False(t, tst.handler.receiveLast, "should not receive last")
		require.Error(t, tst.scanError, "should have error")
		require.ErrorIs(t, tst.scanError, io.ErrNoProgress, "should return no progress")
		require.Len(t, tst.handler.handled, 0, "should not any handle")
	}

	newNoProgressTest(t, scanTest{
		name:   "no progress",
		scans:  scans,
		assert: assertNoProgress,
	}).Run(t)
}

type assertScanner = func(t *testing.T, tst *scanTest)

type scanTest struct {
	name    string
	scanner *scan.NonBlockScanner
	handler *testTokenHandler
	split   bufio.SplitFunc
	scans   [][]byte
	assert  assertScanner

	lastScanIsStopped bool
	scanError         error
}

func (s *scanTest) Run(t *testing.T) {
	t.Run(s.name, func(t *testing.T) {
		for _, w := range s.scans {
			done, err := s.scanner.Scan(w)
			s.lastScanIsStopped = done
			if err != nil {
				s.scanError = err
				break
			}

			if done {
				break
			}
		}

		s.assert(t, s)
	})
}

func assertScannerNoError(t *testing.T, tst *scanTest) {
	require.NoError(t, tst.scanError, "should not have error")
	require.False(t, tst.handler.receiveLast, "should not have receive last")
}

func doAllScannerTests(t *testing.T, tests []*scanTest) {
	for _, tst := range tests {
		tst.Run(t)
	}
}

type testTokenHandler struct {
	handled     [][]byte
	receiveLast bool
}

func (h *testTokenHandler) NewToken(token []byte, isLast bool) {
	h.receiveLast = isLast

	cpy := make([]byte, len(token))
	copy(cpy, token)

	h.handled = append(h.handled, cpy)
}

func newScanTest(t *testing.T, tt scanTest) *scanTest {
	require.NotEmpty(t, tt.name, "name should set")
	require.False(t, len(tt.scans) == 0, "scans should passed")
	require.NotNil(t, tt.assert, "assert should passed")

	handler := &testTokenHandler{}
	scanner := scan.NewNonBlockScanner(handler)
	if tt.split != nil {
		scanner.Split(tt.split)
	}

	tt.scanner = scanner
	tt.handler = handler

	return &tt
}

// genLine writes to buf a predictable but non-trivial line of text of length
// n, including the terminal newline and an occasional carriage return.
// If addNewline is false, the \r and \n are not emitted.
func genLineForScanner(lineNum, n int, addNewline bool) []byte {
	b := bytes.Buffer{}
	doCR := lineNum%5 == 0
	if doCR {
		n--
	}
	for i := 0; i < n-1; i++ { // Stop early for \n.
		c := 'a' + byte(lineNum+i)
		if c == '\n' || c == '\r' { // Don't confuse us.
			c = 'N'
		}
		b.WriteByte(c)
	}
	if addNewline {
		if doCR {
			b.WriteByte('\r')
		}
		b.WriteByte('\n')
	}

	res := b.Bytes()
	if res == nil {
		res = make([]byte, 0)
	}

	return res
}

func newLongLinesTest(t *testing.T, tst scanTest) *scanTest {
	tst.split = bufio.ScanLines

	res := newScanTest(t, tst)
	res.scanner.PrivateMaxTokenSize(smallMaxTokenSize)

	return res
}

func assertLongLineBytes(t *testing.T, indx int, expected, handled []byte) {
	msg := fmt.Sprintf(
		"%d: bad line: %d %d\n%.100q\n%.100q\n",
		indx,
		len(handled),
		len(expected),
		string(handled),
		string(expected),
	)

	require.Equal(t, expected, handled, msg)
}

func assertHandledStrings(unhandled int, expected []string) assertScanner {
	return func(t *testing.T, tst *scanTest) {
		assertScannerNoError(t, tst)

		unhandledRes := tst.scanner.Unhandled()
		require.Len(t, unhandledRes, unhandled, "should have unhandled bytes")

		if len(tst.handler.handled) > 0 {
			handleExpected := len(expected)
			if unhandled > 0 {
				handleExpected = handleExpected - 1
			}

			require.Len(t, tst.handler.handled, handleExpected, "should have correct handles")

			i := 0
			for _, h := range tst.handler.handled {
				require.Equal(t, expected[i], string(h), "%d should handle correct strings", i)
				i++
			}

			if unhandled > 0 {
				require.Equal(t, expected[i], string(unhandledRes), "%d should handle correct last string with unhandled", i)
			}
			return
		}

		if unhandled > 0 {
			require.Len(t, expected, 1, "should have one string when have unhandled bytes with not handled")
			require.Equal(t, expected[0], string(unhandledRes), "unhandled should equal to single string")
		}
	}
}