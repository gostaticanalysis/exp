package main

import (
	"embed"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/gostaticanalysis/analysisutil"
)

//go:embed _gopath
var gopath embed.FS

const src = `package main

import "fmt"

func main() {
	fmt.Println("hello")
}`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		return err
	}

	build.Default.GOROOT = "_gopath"
	build.Default.JoinPath = path.Join
	build.Default.IsAbsPath = path.IsAbs
	build.Default.IsDir = func(p string) bool {
		fmt.Println("IsDir", p)
		fi, err := fs.Stat(gopath, p)
		if err != nil {
			return false
		}
		return fi.IsDir()
	}
	build.Default.HasSubdir = func(root, dir string) (string, bool) {
		root = path.Clean(root)
		dir = path.Clean(dir)

		rel := strings.TrimPrefix(dir, root)
		if rel == dir {
			return dir, false
		}

		return rel, true
	}
	build.Default.ReadDir = func(dir string) ([]fs.FileInfo, error) {
		fmt.Println("ReadDir", dir)
		des, err := fs.ReadDir(gopath, dir)
		if err != nil {
			return nil, err
		}

		fis := make([]fs.FileInfo, len(des))
		for i := range des {
			fi, err := des[i].Info()
			if err != nil {
				return nil, err
			}
			fis[i] = fi
		}
		return fis, nil
	}
	build.Default.OpenFile = func(p string) (io.ReadCloser, error) {
		fmt.Println("OpenFile", p)
		return gopath.Open(p)
	}

	config := &types.Config{
		Importer: importer.ForCompiler(fset, "source", nil),
	}

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Instances:  make(map[*ast.Ident]types.Instance),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
		InitOrder:  make([]*types.Initializer, 0),
	}

	pkg, err := config.Check("main", fset, []*ast.File{f}, info)
	if err != nil {
		return err
	}

	obj := analysisutil.LookupFromImports(pkg.Imports(), "fmt", "Println")
	fmt.Println(obj)

	return nil
}
