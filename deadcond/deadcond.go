package deadcond

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"

	"github.com/gostaticanalysis/comment/passes/commentmap"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name: "deadcond",
	Doc:  Doc,
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
		buildssa.Analyzer,
		commentmap.Analyzer,
	},
}

const Doc = "deadcond is ..."

type PreCond struct {
	m map[ssa.Value]bool
}

func NewPreCond(parents []*PreCond) *PreCond {
	m := map[ssa.Value]bool{}
	for i := range parents {
		for cnd, val := range parents[i].m {
			// TODO(tenntenn): check duplicated
			m[cnd] = val
		}
	}
	return &PreCond{m: m}
}

func (pc *PreCond) Put(condVal ssa.Value, val bool) {
	pc.m[condVal] = val
}

func (pc *PreCond) Lookup(condVal ssa.Value, val bool) bool {

	if _, ok := pc.m[condVal]; ok {
		return true
	}

	c1 := cond{cond: condVal, val: val}
	for c, v := range pc.m {
		if c1.equal(cond{cond: c, val: v}) {
			return true
		}
	}

	return false
}

type cond struct {
	cond ssa.Value
	val  bool
}

func (c cond) converse() (cond, bool) {
	switch cnd := c.cond.(type) {
	case *ssa.UnOp:
		if cnd.Op == token.NOT {
			return cond{cond: cnd, val: !c.val}, true
		}
	case *ssa.BinOp:
		switch cnd.Op {
		case token.EQL:
			newCnd := *cnd
			newCnd.Op = token.NEQ
			return cond{cond: &newCnd, val: !c.val}, true
		case token.NEQ:
			newCnd := *cnd
			newCnd.Op = token.EQL
			return cond{cond: &newCnd, val: !c.val}, true
		case token.LSS:
			newCnd := *cnd
			newCnd.Op = token.GEQ
			return cond{cond: &newCnd, val: !c.val}, true
		case token.GTR:
			newCnd := *cnd
			newCnd.Op = token.LEQ
			return cond{cond: &newCnd, val: !c.val}, true
		case token.LEQ:
			newCnd := *cnd
			newCnd.Op = token.GTR
			return cond{cond: &newCnd, val: !c.val}, true
		case token.GEQ:
			newCnd := *cnd
			newCnd.Op = token.LSS
			return cond{cond: &newCnd, val: !c.val}, true
		}
	}
	return cond{}, false
}

func (c1 cond) equal(c2 cond) bool {
	if c1 == c2 {
		return true
	}

	switch cond2 := c2.cond.(type) {
	case *ssa.UnOp:
		return c1.equalUnOp(cond2, c2.val)
	case *ssa.BinOp:
		return c1.equalBinOp(cond2, c2.val, true)
	}

	return false
}

func (c1 cond) equalUnOp(c2 *ssa.UnOp, val bool) bool {
	if c2.Op == token.NOT {
		return c1.equal(cond{cond: c2.X, val: val})
	}
	return false
}

func (c1 cond) equalBinOp(c2 *ssa.BinOp, val, converse bool) bool {
	switch cond1 := c1.cond.(type) {
	case *ssa.UnOp:
		cond2 := &cond{cond: c2, val: val}
		return cond2.equal(cond{cond: cond1.X, val: c1.val})
	case *ssa.BinOp:
		if cond1.Op == c2.Op && c1.val == val {
			switch cond1.Op {
			case token.EQL, token.NEQ:
				return (equalValue(cond1.X, c2.X) && equalValue(cond1.Y, c2.Y)) ||
					(equalValue(cond1.X, c2.Y) && equalValue(cond1.Y, c2.X))
			case token.LSS, token.GTR, token.LEQ, token.GEQ:
				return (equalValue(cond1.X, c2.X) && equalValue(cond1.Y, c2.Y))
			}
			return false
		}
		if converse {
			if c1, ok := c1.converse(); ok {
				return c1.equalBinOp(c2, !val, false)
			}
		}
	}
	return false
}

func equalValue(v1, v2 ssa.Value) bool {
	if v1 == v2 {
		return true
	}

	switch v1 := v1.(type) {
	case *ssa.Const:
		switch v2 := v2.(type) {
		case *ssa.Const:
			return v1.Value == v2.Value
		}
	}

	return false
}

func run(pass *analysis.Pass) (interface{}, error) {
	funcs := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA).SrcFuncs
	preconds := map[*ssa.BasicBlock]*PreCond{}

	// For Debug
	//for i := range funcs {
	//	for _, b := range funcs[i].Blocks {
	//		fmt.Println(b)
	//		for _, inst := range b.Instrs {
	//			switch inst := inst.(type) {
	//			case *ssa.UnOp:
	//				fmt.Printf("\t%[1]T\n", inst.X)
	//			default:
	//				fmt.Printf("\t%[1]T %[1]p %[1]v\n", inst)
	//			}
	//		}
	//		fmt.Println()
	//	}
	//}

	for i := range funcs {
		for _, b := range funcs[i].Blocks {
			ifinst := ifInst(b)
			if ifinst == nil {
				continue
			}
			var parents []*PreCond
			for _, p := range b.Preds {
				if pc := preconds[p]; pc != nil {
					parents = append(parents, pc)
				}
			}

			for _, p := range parents {
				for _, val := range []bool{false, true} {
					if p.Lookup(ifinst.Cond, val) {
						pos := ifinst.Cond.Pos()
						f := fileByPos(pass, pos)
						path, _ := astutil.PathEnclosingInterval(f, pos, pos)
						if len(path) != 0 {
							var buf bytes.Buffer
							format.Node(&buf, pass.Fset, path[0])
							pass.Reportf(pos, "Condition %s must be %v", buf.String(), val)
						} else {
							pass.Reportf(pos, "Condition must be %v", val)
						}
						break
					}
				}
			}

			truePrecond := NewPreCond(parents)
			truePrecond.Put(ifinst.Cond, true)
			preconds[b.Succs[0]] = truePrecond

			falsePrecond := NewPreCond(parents)
			falsePrecond.Put(ifinst.Cond, false)
			preconds[b.Succs[1]] = falsePrecond
		}
	}

	return nil, nil
}

func ifInst(b *ssa.BasicBlock) *ssa.If {
	if len(b.Instrs) == 0 {
		return nil
	}

	ifinst, ok := b.Instrs[len(b.Instrs)-1].(*ssa.If)
	if !ok {
		return nil
	}

	return ifinst
}

func fileByPos(pass *analysis.Pass, pos token.Pos) *ast.File {
	for _, f := range pass.Files {
		if f.Pos() <= pos && pos <= f.End() {
			return f
		}
	}
	return nil
}
