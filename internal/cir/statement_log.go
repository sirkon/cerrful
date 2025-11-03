package cir

import "fmt"

// Log represents a logging statement that records an error variable
// at a certain log level using a specific logging function or method.
//
// Examples:
//
//	logger.Error("something failed", zap.Err(err))
//	// Var: "err", Level: "error", Msg: "something failed",
//	// Ref: "logger/pkg"."LoggerType"."Error"
//
//	zap.Debug().Err(err).Msg("something failed")
//	// Var: "err", Level: "warn", Msg: "something failed",
//	// Ref: "github.com/rz/zap"."Logger"."Error"
//
//	fmt.Println(err)
//	// Var: "err", Level: "warn", Msg: "", Ref: "fmt"."Println"
//
//	panic(err)
//	// Var: "err", Level: "fatal", Msg: "", Ref: "builtin"."panic"
type Log struct {
	Var   Expr
	Level LogLevel
	Msg   string
	Ref   Reference
}

// LogLevel defines the severity level of a logging operation.
type LogLevel int

const (
	LogLevelUnknown LogLevel = iota
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// String returns the string representation of a LogLevel value.
func (l LogLevel) String() string {
	switch l {
	case LogLevelWarn:
		return "warn"
	case LogLevelError:
		return "error"
	case LogLevelFatal:
		return "fatal"
	default:
		return fmt.Sprintf("unknown(%d)", l)
	}
}

func (*Log) isNode()      {}
func (*Log) isStatement() {}
