package gossa

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"

	"github.com/bjwbell/cmd/obj"
	"github.com/bjwbell/ssa"
)

// type phivar struct {
// 	parent  *ast.AssignStmt
// 	varName *ast.Ident
// 	typ     *ast.Ident
// 	expr    ast.Expr
// }

// type ssaVar struct {
// 	name string
// 	node *ast.AssignStmt
// }

// type fnSSA struct {
// 	phi  []phivar
// 	vars []ssaVar
// 	decl *ast.FuncDecl
// }

// func (fn *fnSSA) initPhi() bool {

// 	ast.Inspect(fn.decl, func(n ast.Node) bool {
// 		assignStmt, ok := n.(*ast.AssignStmt)
// 		if !ok {
// 			return true
// 		}
// 		if len(assignStmt.Lhs) != 1 {
// 			panic("invalid assignment stmt")
// 		}
// 		if len(assignStmt.Lhs) != 2 {
// 			return true
// 		}
// 		if _, ok := assignStmt.Lhs[0].(*ast.Ident); !ok {
// 			return true
// 		}
// 		phiType, ok := assignStmt.Rhs[1].(*ast.Ident)
// 		if !ok {
// 			return true
// 		}
// 		phiExpr := assignStmt.Rhs[0]
// 		phiLit, ok := phiExpr.(*ast.CompositeLit)
// 		if !ok {
// 			return true
// 		}
// 		if phiLit.Type == nil {
// 			return true
// 		}
// 		phiIdent, ok := phiLit.Type.(*ast.Ident)
// 		if !ok {
// 			return true
// 		}
// 		if phiIdent.Name != "phi" {
// 			return true
// 		}
// 		var phi phivar
// 		phi.parent = assignStmt
// 		phi.expr = phiExpr
// 		phi.typ = phiType
// 		phi.varName = assignStmt.Lhs[0].(*ast.Ident)
// 		fn.phi = append(fn.phi, phi)
// 		return true
// 	})

// 	return true
// }

// func (fn *fnSSA) removePhi() bool {
// 	return true
// }

// func (fn *fnSSA) rewriteAssign() bool {
// 	return true
// }

// func (fn *fnSSA) restorePhi() bool {
// 	return true
// }

// ParseSSA parses the function, fn, which must be in ssa form and returns
// the corresponding ssa.Func
func BuildSSA(file, pkgName, fn string) (ssafn *ssa.Func, usessa bool) {
	var conf types.Config
	conf.Importer = importer.Default()
	/*conf.Error = func(err error) {
		fmt.Println("terror:", err)
	}*/
	fset := token.NewFileSet()
	fileAst, err := parser.ParseFile(fset, file, nil, parser.AllErrors)
	fileTok := fset.File(fileAst.Pos())
	var terrors string
	if err != nil {
		fmt.Printf("Error parsing %v, error message: %v\n", file, err)
		terrors += fmt.Sprintf("err: %v\n", err)
		return
	}

	files := []*ast.File{fileAst}
	info := types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := conf.Check(pkgName, fset, files, &info)
	if err != nil {
		if terrors != fmt.Sprintf("err: %v\n", err) {
			fmt.Printf("Type error (%v) message: %v\n", file, err)
			return
		}
	}

	fmt.Println("pkg: ", pkg)
	fmt.Println("pkg.Complete:", pkg.Complete())
	scope := pkg.Scope()
	obj := scope.Lookup(fn)
	if obj == nil {
		fmt.Println("Couldnt lookup function: ", fn)
		return
	}
	function, ok := obj.(*types.Func)
	if !ok {
		fmt.Printf("%v is a %v, not a function\n", fn, obj.Type().String())
	}
	var fnDecl *ast.FuncDecl
	for _, decl := range fileAst.Decls {
		if fdecl, ok := decl.(*ast.FuncDecl); ok {
			if fdecl.Name.Name == fn {
				fnDecl = fdecl
				break
			}
		}
	}
	if fnDecl == nil {
		fmt.Println("couldn't find function: ", fn)
		return
	}
	ssafn, ok = buildSSA(fileTok, fileAst, fnDecl, function, &info)
	return ssafn, ok
}

type Ctx struct {
	file *token.File
	fn   *types.Info
}

type ssaVar interface {
	ssaVarType()
	Name() string
	Class() NodeClass
	String() string
	Typ() ssa.Type
}

type ssaParam struct {
	ssaVar
	v   *types.Var
	ctx Ctx
}

func (p *ssaParam) Name() string {
	return p.v.Name()
}

func (p ssaParam) String() string {
	return fmt.Sprintf("{ssaParam: %v}", p.Name())
}

func (p *ssaParam) Class() NodeClass {
	return PPARAM
}

func (p ssaParam) Typ() ssa.Type {
	return &Type{p.v.Type()}
}

type ssaLocal struct {
	ssaVar
	obj types.Object
	ctx Ctx
}

func (local *ssaLocal) Name() string {
	return local.obj.Name()
}

func (local ssaLocal) String() string {
	return fmt.Sprintf("{ssaLocal: %v}", local.Name())
}

func (local *ssaLocal) Class() NodeClass {
	return PAUTO
}

func (local ssaLocal) Typ() ssa.Type {
	return &Type{local.obj.Type()}
}

func getParameters(ctx Ctx, fn *types.Func) []*ssaParam {
	signature := fn.Type().(*types.Signature)
	if signature.Recv() != nil {
		panic("methods unsupported (only functions are supported)")
	}
	var params []*ssaParam
	for i := 0; i < signature.Params().Len(); i++ {
		param := signature.Params().At(i)
		n := ssaParam{v: param, ctx: ctx}
		params = append(params, &n)
	}
	return params
}

func linenum(f *token.File, p token.Pos) int32 {
	return int32(f.Line(p))
}

func isParam(ctx Ctx, fn *types.Func, obj types.Object) bool {
	params := getParameters(ctx, fn)
	for _, p := range params {
		if p.v.Id() == obj.Id() {
			return true
		}
	}
	return false
}

func getLocalDecls(ctx Ctx, fnDecl *ast.FuncDecl, fn *types.Func) []*ssaLocal {
	scope := fn.Scope()
	names := scope.Names()
	var locals []*ssaLocal
	for i := 0; i < len(names); i++ {
		name := names[i]
		obj := scope.Lookup(name)
		if isParam(ctx, fn, obj) {
			continue
		}
		node := ssaLocal{obj: obj, ctx: ctx}
		locals = append(locals, &node)
	}
	return locals
}

func getVars(ctx Ctx, fnDecl *ast.FuncDecl, fnType *types.Func) []ssaVar {
	var vars []ssaVar
	params := getParameters(ctx, fnType)
	locals := getLocalDecls(ctx, fnDecl, fnType)
	for _, p := range params {
		for _, local := range locals {
			if p.Name() == local.Name() {
				fmt.Printf("p.Name(): %v, local.Name(): %v\n", p.Name(), local.Name())
				panic("param and local with same name")
			}
		}
	}
	for _, p := range params {
		vars = append(vars, p)
	}

	for _, local := range locals {
		vars = append(vars, local)
	}
	return vars
}

func buildSSA(ftok *token.File, f *ast.File, fn *ast.FuncDecl, fnType *types.Func, fnInfo *types.Info) (ssafn *ssa.Func, ok bool) {

	// HACK, hardcoded
	arch := "amd64"

	signature, ok := fnType.Type().(*types.Signature)
	if !ok {
		panic("function type is not types.Signature")
	}
	if signature.Recv() != nil {
		fmt.Println("Methods not supported")
		return nil, false
	}
	if signature.Results().Len() > 1 {
		fmt.Println("Multiple return values not supported")
	}

	var e ssaExport
	var s state
	e.log = true
	link := obj.Link{}
	s.ctx = Ctx{ftok, fnInfo}
	s.fnDecl = fn
	s.fnType = fnType
	s.fnInfo = fnInfo
	s.config = ssa.NewConfig(arch, &e, &link)
	s.f = s.config.NewFunc()
	s.f.Name = fnType.Name()
	//s.f.Entry = s.f.NewBlock(ssa.BlockPlain)

	s.scanBlocks(fn.Body)
	if len(s.blocks) < 1 {
		panic("no blocks found, need at least one block per function")
	}

	s.f.Entry = s.blocks[0].b

	s.startBlock(s.f.Entry)

	// Allocate starting values
	s.labels = map[string]*ssaLabel{}
	s.labeledNodes = map[ast.Node]*ssaLabel{}
	s.startmem = s.entryNewValue0(ssa.OpInitMem, ssa.TypeMem)
	s.sp = s.entryNewValue0(ssa.OpSP, Typ[types.Uintptr]) // TODO: use generic pointer type (unsafe.Pointer?) instead
	s.sb = s.entryNewValue0(ssa.OpSB, Typ[types.Uintptr])

	s.vars = map[ssaVar]*ssa.Value{}
	s.vars[&memVar] = s.startmem

	//s.varsyms = map[*Node]interface{}{}

	// Generate addresses of local declarations
	s.decladdrs = map[ssaVar]*ssa.Value{}
	vars := getVars(s.ctx, fn, fnType)
	for _, v := range vars {
		switch v.Class() {
		case PPARAM:
			//aux := s.lookupSymbol(n, &ssa.ArgSymbol{Typ: n.Type, Node: n})
			//s.decladdrs[n] = s.entryNewValue1A(ssa.OpAddr, Ptrto(n.Type), aux, s.sp)
		case PAUTO | PHEAP:
			// TODO this looks wrong for PAUTO|PHEAP, no vardef, but also no definition
			//aux := s.lookupSymbol(n, &ssa.AutoSymbol{Typ: n.Type, Node: n})
			//s.decladdrs[n] = s.entryNewValue1A(ssa.OpAddr, Ptrto(n.Type), aux, s.sp)
		case PPARAM | PHEAP, PPARAMOUT | PHEAP:
		// This ends up wrong, have to do it at the PARAM node instead.
		case PAUTO, PPARAMOUT:
			// processed at each use, to prevent Addr coming
			// before the decl.
		case PFUNC:
			// local function - already handled by frontend
		default:
			str := ""
			if v.Class()&PHEAP != 0 {
				str = ",heap"
			}
			s.Unimplementedf("local variable with class %s%s unimplemented", v.Class(), str)
		}
	}

	//fnType.Pkg()
	//

	fpVar := types.NewVar(0, fnType.Pkg(), ".fp", Typ[types.Int32].Type)
	nodfp := &ssaParam{v: fpVar, ctx: s.ctx}

	// nodfp is a special argument which is the function's FP.
	aux := &ssa.ArgSymbol{Typ: Typ[types.Uintptr], Node: nodfp}
	s.decladdrs[nodfp] = s.entryNewValue1A(ssa.OpAddr, Typ[types.Uintptr], aux, s.sp)

	//s.body(fn.Body)
	s.processBlocks()

	fmt.Println("f:", f)

	ssa.Compile(s.f)

	return s.f, true
}