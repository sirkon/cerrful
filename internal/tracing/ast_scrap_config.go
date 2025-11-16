package tracing

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/sirkon/cerrful/internal/cir"
)

// WrapKind represents different wrap strategies (fmt-style, errors-style).
type WrapKind int

const (
	_ WrapKind = iota
	WrapKindFmt
	WrapKindErrors
)

func (k *WrapKind) String() string {
	v, err := k.MarshalText()
	if err != nil {
		return fmt.Sprintf("wrap-kind-invalid(%d)", *k)
	}

	return string(v)
}

var _ encoding.TextUnmarshaler = (*WrapKind)(nil)

func (k *WrapKind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "fmt":
		*k = WrapKindFmt
		return nil
	case "errors":
		*k = WrapKindErrors
		return nil
	default:
		return fmt.Errorf("unknown kind %q of wrap", b)
	}
}

func (k *WrapKind) MarshalText() ([]byte, error) {
	switch *k {
	case WrapKindFmt:
		return []byte("fmt"), nil
	case WrapKindErrors:
		return []byte("errors"), nil
	default:
		return nil, fmt.Errorf("cannot marshal invalid WrapKind(%d)", *k)
	}
}

// LoggingKind represents the style of logging (format-like, zap-like).
type LoggingKind int

const (
	_ LoggingKind = iota
	LoggingKindFormat
	LoggingKindZap
	LoggingKindZeroLog
)

func (k *LoggingKind) String() string {
	v, err := k.MarshalText()
	if err != nil {
		return fmt.Sprintf("logging-kind-invalid(%d)", *k)
	}

	return string(v)
}

var _ encoding.TextUnmarshaler = (*LoggingKind)(nil)

func (k *LoggingKind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "format":
		*k = LoggingKindFormat
		return nil
	case "zap":
		*k = LoggingKindZap
		return nil
	case "zerolog":
		*k = LoggingKindZeroLog
		return nil
	default:
		return fmt.Errorf("unknown kind %q of logger", b)
	}
}

func (k *LoggingKind) MarshalText() ([]byte, error) {
	switch *k {
	case LoggingKindFormat:
		return []byte("format"), nil
	case LoggingKindZap:
		return []byte("zap"), nil
	case LoggingKindZeroLog:
		return []byte("zerolog"), nil
	default:
		return nil, fmt.Errorf("cannot marshal invalid LoggingKind(%d)", *k)
	}
}

// Reference is a full twin of [cir.Reference] defined for proper layer isolation.
type Reference struct {
	Package string
	Type    string
	Name    string
}

func (r *Reference) CIR() cir.Reference {
	return cir.Reference{
		Package: r.Package,
		Type:    r.Type,
		Name:    r.Name,
	}
}

var _ encoding.TextUnmarshaler = (*Reference)(nil)

func (r *Reference) UnmarshalText(b []byte) error {
	s := string(bytes.TrimSpace(b))
	if s == "" {
		return errors.New("empty reference")
	}

	// Expected forms:
	//   "pkg/path".Name
	//   "pkg/path".Type.Name

	// 1) split at the quoted package
	if !strings.HasPrefix(s, `"`) {
		return fmt.Errorf("reference must start with quoted package: %q", s)
	}
	end := strings.Index(s[1:], `"`)
	if end < 0 {
		return fmt.Errorf("unterminated quoted package in reference: %q", s)
	}
	end++ // include the first quote

	pkg := s[1:end]
	if pkg == "" {
		return fmt.Errorf("package cannot be empty in reference: %q", s)
	}

	rest := strings.TrimPrefix(s[end+1:], ".")
	if rest == "" {
		return fmt.Errorf("reference must contain a name: %q", s)
	}

	parts := strings.Split(rest, ".")
	if len(parts) < 1 || len(parts) > 2 {
		return fmt.Errorf("reference must have 1 or 2 identifiers after package: %q", s)
	}

	for _, p := range parts {
		if !isIdent(p) {
			return fmt.Errorf("invalid identifier %q in reference %q", p, s)
		}
	}

	r.Package = pkg
	switch len(parts) {
	case 1:
		r.Type = ""
		r.Name = parts[0]
	case 2:
		r.Type = parts[0]
		r.Name = parts[1]
	}

	return nil
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func (r Reference) MarshalText() ([]byte, error) {
	if r.Package == "" {
		return nil, fmt.Errorf("cannot marshal Reference: empty Package")
	}
	if r.Name == "" {
		return nil, fmt.Errorf("cannot marshal Reference: empty Name")
	}

	// Base: "pkg"
	var b strings.Builder
	b.WriteByte('"')
	b.WriteString(r.Package)
	b.WriteByte('"')
	b.WriteByte('.')

	// Optional type
	if r.Type != "" {
		b.WriteString(r.Type)
		b.WriteByte('.')
	}

	// Name
	b.WriteString(r.Name)

	return []byte(b.String()), nil
}

// WrapSpec describes a registered wrap function.
type WrapSpec struct {
	Ref  Reference
	Kind WrapKind
}

// LoggerSpec describes a registered logger function.
type LoggerSpec struct {
	Ref  Reference
	Kind LoggingKind
}

// NewSpec describes a registered constructor-like new function.
type NewSpec struct {
	Ref Reference
}

// IgnoredError marks an error type that should be treated as non-error
// during analysis. These represent values such as io.EOF or context.Canceled
// in circumstances where they do not indicate an actual failure.
type IgnoredError struct {
	Ref Reference
}
