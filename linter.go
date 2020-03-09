// +build tools

package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/unitchecker"

	"github.com/gostaticanalysis/builtinprint"
	"github.com/gostaticanalysis/called"
	"github.com/gostaticanalysis/ctxfield"
	"github.com/gostaticanalysis/dupimport"
	"github.com/gostaticanalysis/importgroup"
	"github.com/gostaticanalysis/lion"
	"github.com/gostaticanalysis/nofmt"
	"github.com/gostaticanalysis/noreplace"
	"github.com/gostaticanalysis/notest"
	"github.com/gostaticanalysis/typeswitch"
	"github.com/gostaticanalysis/unused"
	"github.com/gostaticanalysis/vetgen/analyzers"
)

func main() {
	unitchecker.Main(append(
		analyzers.Govet(),
		[]*analysis.Analyzer{
			builtinprint.Analyzer,
			called.Analyzer,
			ctxfield.Analyzer,
			dupimport.Analyzer,
			importgroup.Analyzer,
			lion.Analyzer,
			nofmt.Analyzer,
			noreplace.Analyzer,
			notest.Analyzer,
			typeswitch.Analyzer,
			unused.Analyzer,
		}...,
	)...)
}
