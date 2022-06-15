package gcexportdata

import (
	"archive/zip"
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/gcexportdata"
)

//go:generate go run github.com/gostaticanalysis/exp/gcexportdata/cmd/dump

var exportFiles *zip.Reader

func init() {
	var err error
	exportFiles, err = zip.NewReader(bytes.NewReader(exportFilesZIP), int64(len(exportFilesZIP)))
	if err != nil {
		panic("gcexportdata: " + err.Error())
	}
}

func Importer(fset *token.FileSet, vendor fs.FS) types.Importer {
	return &defaultImporter{
		imports: make(map[string]*types.Package),
		fset:    fset,
		vendor:  vendor,
		ctx:     context(vendor),
	}
}

func context(vendor fs.FS) *build.Context {
	ctx := build.Default
	ctx.OpenFile = func(p string) (io.ReadCloser, error) {
		p = filepath.FromSlash(p)
		if strings.HasPrefix(p, "/") {
			p = p[1:]
		}
		return vendor.Open(p)
	}
	ctx.JoinPath = func(elem ...string) string {
		return path.Join(elem...)
	}
	ctx.SplitPathList = func(list string) []string {
		return strings.Split(list, "/")
	}
	ctx.IsAbsPath = func(p string) bool {
		return path.IsAbs(p)
	}
	ctx.IsDir = func(p string) bool {
		fi, err := fs.Stat(vendor, p)
		if err != nil {
			return false
		}
		return fi.IsDir()
	}
	ctx.HasSubdir = func(root, dir string) (rel string, ok bool) {
		root = path.Clean(root)
		dir = path.Clean(dir)
		if strings.HasPrefix(dir, root) {
			return dir[len(root):], true
		}
		return "", false
	}
	ctx.ReadDir = func(dir string) ([]fs.FileInfo, error) {
		des, err := fs.ReadDir(vendor, dir)
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

	return &ctx
}

type defaultImporter struct {
	mu      sync.RWMutex
	imports map[string]*types.Package
	fset    *token.FileSet
	vendor  fs.FS
	ctx     *build.Context
}

func (im *defaultImporter) Import(p string) (*types.Package, error) {

	if p == "unsafe" {
		return types.Unsafe, nil
	}

	im.mu.RLock()
	pkg := im.imports[p]
	im.mu.RUnlock()

	if pkg != nil {
		return pkg, nil
	}

	pkg, err := im.readExportFile(p)
	if pkg != nil {
		return pkg, nil
	}

	pkg, err = im.load(p)
	if err != nil {
		return nil, fmt.Errorf("import %s: %w", p, err)
	}

	if pkg != nil {
		im.mu.Lock()
		im.imports[p] = pkg
		im.mu.Unlock()
		return pkg, nil
	}

	return nil, fmt.Errorf("not found %s", p)
}

func (im *defaultImporter) readExportFile(p string) (*types.Package, error) {
	filename := strings.ReplaceAll(p, "/", "_")
	f, err := exportFiles.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("read export data of %q: %w", p, err)
	}
	defer f.Close()

	im.mu.Lock()
	pkg, err := gcexportdata.Read(f, im.fset, make(map[string]*types.Package), p)
	im.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("read export data of %q: %w", p, err)
	}

	return pkg, nil
}

func (im *defaultImporter) load(p string) (*types.Package, error) {
	pkgs, err := im.parseDir(p, 0)
	if err != nil {
		return nil, err
	}

	for name, files := range pkgs {
		var firstHardErr error
		config := &types.Config{
			IgnoreFuncBodies: true,
			Importer:         im,
			Error: func(err error) {
				if firstHardErr == nil && !err.(types.Error).Soft {
					firstHardErr = err
				}
			},
		}
		
		typesPkg, err := config.Check(name, im.fset, files, nil)
		if err != nil {
			if firstHardErr != nil {
				typesPkg = nil
				err = firstHardErr
			}
			return typesPkg, fmt.Errorf("type-checking package %q failed: %w", name, err)
		}
		if firstHardErr != nil {
			panic("package is not safe yet no error was returned")
		}

		if typesPkg != nil {
			return typesPkg, nil
		}
	}

	return nil, nil
}

func (im *defaultImporter) parseDir(dir string, mode parser.Mode) (map[string][]*ast.File, error) {
	fis, err := im.ctx.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("parse dir: %w", err)
	}

	pkgs := make(map[string][]*ast.File)

	for _, fi := range fis {
		if path.Ext(fi.Name()) != ".go" {
			continue
		}

		match, err := im.ctx.MatchFile(dir, fi.Name())
		if err != nil {
			return nil, err
		}

		if !match {
			continue
		}

		filename := im.ctx.JoinPath(dir, fi.Name())
		file, err := im.parseFile(filename, mode)
		if err != nil {
			return nil, err
		}

		pkgs[file.Name.Name] = append(pkgs[file.Name.Name], file)
	}

	return pkgs, nil
}

func (im *defaultImporter) parseFile(filename string, mode parser.Mode) (*ast.File, error) {
	src, err := im.ctx.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	file, err := parser.ParseFile(im.fset, filename, src, mode)
	if err != nil {
		return nil, err
	}
	return file, nil
}
