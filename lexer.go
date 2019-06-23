package main

import (
	"fmt"
	"log"
	"strconv"
	"unicode"
)

const (
	_ = iota
	Block
	UnaryExpr
	BinaryExpr
	MatchExpr
	MatchClause

	Identifier
	EmptyIdentifier

	FunctionCall

	NumberLiteral
	StringLiteral
	ObjectLiteral
	ListLiteral
	FunctionLiteral

	TrueLiteral
	FalseLiteral
	NullLiteral

	// ambiguous operators and symbols
	AccessorOp

	EqRefOp

	// =
	EqualOp
	FunctionArrow

	// :
	KeyValueSeparator
	DefineOp
	MatchColon

	// -
	CaseArrow
	SubtractOp

	// single char, unambiguous
	NegationOp
	AddOp
	MultiplyOp
	DivideOp
	ModulusOp
	GreaterThanOp
	LessThanOp

	Separator
	LeftParen
	RightParen
	LeftBracket
	RightBracket
	LeftBrace
	RightBrace
)

// TODO: use just line and col with span, not an actual span. It's good enough.
type span struct {
	startLine, startCol int
	endLine, endCol     int
}

func (sp *span) String() string {
	return fmt.Sprintf("%d:%d - %d:%d",
		sp.startLine, sp.startCol,
		sp.endLine, sp.endCol)
}

type Tok struct {
	val  interface{}
	kind int
	span
}

func (tok *Tok) stringVal() string {
	switch v := tok.val.(type) {
	case string:
		return string(v)
	case float64:
		return strconv.FormatFloat(float64(v), 'f', -1, 64)
	default:
		return ""
	}
}

func (tok *Tok) numberVal() float64 {
	switch v := tok.val.(type) {
	case float64:
		return float64(v)
	default:
		return 0
	}
}

func (tok Tok) String() string {
	str := tok.stringVal()
	if str == "" {
		return fmt.Sprintf("%s - %s",
			tokKindToName(tok.kind),
			tok.span.String())
	} else {
		return fmt.Sprintf("%s %s - %s",
			tokKindToName(tok.kind),
			str,
			tok.span.String())
	}
}

func Tokenize(input <-chan rune, tokens chan<- Tok, done chan<- bool) {
	lastTokKind := Separator
	buf := ""
	strbuf := ""
	var strbufStartLine, strbufStartCol int

	lineNo := 1
	colNo := 1

	simpleCommit := func(tok Tok) {
		lastTokKind = tok.kind
		tokens <- tok
	}
	simpleCommitChar := func(kind int) {
		simpleCommit(Tok{
			val:  "",
			kind: kind,
			span: span{lineNo, colNo, lineNo, colNo + 1},
		})
	}
	ensureSeparator := func() {
		// no-op, re-bound below
		log.Fatalf("This function should never run!")
	}
	commitClear := func() {
		if buf != "" {
			cbuf := buf
			buf = ""
			switch cbuf {
			case "is":
				simpleCommitChar(EqRefOp)
			case "true":
				simpleCommitChar(TrueLiteral)
			case "false":
				simpleCommitChar(FalseLiteral)
			case "null":
				simpleCommitChar(NullLiteral)
			default:
				if unicode.IsDigit(rune(cbuf[0])) {
					f, err := strconv.ParseFloat(cbuf, 64)
					if err != nil {
						log.Fatalf("Parsing error in number at %d:%d, %s", lineNo, colNo, err.Error())
					}
					simpleCommit(Tok{
						val:  f,
						kind: NumberLiteral,
						span: span{lineNo, colNo - len(cbuf), lineNo, colNo + 1},
					})
				} else {
					simpleCommit(Tok{
						val:  cbuf,
						kind: Identifier,
						span: span{lineNo, colNo - len(cbuf), lineNo, colNo + 1},
					})
				}
			}
		}
	}
	commit := func(tok Tok) {
		commitClear()
		simpleCommit(tok)
	}
	commitChar := func(kind int) {
		commit(Tok{
			val:  "",
			kind: kind,
			span: span{lineNo, colNo, lineNo, colNo + 1},
		})
	}
	ensureSeparator = func() {
		commitClear()
		switch lastTokKind {
		case Separator, LeftParen, LeftBracket, LeftBrace:
			// do nothing
		default:
			commitChar(Separator)
		}
	}

	inStringLiteral := false

	go func() {
		for char := range input {
			switch {
			case char == '\'':
				if inStringLiteral {
					commit(Tok{
						val:  strbuf,
						kind: StringLiteral,
						span: span{strbufStartLine, strbufStartCol, lineNo, colNo + 1},
					})
				} else {
					strbuf = ""
					strbufStartLine, strbufStartCol = lineNo, colNo
				}
				inStringLiteral = !inStringLiteral
			case inStringLiteral:
				strbuf += string(char)
			case char == '`':
				nextChar := <-input
				for nextChar != '`' {
					nextChar = <-input
				}
			case char == '\n':
				lineNo++
				colNo = 1
				ensureSeparator()
			case unicode.IsSpace(char):
				commitClear()
			case char == '_':
				commitChar(EmptyIdentifier)
			case char == '~':
				commitChar(NegationOp)
			case char == '+':
				commitChar(AddOp)
			case char == '*':
				commitChar(MultiplyOp)
			case char == '/':
				commitChar(DivideOp)
			case char == '%':
				commitChar(ModulusOp)
			case char == '<':
				commitChar(LessThanOp)
			case char == '>':
				commitChar(GreaterThanOp)
			case char == ',':
				commitChar(Separator)
			case char == ':':
				nextChar := <-input
				if unicode.IsSpace(nextChar) {
					ensureSeparator()
					commitChar(KeyValueSeparator)
				} else if nextChar == '=' {
					commitChar(DefineOp)
				} else if nextChar == ':' {
					commitChar(MatchColon)
				} else {
					buf += ":" + string(nextChar)
				}
			case char == '.':
				nextChar := <-input
				if unicode.IsDigit(nextChar) {
					buf += "."
				} else {
					commitChar(AccessorOp)
				}
				buf += string(nextChar)
			case char == '=':
				nextChar := <-input
				if nextChar == '>' {
					commitChar(FunctionArrow)
				} else if unicode.IsSpace(nextChar) {
					commitChar(EqualOp)
				} else {
					buf += "=" + string(nextChar)
				}
			case char == '-':
				nextChar := <-input
				if nextChar == '>' {
					commitChar(CaseArrow)
				} else if unicode.IsSpace(nextChar) {
					commitChar(SubtractOp)
				} else {
					buf += "-" + string(nextChar)
				}
			case char == '(':
				commitChar(LeftParen)
			case char == ')':
				ensureSeparator()
				commitChar(RightParen)
			case char == '[':
				commitChar(LeftBracket)
			case char == ']':
				ensureSeparator()
				commitChar(RightBracket)
			case char == '{':
				commitChar(LeftBrace)
			case char == '}':
				ensureSeparator()
				commitChar(RightBrace)
			default:
				buf += string(char)
			}
			colNo++
		}

		close(tokens)
		done <- true
	}()
}

func isValidIdentifierStartChar(char rune) bool {
	if unicode.IsLetter(char) {
		return true
	}

	switch char {
	case '@', '!', '?':
		return true
	default:
		return false
	}
}

func isValidIdentifierChar(char rune) bool {
	return isValidIdentifierStartChar(char) || unicode.IsDigit(char)
}

func tokKindToName(kind int) string {
	switch kind {
	case Block:
		return "Block"
	case UnaryExpr:
		return "UnaryExpr"
	case BinaryExpr:
		return "BinaryExpr"
	case MatchExpr:
		return "MatchExpr"
	case MatchClause:
		return "MatchClause"

	case Identifier:
		return "Identifier"
	case EmptyIdentifier:
		return "EmptyIdentifier"

	case FunctionCall:
		return "FunctionCall"

	case NumberLiteral:
		return "NumberLiteral"
	case StringLiteral:
		return "StringLiteral"
	case ObjectLiteral:
		return "ObjectLiteral"
	case ListLiteral:
		return "ListLiteral"
	case FunctionLiteral:
		return "FunctionLiteral"

	case TrueLiteral:
		return "TrueLiteral"
	case FalseLiteral:
		return "FalseLiteral"
	case NullLiteral:
		return "NullLiteral"

	case AccessorOp:
		return "AccessorOp"

	case EqRefOp:
		return "EqRefOp"

	case EqualOp:
		return "EqualOp"
	case FunctionArrow:
		return "FunctionArrow"

	case KeyValueSeparator:
		return "KeyValueSeparator"
	case DefineOp:
		return "DefineOp"
	case MatchColon:
		return "MatchColon"

	case CaseArrow:
		return "CaseArrow"
	case SubtractOp:
		return "SubtractOp"

	case NegationOp:
		return "NegationOp"
	case AddOp:
		return "AddOp"
	case MultiplyOp:
		return "MultiplyOp"
	case DivideOp:
		return "DivideOp"
	case ModulusOp:
		return "ModulusOp"
	case GreaterThanOp:
		return "GreaterThanOp"
	case LessThanOp:
		return "LessThanOp"

	case Separator:
		return "Separator"
	case LeftParen:
		return "LeftParen"
	case RightParen:
		return "RightParen"
	case LeftBracket:
		return "LeftBracket"
	case RightBracket:
		return "RightBracket"
	case LeftBrace:
		return "LeftBrace"
	case RightBrace:
		return "RightBrace"

	default:
		return "UnknownToken"
	}
}
