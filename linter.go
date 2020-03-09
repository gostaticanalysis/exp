// +build tools

package main

import (
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
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	unitchecker.Main(
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
	)
}
