package gossa

import (
	"fmt"
	"go/types"

	"github.com/bjwbell/ssa"
)

type ssaLabel struct {
	target         *ssa.Block // block identified by this label
	breakTarget    *ssa.Block // block to break to in control flow node identified by this label
	continueTarget *ssa.Block // block to continue to in control flow node identified by this label
	defNode        *Node      // label definition Node
	// Label use Node (OGOTO, OBREAK, OCONTINUE).
	// Used only for error detection and reporting.
	// There might be multiple uses, but we only need to track one.
	useNode  *Node
	reported bool // reported indicates whether an error has already been reported for this label
}

// defined reports whether the label has a definition (OLABEL node).
func (l *ssaLabel) defined() bool { return l.defNode != nil }

// used reports whether the label has a use (OGOTO, OBREAK, or OCONTINUE node).
func (l *ssaLabel) used() bool { return l.useNode != nil }

// ssaExport exports a bunch of compiler services for the ssa backend.
type ssaExport struct {
	log bool
}

func (s *ssaExport) TypeBool() ssa.Type    { return Typ[types.Bool] }
func (s *ssaExport) TypeInt8() ssa.Type    { return Typ[types.Int8] }
func (s *ssaExport) TypeInt16() ssa.Type   { return Typ[types.Int16] }
func (s *ssaExport) TypeInt32() ssa.Type   { return Typ[types.Int32] }
func (s *ssaExport) TypeInt64() ssa.Type   { return Typ[types.Int64] }
func (s *ssaExport) TypeUInt8() ssa.Type   { return Typ[types.Uint8] }
func (s *ssaExport) TypeUInt16() ssa.Type  { return Typ[types.Uint16] }
func (s *ssaExport) TypeUInt32() ssa.Type  { return Typ[types.Uint32] }
func (s *ssaExport) TypeUInt64() ssa.Type  { return Typ[types.Uint64] }
func (s *ssaExport) TypeFloat32() ssa.Type { return Typ[types.Float32] }
func (s *ssaExport) TypeFloat64() ssa.Type { return Typ[types.Float64] }
func (s *ssaExport) TypeInt() ssa.Type     { return Typ[types.Int] }
func (s *ssaExport) TypeUintptr() ssa.Type { return Typ[types.Uintptr] }
func (s *ssaExport) TypeString() ssa.Type  { return Typ[types.String] }
func (s *ssaExport) TypeBytePtr() ssa.Type { return Typ[types.Uint8].PtrTo() }

// StringData returns a symbol (a *Sym wrapped in an interface) which
// is the data component of a global string constant containing s.
func (*ssaExport) StringData(s string) interface{} {
	// TODO
	return nil
}

func (e *ssaExport) Auto(t ssa.Type) ssa.GCNode {
	/*n := temp(t.(*Type))   // Note: adds new auto to Curfn.Func.Dcl list
	e.mustImplement = true // This modifies the input to SSA, so we want to make sure we succeed from here!*/
	//return n
	return nil
}

func (e *ssaExport) CanSSA(t ssa.Type) bool {
	return true //canSSAType(t.(*Type))
}

// Log logs a message from the compiler.
func (e *ssaExport) Logf(msg string, args ...interface{}) {
	// If e was marked as unimplemented, anything could happen. Ignore.
	if e.log {
		fmt.Printf(msg, args...)
	}
}

func Fatalf(format string, args ...interface{}) {
	msg := "internal compiler error: " + format
	fmt.Printf(msg, args)
	fmt.Printf("\n")
	panic("")
}

// Fatal reports a compiler error and exits.
func (e *ssaExport) Fatalf(msg string, args ...interface{}) {
	Fatalf(msg, args...)
}

// Unimplemented reports that the function cannot be compiled.
// It will be removed once SSA work is complete.
func (e *ssaExport) Unimplementedf(msg string, args ...interface{}) {
	Fatalf(msg, args...)
}

// Warnl reports a "warning", which is usually flag-triggered
// logging output for the benefit of tests.
func (e *ssaExport) Warnl(line int, fmt_ string, args ...interface{}) {
	panic("Warnl")
	//Warnl(line, fmt_, args...)
}

func (e *ssaExport) Debug_checknil() bool {
	return false
}