package cmap

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = "cmap is ..."

// Analyzer is ...
var Analyzer = &analysis.Analyzer{
	Name: "cmap",
	Doc:  doc,
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {

	for _, file := range pass.Files {
		cmap := ast.NewCommentMap(pass.Fset, file, file.Comments)
		ast.Inspect(file, func(n ast.Node) bool {
			cgs := cmap[n]
			if len(cgs) != 0 {
				cs := make([]string, len(cgs))
				for i, cg := range cgs {
					cs[i] = cg.Text()
				}
				pass.Reportf(n.Pos(), "%T: %s", n, strings.Join(cs, ";"))
			}
			return true
		})
	}

	return nil, nil
}
