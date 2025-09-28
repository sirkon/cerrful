package main

import (
	"fmt"
)

type packagedFunc struct {
	pkgPath string
	name    string
}

// SigWrapType describes varieties of errors wrapping.
type SigWrapType int

const (
	SigWrapTypeInvalid SigWrapType = iota

	// SigWrapTypeErrorf demands an error to be an argument of the list.
	SigWrapTypeErrorf

	// SigWrapTypeWrap demands an error to be the first variable of the call and the message to be not empty.
	SigWrapTypeWrap
)

var sigTypeValueMap = map[SigWrapType]string{
	SigWrapTypeErrorf: "errorf",
	SigWrapTypeWrap:   "wrap",
}

func (s SigWrapType) String() string {
	v, ok := sigTypeValueMap[s]
	if !ok {
		return fmt.Sprintf("invalid(%d)", s)
	}

	return v
}

// UnmarshalText for setting values with configs, CLI, etc.
func (s *SigWrapType) UnmarshalText(rawtext []byte) error {
	text := string(rawtext)
	for k, v := range sigTypeValueMap {
		if v == text {
			*s = k
		}
	}

	return fmt.Errorf("unknown error wrap type %q", text)
}

// SigLoggingType describes varieties of logging.
type SigLoggingType int

const (
	SigLoggingTypeInvalid SigLoggingType = iota
	SigLoggingTypeFormat
	SigLoggingTypeZap
	SigLoggingTypeZerolog
	SigLoggingTypeSlog

	// TODO support more logging types.
)

var sigLoggingTypeValueMap = map[SigLoggingType]string{
	SigLoggingTypeFormat:  "format",
	SigLoggingTypeZap:     "zap",
	SigLoggingTypeZerolog: "zerolog",
	SigLoggingTypeSlog:    "slog",
}

func (s SigLoggingType) String() string {
	v, ok := sigLoggingTypeValueMap[s]
	if !ok {
		return fmt.Sprintf("invalid(%d)", s)
	}

	return v
}

func (s *SigLoggingType) UnmarshalText(rawtext []byte) error {
	text := string(rawtext)
	for k, v := range sigLoggingTypeValueMap {
		if v == text {
			*s = k
		}
	}

	return fmt.Errorf("unknown error logging type %q", text)
}

// SigAbandonType describes varieties of execution abandoning.
type SigAbandonType int

const (
	SigAbandonTypeInvalid SigAbandonType = iota

	SigAbandonTypeSilent
	SigAbandonTypeFormat
	SigAbandonTypeZap
	SigAbandonTypeZerolog
)

var sigAbandonTypeValueMap = map[SigAbandonType]string{
	SigAbandonTypeSilent:  "silent",
	SigAbandonTypeFormat:  "format",
	SigAbandonTypeZap:     "zap",
	SigAbandonTypeZerolog: "zerolog",
}

func (s SigAbandonType) String() string {
	v, ok := sigAbandonTypeValueMap[s]
	if !ok {
		return fmt.Sprintf("invalid(%d)", s)
	}

	return v
}

func (s *SigAbandonType) UnmarshalText(rawtext []byte) error {
	text := string(rawtext)
	for k, v := range sigAbandonTypeValueMap {
		if v == text {
			*s = k
			return nil
		}
	}

	return fmt.Errorf("unknown execution abandon type %q", text)
}
