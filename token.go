package tomledit

// TokenType identifies the kind of lexical token.
type TokenType int

const (
	TokenBareKey TokenType = iota
	TokenBasicString
	TokenLiteralString
	TokenMultiLineBasicString
	TokenMultiLineLiteralString
	TokenInteger
	TokenFloat
	TokenBoolean
	TokenOffsetDateTime
	TokenLocalDateTime
	TokenLocalDate
	TokenLocalTime
	TokenEquals
	TokenDot
	TokenComma
	TokenLeftBracket
	TokenRightBracket
	TokenDoubleLeftBracket
	TokenDoubleRightBracket
	TokenLeftBrace
	TokenRightBrace
	TokenComment
	TokenWhitespace
	TokenNewline
	TokenEOF
)

var tokenTypeNames = [...]string{
	TokenBareKey:                "BareKey",
	TokenBasicString:            "BasicString",
	TokenLiteralString:          "LiteralString",
	TokenMultiLineBasicString:   "MultiLineBasicString",
	TokenMultiLineLiteralString: "MultiLineLiteralString",
	TokenInteger:                "Integer",
	TokenFloat:                  "Float",
	TokenBoolean:                "Boolean",
	TokenOffsetDateTime:         "OffsetDateTime",
	TokenLocalDateTime:          "LocalDateTime",
	TokenLocalDate:              "LocalDate",
	TokenLocalTime:              "LocalTime",
	TokenEquals:                 "Equals",
	TokenDot:                    "Dot",
	TokenComma:                  "Comma",
	TokenLeftBracket:            "LeftBracket",
	TokenRightBracket:           "RightBracket",
	TokenDoubleLeftBracket:      "DoubleLeftBracket",
	TokenDoubleRightBracket:     "DoubleRightBracket",
	TokenLeftBrace:              "LeftBrace",
	TokenRightBrace:             "RightBrace",
	TokenComment:                "Comment",
	TokenWhitespace:             "Whitespace",
	TokenNewline:                "Newline",
	TokenEOF:                    "EOF",
}

// String returns the human-readable name of the token type.
func (t TokenType) String() string {
	if int(t) >= 0 && int(t) < len(tokenTypeNames) {
		return tokenTypeNames[t]
	}
	return "Unknown"
}

// Token represents a single lexical token from TOML source.
type Token struct {
	Type   TokenType
	Raw    []byte // exact bytes from source
	Line   int    // 1-based
	Column int    // 1-based
}
