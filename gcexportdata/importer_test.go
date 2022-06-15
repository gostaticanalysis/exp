package gcexportdata

import (
	"embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"testing"

	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/analysis"
)

//go:embed testdata/src/*
var testdata embed.FS

var Analyzer = &analysis.Analyzer{
	Name: "test",
	Doc:  "test",
	Run: func(pass *analysis.Pass) (any, error) {
		fmt.Println(pass.Pkg)
		return nil, nil
	},
}

func Test(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, "testdata/src/a", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	//fs.WalkDir(testdata, ".", func(path string, d fs.DirEntry, err error) error {
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	fmt.Println(path)
	//	return nil
	//})

	for name, pkg := range pkgs {
		vendor, err := fs.Sub(testdata, "testdata/src/a/vendor")
		if err != nil {
			t.Fatal(err)
		}
		im := Importer(fset, vendor)
		files := maps.Values(pkg.Files)
		config := &types.Config{
			FakeImportC: true,
			Importer:    im,
		}

		info := &types.Info{
			Types:      make(map[ast.Expr]types.TypeAndValue),
			Instances:  make(map[*ast.Ident]types.Instance),
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Implicits:  make(map[ast.Node]types.Object),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
			Scopes:     make(map[ast.Node]*types.Scope),
		}

		typesPkg, err := config.Check(name, fset, files, info)
		if err != nil {
			t.Fatal("typecheck", err)
		}

		pass := &analysis.Pass{
			Analyzer:   Analyzer,
			Fset:       fset,
			Files:      files,
			Pkg:        typesPkg,
			TypesInfo:  info,
			TypesSizes: types.SizesFor("gc", "amd64"),
			Report: func(d analysis.Diagnostic) {
			},
			ResultOf: make(map[*analysis.Analyzer]any),
		}

		result, err := Analyzer.Run(pass)
		fmt.Println(result, err)
	}
}
