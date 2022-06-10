package main

import (
	"github.com/gostaticanalysis/cmap"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() { unitchecker.Main(cmap.Analyzer) }
