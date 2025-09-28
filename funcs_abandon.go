package main

import (
	"maps"
)

// Some funcs are known for stopping current func execution or even stopping the whole program.
// Some of them can be used to report an error, some just stops the flow and this means a proper
// logging should be made before the call.
//
// This project provides means to detect these and report their usage signature types.
type knownAbandonFuncs struct {
	known map[packagedFunc]SigAbandonType
}

func newKnownAbandonFuncs(custom map[packagedFunc]SigAbandonType) *knownAbandonFuncs {
	predefined := map[packagedFunc]SigAbandonType{
		// Stdlib.
		{pkgPath: "builtin", name: "panic"}:  SigAbandonTypeInvalid,
		{pkgPath: "os", name: "Exit"}:        SigAbandonTypeInvalid,
		{pkgPath: "testing", name: "Fatal"}:  SigAbandonTypeFormat,
		{pkgPath: "testing", name: "Fatalf"}: SigAbandonTypeFormat,
		{pkgPath: "log", name: "Fatal"}:      SigAbandonTypeFormat,
		{pkgPath: "log", name: "Fatalf"}:     SigAbandonTypeFormat,
		{pkgPath: "log", name: "Fatalln"}:    SigAbandonTypeFormat,
		{pkgPath: "log", name: "Panic"}:      SigAbandonTypeFormat,
		{pkgPath: "log", name: "Panicf"}:     SigAbandonTypeFormat,
		{pkgPath: "log", name: "Panicln"}:    SigAbandonTypeFormat,

		// Zap.
		{pkgPath: "github.com/uber-go/zap", name: "DPanic"}: SigAbandonTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "Panic"}:  SigAbandonTypeZap,
		{pkgPath: "github.com/uber-go/zap", name: "Fatal"}:  SigAbandonTypeZap,

		// My bias again!
		{pkgPath: "github.com/sirkon/message", name: "Fatal"}:     SigAbandonTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Fatalf"}:    SigAbandonTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Critical"}:  SigAbandonTypeFormat,
		{pkgPath: "github.com/sirkon/message", name: "Criticalf"}: SigAbandonTypeFormat,
	}

	if custom == nil {
		custom = map[packagedFunc]SigAbandonType{}
	} else {
		custom = maps.Clone(custom)
	}

	maps.Insert(custom, maps.All(predefined))

	return &knownAbandonFuncs{
		known: custom,
	}
}
