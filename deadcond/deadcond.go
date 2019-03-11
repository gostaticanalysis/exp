package deadcond

import (
	"bytes"
	"fmt"
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
	b        *ssa.BasicBlock
	parents  []*PreCond
	m        map[ssa.Value]bool
	conflict map[ssa.Value]bool
	from     map[ssa.Value]*ssa.BasicBlock
}

func NewPreCond(parents []*PreCond, b *ssa.BasicBlock) *PreCond {
	pc := &PreCond{
		b:        b,
		parents:  parents,
		m:        map[ssa.Value]bool{},
		conflict: map[ssa.Value]bool{},
		from:     map[ssa.Value]*ssa.BasicBlock{},
	}

	for i := range parents {
		for cnd, val := range parents[i].m {
			// skip from me
			if parents[i].from[cnd] == b {
				continue
			}

			switch {
			case pc.Lookup(cnd, !val):
				pc.conflict[cnd] = val
			case pc.hasPhi(cnd):
				// skip
			default:
				pc.m[cnd] = val
			}
		}
	}

	return pc
}

func (pc *PreCond) hasPhi(v ssa.Value) bool {

	switch v := v.(type) {
	case *ssa.Phi, *ssa.Parameter, *ssa.Alloc, *ssa.Field,
		*ssa.FieldAddr, *ssa.IndexAddr, *ssa.Next:
		return true
	case *ssa.UnOp:
		return pc.hasPhi(v.X)
	case *ssa.BinOp:
		return pc.hasPhi(v.X) || pc.hasPhi(v.Y)
	case ssa.CallInstruction:
		if pc.hasPhi(v.Common().Value) {
			return true
		}

		for _, arg := range v.Common().Args {
			if pc.hasPhi(arg) {
				return true
			}
		}
	case *ssa.ChangeType:
		return pc.hasPhi(v.X)
	case *ssa.ChangeInterface:
		return pc.hasPhi(v.X)
	case *ssa.Convert:
		return pc.hasPhi(v.X)
	case *ssa.Extract:
		return pc.hasPhi(v.Tuple)
	case *ssa.Index:
		return pc.hasPhi(v.X)
	case *ssa.Lookup:
		return pc.hasPhi(v.X) || pc.hasPhi(v.Index)
	case *ssa.MakeChan:
		return pc.hasPhi(v.Size)
	case *ssa.MakeClosure:
		if pc.hasPhi(v.Fn) {
			return true
		}
		for _, b := range v.Bindings {
			if pc.hasPhi(b) {
				return true
			}
		}
	case *ssa.MakeInterface:
		return pc.hasPhi(v.X)
	case *ssa.MakeMap:
		return pc.hasPhi(v.Reserve)
	case *ssa.MakeSlice:
		return pc.hasPhi(v.Len) || pc.hasPhi(v.Cap)
	case *ssa.Range:
		return pc.hasPhi(v.X)
	case *ssa.Select:
		for _, s := range v.States {
			if pc.hasPhi(s.Chan) || pc.hasPhi(s.Send) {
				return true
			}
		}
	case *ssa.Slice:
		return pc.hasPhi(v.X) || pc.hasPhi(v.Low) ||
			pc.hasPhi(v.High) || pc.hasPhi(v.Max)
	case *ssa.TypeAssert:
		return pc.hasPhi(v.X)
	case *ssa.Function:
		/*
			fmt.Print(v)
			for _, inst := range v.Blocks[0].Instrs {
				switch inst := inst.(type) {
				case *ssa.Return:
					fmt.Printf("\t %[1]v %[1]T\n", inst.Results[0])
				}
			}
		*/
	}

	return false
}

func (pc *PreCond) Put(condVal ssa.Value, val bool, from *ssa.BasicBlock) (conflicted bool) {

	if pc.hasPhi(condVal) {
		return false
	}

	if pc.conflicted(condVal, val) {
		pc.conflict[condVal] = val
		pc.from[condVal] = from
		return true
	}

	if c := pc.lookup(condVal, !val); c != nil {
		delete(pc.m, c)
		pc.conflict[condVal] = val
		pc.from[condVal] = from
		return true
	}
	pc.m[condVal] = val
	pc.from[condVal] = from
	return false
}

func (pc *PreCond) Lookup(condVal ssa.Value, val bool) bool {
	return pc.lookup(condVal, val) != nil
}

func (pc *PreCond) conflicted(condVal ssa.Value, val bool) bool {
	c1 := cond{cond: condVal, val: val}
	for c, v := range pc.conflict {
		if c1.equal(cond{cond: c, val: !v}) {
			return true
		}
	}
	return false
}

func (pc *PreCond) lookup(condVal ssa.Value, val bool) ssa.Value {

	if _, ok := pc.m[condVal]; ok {
		return condVal
	}

	c1 := cond{cond: condVal, val: val}
	for c, v := range pc.m {
		if c1.equal(cond{cond: c, val: v}) {
			return c
		}
	}

	return nil
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

		if cond1.Op == token.EQL && c1.val == val &&
			(c2.Op == token.LEQ || c2.Op == token.GEQ) {
			return equalValue(cond1.X, c2.X) && equalValue(cond1.Y, c2.Y)
		}

		if cond1.Op == token.GTR && c1.val == val && c2.Op == token.GEQ {
			return equalValue(cond1.X, c2.X) && equalValue(cond1.Y, c2.Y)
		}

		if cond1.Op == token.LSS && c1.val == val && c2.Op == token.LEQ {
			return equalValue(cond1.X, c2.X) && equalValue(cond1.Y, c2.Y)
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

	for i := range funcs {
		for _, b := range funcs[i].Blocks {

			var parents []*PreCond
			for _, p := range b.Preds {
				if pc := preconds[p]; pc != nil {
					parents = append(parents, pc)
				}
			}

			pc := NewPreCond(parents, b)
			for _, p := range b.Preds {
				ifinst := ifInst(p)
				if ifinst == nil {
					continue
				}
				pc.Put(ifinst.Cond, true, p)
			}
			preconds[b] = pc
		}
	}

	for i := range funcs {
		for _, b := range funcs[i].Blocks {
			pc, ok := preconds[b]
			if !ok {
				continue
			}

			ifinst := ifInst(b)
			if ifinst == nil {
				continue
			}

			for _, val := range []bool{false, true} {
				if cnd := pc.lookup(ifinst.Cond, val); cnd != nil {
					//fmt.Println(ifinst.Cond)
					//fmt.Println(cnd)

					pos := ifinst.Cond.Pos()
					f := fileByPos(pass, pos)
					var path []ast.Node
					if f != nil {
						path, _ = astutil.PathEnclosingInterval(f, pos, pos)
					}
					if len(path) != 0 {
						var buf bytes.Buffer
						format.Node(&buf, pass.Fset, path[0])
						pass.Reportf(pos, "Condition %s is always %v", buf.String(), val)
					} else {
						pass.Reportf(pos, "Condition is always %v", val)
					}
					break
				}
			}
		}
	}

	// For Debug
	//debugPrint(funcs, preconds)
	//dotPrint(funcs, preconds)

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

func printValue(v ssa.Value) {
	switch v := v.(type) {
	case *ssa.Phi:
		fmt.Print("Phi ", v, " ")
	case *ssa.UnOp:
		fmt.Print(v.Op, " ")
		printValue(v.X)
	case *ssa.BinOp:
		printValue(v.X)
		fmt.Print(" ", v.Op, " ")
		printValue(v.Y)
	case *ssa.Call:
		printValue(v.Call.Value)
		fmt.Print("(")
		for _, arg := range v.Call.Args {
			printValue(arg)
		}
		fmt.Print(")")
	default:
		fmt.Printf("<%[1]p>%[1]v[%[1]T]", v)
	}
}

func debugPrint(funcs []*ssa.Function, preconds map[*ssa.BasicBlock]*PreCond) {
	for i := range funcs {
		fmt.Println(funcs[i])
		for _, b := range funcs[i].Blocks {
			fmt.Println(b, b.Preds, b.Succs)

			if preconds[b] != nil && len(preconds[b].m) != 0 {
				fmt.Println("=========")
				for cnd := range preconds[b].m {
					fmt.Println(cnd)
					//fmt.Printf("<%p>", cnd)
					//printValue(cnd)
					//fmt.Println()
				}
				fmt.Println("=========")
			}

			for _, inst := range b.Instrs {
				switch inst := inst.(type) {
				case ssa.Value:
					fmt.Print("\t")
					printValue(inst)
					fmt.Printf(" {{%[1]v}}\n", inst)
				default:
					fmt.Printf("\t%[1]T %[1]p %[1]v\n", inst)
				}
			}
			fmt.Println()
		}
	}
}

func dotPrint(funcs []*ssa.Function, preconds map[*ssa.BasicBlock]*PreCond) {
	for i := range funcs {
		fmt.Println(funcs[i])
		for _, b := range funcs[i].Blocks {
			fmt.Printf("B%v[\n", b)
			if preconds[b] != nil && len(preconds[b].m) != 0 {
				fmt.Printf("label=\"B%v\n", b)
				for cnd := range preconds[b].m {
					fmt.Printf("%v\n", cnd)
				}
				fmt.Printf("\"")
			}
			fmt.Printf("];\n")

			for _, s := range b.Succs {
				fmt.Printf("B%v -> B%v;\n", b, s)
			}

			fmt.Println()
		}
	}
}
