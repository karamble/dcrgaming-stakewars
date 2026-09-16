// Package analyzer rejects nondeterministic constructs in the simulation.
package analyzer

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const SimPrefix = "github.com/karamble/dcrstakewars/pkg/sim"

func New(prefix string) *analysis.Analyzer {
	return &analysis.Analyzer{Name: "simdet", Doc: "check simulation determinism boundaries", Run: func(pass *analysis.Pass) (any, error) {
		if pass.Pkg.Path() != prefix && !strings.HasPrefix(pass.Pkg.Path(), prefix+"/") {
			return nil, nil
		}
		for _, f := range pass.Files {
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if path == prefix || strings.HasPrefix(path, prefix+"/") {
					continue
				}
				// Small explicit pure stdlib surface. Tests may use testing;
				// production files are checked separately below.
				allowed := path == "encoding/binary" || path == "errors" || path == "math/bits" || path == "sort" || path == "slices" || path == "cmp"
				isTest := strings.HasSuffix(pass.Fset.Position(f.Pos()).Filename, "_test.go")
				if isTest && (path == "testing" || path == "bytes" || path == "encoding/hex") {
					allowed = true
				}
				if !allowed {
					pass.Reportf(imp.Pos(), "import %q is outside the pure simulation boundary", path)
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if n == nil {
					return true
				}
				if e, ok := n.(ast.Expr); ok {
					if typ := pass.TypesInfo.Types[e].Type; typ != nil {
						if b, ok := typ.Underlying().(*types.Basic); ok && b.Info()&(types.IsFloat|types.IsComplex) != 0 {
							pass.Reportf(e.Pos(), "floating-point or complex value in simulation")
						}
					}
				}
				switch v := n.(type) {
				case *ast.GoStmt, *ast.ChanType, *ast.SelectStmt, *ast.SendStmt:
					pass.Reportf(n.Pos(), "concurrency construct in simulation")
				case *ast.RangeStmt:
					if typ := pass.TypesInfo.TypeOf(v.X); typ != nil {
						if _, ok := typ.Underlying().(*types.Map); ok {
							pass.Reportf(v.Pos(), "map iteration in simulation")
						}
					}
				case *ast.StructType:
					for _, field := range v.Fields.List {
						if hasPointer(pass.TypesInfo.TypeOf(field.Type)) {
							pass.Reportf(field.Pos(), "pointer or interface field in simulation state")
						}
					}
				case *ast.CallExpr:
					if id, ok := v.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
						pass.Reportf(v.Pos(), "output in simulation")
					}
				}
				return true
			})
		}
		return nil, nil
	}}
}

func hasPointer(t types.Type) bool {
	if t == nil {
		return false
	}
	switch v := t.Underlying().(type) {
	case *types.Pointer, *types.Interface, *types.Signature, *types.Chan:
		return true
	case *types.Slice:
		return hasPointer(v.Elem())
	case *types.Array:
		return hasPointer(v.Elem())
	case *types.Map:
		return hasPointer(v.Key()) || hasPointer(v.Elem())
	}
	return false
}
