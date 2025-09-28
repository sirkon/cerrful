package main

import (
	"maps"
)

type knownLoggingFuncs struct {
	known map[packagedFunc]SigLoggingType
}

func newKnownLoggingFuncs(custom map[packagedFunc]SigLoggingType) *knownLoggingFuncs {
	predefined := map[packagedFunc]SigLoggingType{
		// Stdlib.
		{pkgPath: "builtin", name: "print"}:   SigLoggingTypeFormat,
		{pkgPath: "builtin", name: "printf"}:  SigLoggingTypeFormat,
		{pkgPath: "builtin", name: "println"}: SigLoggingTypeFormat,
		{pkgPath: "fmt", name: "Print"}:       SigLoggingTypeFormat,
		{pkgPath: "fmt", name: "Printf"}:      SigLoggingTypeFormat,
		{pkgPath: "fmt", name: "Println"}:     SigLoggingTypeFormat,
		{pkgPath: "log", name: "Print"}:       SigLoggingTypeFormat,
		{pkgPath: "log", name: "Printf"}:      SigLoggingTypeFormat,
		{pkgPath: "log", name: "Println"}:     SigLoggingTypeFormat,
		{pkgPath: "log", name: "Panic"}:       SigLoggingTypeFormat,
		{pkgPath: "log", name: "Panicf"}:      SigLoggingTypeFormat,
		{pkgPath: "log", name: "Panicln"}:     SigLoggingTypeFormat,
		{pkgPath: "log", name: "Fatal"}:       SigLoggingTypeFormat,
		{pkgPath: "log", name: "Fatalf"}:      SigLoggingTypeFormat,
		{pkgPath: "log", name: "Fatalln"}:     SigLoggingTypeFormat,
		{pkgPath: "log/slog", name: "Debug"}:  SigLoggingTypeSlog,
		{pkgPath: "log/slog", name: "Info"}:   SigLoggingTypeSlog,
		{pkgPath: "log/slog", name: "Warn"}:   SigLoggingTypeSlog,
		{pkgPath: "log/slog", name: "Error"}:  SigLoggingTypeSlog,

		// Zap.
		{pkgPath: "github.com/uber-go/zap", name: "Log"}:    SigLoggingTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "Debug"}:  SigLoggingTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "Info"}:   SigLoggingTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "Warn"}:   SigLoggingTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "Error"}:  SigLoggingTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "DPanic"}: SigLoggingTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "Panic"}:  SigLoggingTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "Fatal"}:  SigLoggingTypeZap,

		// Zerolog
		{pkgPath: "github.com/rs/zerolog/log", name: "Msg"}:   SigLoggingTypeZap,
		{pkgPath: "github.com/rs/zerolog/log", name: "Msgf"}:  SigLoggingTypeZap,
		{pkgPath: "github.com/rs/zerolog/log", name: "Print"}: SigLoggingTypeZap,
		{pkgPath: "github.com/rs/zerolog", name: "Msg"}:       SigLoggingTypeZap,
		{pkgPath: "github.com/rs/zerolog", name: "Msgf"}:      SigLoggingTypeZap,

		// My bias in work!
		{pkgPath: "github.com/sirkon/message", name: "Debug"}:     SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Debugf"}:    SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Info"}:      SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Infof"}:     SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Notice"}:    SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Noticef"}:   SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Warning"}:   SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Warningf"}:  SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Error"}:     SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Errorf"}:    SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Critical"}:  SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Criticalf"}: SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Fatal"}:     SigLoggingTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Fatalf"}:    SigLoggingTypeFormat,
	}

	if custom == nil {
		custom = map[packagedFunc]SigLoggingType{}
	} else {
		custom = maps.Clone(custom)
	}

	maps.Insert(custom, maps.All(predefined))

	return &knownLoggingFuncs{known: custom}
}
