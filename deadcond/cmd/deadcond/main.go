package main

import (
	"github.com/gostaticanalysis/exp/deadcond"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(deadcond.Analyzer) }
