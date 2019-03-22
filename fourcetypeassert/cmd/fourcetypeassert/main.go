package main

import (
	"github.com/gostaticanalysis/exp/fourcetypeassert"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(fourcetypeassert.Analyzer) }