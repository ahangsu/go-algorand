// Copyright (C) 2019-2023 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"

	"github.com/algorand/go-algorand/crypto"
	v2 "github.com/algorand/go-algorand/daemon/algod/api/server/v2"
	"github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated/model"
	"github.com/algorand/go-algorand/ledger/simulation"
	"github.com/algorand/go-algorand/util"

	"golang.org/x/exp/maps"
)

// NOTE: this file records debugger needed resources to run DAP service.
//
//	The functionalities here loads TEAL source(map)s from file-system at
//	the start of server booting, and provide correct TEAL source(map)s
//	for execution trace by hash of executed bytecodes.
//
//	Then methods here `flatten` the whole transaction group execution into
//	an execution `tape`, which allows a debugger to walk forward and
//	backward.

/******************************************************************************
 * Following section is the definition of execution tape, which is the        *
 * flattened version of the execution trace in the simulation response.       *
 ******************************************************************************/

// TraceType is an enumeration for byte code exec trace type.
type TraceType uint64

const (
	// LogicSig stands for the logic sig exec trace.
	LogicSig TraceType = iota
	// Approval means execution trace derive from an approval program bytecode.
	Approval
	// ClearState means execution trace derive from a clear state program.
	ClearState
)

// ExecSegment represents a segment of execution trace inside of a single
// program bytecode.  The segment can be located with txnPath into the
// transaction group, transaction trace type, program bytecode hash digest,
// and the beginning pc index to the end of pc index.
type ExecSegment struct {
	TxnPath      simulation.TxnPath
	TraceType    TraceType
	StartPCIndex uint64
	EndPCIndex   uint64
	Digest       crypto.Digest
}

// ExecSegments is the slice of ExecSegments' whose concatenation yields the
// full execution of the transaction group(s).
type ExecSegments []ExecSegment

// selectExecUnits is a helper function used only in preorderTraversal, that
// returns either approval program trace or clear state program trace, assuming
// that either one of the execution traces are available.
func selectExecUnits(
	trace *model.SimulationTransactionExecTrace,
) (*[]model.SimulationOpcodeTraceUnit, crypto.Digest, TraceType) {
	if trace.ApprovalProgramTrace != nil {
		return trace.ApprovalProgramTrace, crypto.Digest(*trace.ApprovalProgramHash), Approval
	}
	return trace.ClearStateProgramTrace, crypto.Digest(*trace.ClearStateProgramHash), ClearState
}

// preorderTraversal is a helper method used only in tapeForGroup, which
// performs a preorder traversal for the transaction group, and appends to the
// tape.ExecSegments by each segment of execution trace in program bytecode.
func (tape ExecTape) preorderTraversal(
	currentPath simulation.TxnPath, trace *model.SimulationTransactionExecTrace,
) {
	if trace.ApprovalProgramTrace == nil &&
		trace.ClearStateProgramTrace == nil &&
		trace.InnerTrace == nil {
		return
	}

	units, digest, traceType := selectExecUnits(trace)

	if trace.InnerTrace == nil {
		tape.ExecSegments = append(tape.ExecSegments, ExecSegment{
			TxnPath:      currentPath,
			TraceType:    traceType,
			Digest:       digest,
			StartPCIndex: 0,
			EndPCIndex:   uint64(len(*units) - 1),
		})
		return
	}

	startPCIndex := 0

	for i, unit := range *units {
		if i == len(*units)-1 {
			tape.ExecSegments = append(tape.ExecSegments, ExecSegment{
				TxnPath:      currentPath,
				TraceType:    traceType,
				Digest:       digest,
				StartPCIndex: uint64(startPCIndex),
				EndPCIndex:   uint64(i),
			})
		}

		if unit.SpawnedInners == nil {
			continue
		}

		tape.ExecSegments = append(tape.ExecSegments, ExecSegment{
			TxnPath:      currentPath,
			TraceType:    traceType,
			Digest:       digest,
			StartPCIndex: uint64(startPCIndex),
			EndPCIndex:   uint64(i),
		})

		for j, innerIndex := range *unit.SpawnedInners {
			nextPath := make(simulation.TxnPath, len(currentPath))
			copy(nextPath, currentPath)
			nextPath = append(nextPath, uint64(j))

			tape.preorderTraversal(nextPath, &((*trace.InnerTrace)[innerIndex]))
		}

		if i != len(*units)-1 {
			startPCIndex = i + 1
		}
	}
}

// tapeForGroup is the method used to construct tape.ExecSegments, which
// contains logic sig execution segments first, then transaction group execution
// append to the tape.ExecSegments.
func (tape ExecTape) tapeForGroup(groupIndex uint64) (ExecTape, error) {
	for i, txn := range tape.SimulationResponse.TxnGroups[groupIndex].Txns {
		if txn.TransactionTrace == nil {
			continue
		}
		if txn.TransactionTrace.LogicSigHash == nil {
			continue
		}
		tape.ExecSegments = append(tape.ExecSegments, ExecSegment{
			TxnPath:      simulation.TxnPath{groupIndex, uint64(i)},
			TraceType:    LogicSig,
			StartPCIndex: 0,
			EndPCIndex:   uint64(len(*txn.TransactionTrace.LogicSigTrace) - 1),
			Digest:       crypto.Digest(*txn.TransactionTrace.LogicSigHash),
		})
	}

	for i, txn := range tape.SimulationResponse.TxnGroups[groupIndex].Txns {
		if txn.TransactionTrace == nil {
			continue
		}
		tape.preorderTraversal(
			simulation.TxnPath{groupIndex, uint64(i)}, txn.TransactionTrace,
		)
	}

	return tape, nil
}

// ExecTape is the struct that records all the debug assets that should be
// read from the file system, including:
//
//   - BytecodeHashToSource, which is loaded from transaction group description
//     file, describing the TEAL source(map)s tied to each hash of executed
//     byte code.
//
//   - SimulationResponse is the deserialized simulation response from the dump
//     of the simulation response on file system.
//
//   - ExecSegments is the slice of ExecSegment, representing the execution
//     throughout the transaction group(s) opcode by opcode.
//
// During debugging transaction group, there will be only 1 single instance of
// ExecTape in server that the automata work on.
type ExecTape struct {
	DebugResources     DebugResources
	SimulationResponse v2.PreEncodedSimulateResponse
	ExecSegments       ExecSegments
	// TODO initial state extracted from simulate response,
	// TODO after initial state PR merged
}

// MakeExecTape loads all the resources of a debugger needed from the
// file system.  This method should be called before server starts serving.
func MakeExecTape(txnJsonPath, simRespPath string) (tape ExecTape, err error) {
	if tape.DebugResources, err =
		MakeDebugResources(txnJsonPath, projectRootAbsPath); err != nil {
		return
	}

	if tape.SimulationResponse, err =
		ReadSimulateResponse(simRespPath, projectRootAbsPath); err != nil {
		return
	}

	return tape.tapeForGroup(0)
}

/******************************************************************************
 * Following section is the definition of RuntimeBreakpoints, which keeps     *
 * track of the breakpoints on TEAL source code level and program bytecode    *
 * level.                                                                     *
 ******************************************************************************/

// RuntimeBreakpoint is the struct indicating the breakpoint over a source line.
type RuntimeBreakpoint struct {
	SrcLine      uint64
	BreakpointID uint64
	Verified     bool
}

// RuntimeBreakpoints gathers all the RuntimeBreakpoint indexed by hash of
// program bytecode.
type RuntimeBreakpoints map[crypto.Digest]util.Set[RuntimeBreakpoint]

// MakeRuntimeBreakPoints constructs an instance of RuntimeBreakpoints.
func MakeRuntimeBreakPoints() RuntimeBreakpoints { return make(RuntimeBreakpoints) }

// BreakpointID is the global breakpoint ID that keep incrementing on new bps.
var BreakpointID int = 1

// allocBreakPointID alloc a new breakpoint ID and increment BreakpointID by 1.
func allocBreakPointID() int {
	newAllocation := BreakpointID
	BreakpointID++
	return newAllocation
}

/******************************************************************************
 * Following section is the definition of TransactionWalker, which walks over *
 * the execution traces throughout the whole transaction group.               *
 * Such instance will keep track of breakpoints and current location in the   *
 * execution (PC index and segment index).                                    *
 ******************************************************************************/

// TransactionWalker is the struct that points into ExecTape with SegmentIndex
// and PCIndex, to represent the current execution progress inside of the
// transaction group.  This struct additionally contains breakpoint locations
// w.r.t. both source file and program bytecode.  Note that during debugging,
// there would be only one instance of TransactionWalker throughout the
// debugging service, for there is only one simulation response file.
type TransactionWalker struct {
	ExecTape

	BreakPoints  RuntimeBreakpoints
	SegmentIndex uint64
	PCIndex      uint64
}

// MakeTransactionWalker constructs a new TransactionWalker instance for debugger.
func MakeTransactionWalker(txnJsonPath, simRespPath string) (TransactionWalker, error) {
	execTape, err := MakeExecTape(txnJsonPath, simRespPath)
	if err != nil {
		return TransactionWalker{}, err
	}
	if len(execTape.ExecSegments) == 0 {
		return TransactionWalker{},
			fmt.Errorf("exec segment construction failed, tape length should > 0")
	}
	return TransactionWalker{
		ExecTape:    execTape,
		BreakPoints: MakeRuntimeBreakPoints(),
		PCIndex:     execTape.ExecSegments[0].StartPCIndex,
	}, nil
}

// Forward is a method that let TransactionWalker walk one step ahead,
// i.e., forward one opcode execution.
// Return false iff reaching the last opcode in the last execution segment.
func (w *TransactionWalker) Forward() bool {
	if w.ExecSegments[w.SegmentIndex].EndPCIndex > w.PCIndex {
		w.PCIndex++
		return true
	} else if len(w.ExecSegments)-1 == int(w.SegmentIndex) {
		return false
	} else {
		w.SegmentIndex++
		w.PCIndex = w.ExecSegments[w.SegmentIndex].StartPCIndex
		return true
	}
}

// Backward is a method that let TransactionWalker walk one step backward,
// i.e., revert one opcode execution.
// Return false iff reaching the first opcode in the first execution segment.
func (w *TransactionWalker) Backward() bool {
	if w.ExecSegments[w.SegmentIndex].StartPCIndex < w.PCIndex {
		w.PCIndex--
		return true
	} else if w.SegmentIndex == 0 {
		return false
	} else {
		w.SegmentIndex--
		w.PCIndex = w.ExecSegments[w.SegmentIndex].EndPCIndex
		return true
	}
}

// currentTrace is a helper function that is used by currentExecUnits, that
// returns the model.SimulationTransactionExecTrace object by TxnPath specified.
func (w *TransactionWalker) currentTrace() *model.SimulationTransactionExecTrace {
	path := w.ExecSegments[w.SegmentIndex].TxnPath
	pathIndex := 0

	txnGroup := w.SimulationResponse.TxnGroups[path[pathIndex]]
	pathIndex++

	trace := txnGroup.Txns[path[pathIndex]].TransactionTrace
	pathIndex++

	for len(path) > pathIndex {
		trace = &(*trace.InnerTrace)[path[pathIndex]]
		pathIndex++
	}

	return trace
}

// currentExecUnits is the helper function that is used returns a slice of
// model.SimulationOpcodeTraceUnit.  The type of the trace is decided by the
// TraceType of the very execution segment.
func (w *TransactionWalker) currentExecUnits() *[]model.SimulationOpcodeTraceUnit {
	trace := w.currentTrace()

	switch w.ExecSegments[w.SegmentIndex].TraceType {
	case Approval:
		return trace.ApprovalProgramTrace
	case ClearState:
		return trace.ClearStateProgramTrace
	case LogicSig:
		return trace.LogicSigTrace
	default:
		return nil
	}
}

// scratchVars computes the scratch vars at this PC for this program in the
// transaction walker instance.
func (w *TransactionWalker) scratchVars() (scratchMap map[uint64]model.AvmValue) {
	scratchMap = make(map[uint64]model.AvmValue)

	execUnits := w.currentExecUnits()
	if execUnits == nil {
		return
	}

	for _, unit := range *execUnits {
		if unit.ScratchChanges == nil {
			continue
		}
		for _, change := range *unit.ScratchChanges {
			scratchMap[change.Slot] = change.NewValue
		}
	}
	return
}

// Stack is a data structure used only for displaying TEAL stack computation.
// The implementation is achieved through slice, while exporting the resulting
// will reverse the slice to put the latest pushed element to the first place.
type Stack[T any] []T

// Push appends a new element to the slice.
func (s Stack[T]) Push(t T) { s = append(s, t) }

// Pop drops the last element from the slice.
func (s Stack[T]) Pop() (r T) {
	r, s = s[len(s)-1], s[:len(s)-1]
	return
}

// PopN pops N elements from the stack.
func (s Stack[T]) PopN(n uint64) {
	for i := uint64(0); i < n; i++ {
		s.Pop()
	}
}

// Export reverse the slice to put the latest pushed element to the first place.
func (s Stack[T]) Export() (res []T) {
	res = make([]T, len(s))
	for i, si := range s {
		res[len(s)-1-i] = si
	}
	return
}

// stackVars computes the stack vars at the PC for this program in the
// transaction walker instance.
func (w *TransactionWalker) stackVars() (stack Stack[model.AvmValue]) {
	execUnits := w.currentExecUnits()
	if execUnits == nil {
		return
	}

	for _, unit := range *execUnits {
		if unit.StackPopCount != nil {
			stack.PopN(*unit.StackPopCount)
		}
		if unit.StackAdditions != nil {
			for _, add := range *unit.StackAdditions {
				stack.Push(add)
			}
		}
	}

	return
}

/******************************************************************************
 * Following section are methods defined for breakpoint (un)set.              *
 ******************************************************************************/

// setBreakpoint sets breakpoint on a line for source code specified by path.
// This method returns newly added breakpoint, and verified breakpoint IDs.
func (w *TransactionWalker) setBreakpoint(
	path string, line uint64,
) (newBreakpoint RuntimeBreakpoint, newlyVerifiedID util.Set[uint64]) {
	absPath := toAbsoluteFilePath(path, projectRootAbsPath)

	digest, ok := w.DebugResources.PathDigestBiMap.GetValue(absPath)
	if !ok {
		return
	}

	if _, ok := w.BreakPoints[digest]; !ok {
		w.BreakPoints[digest] = util.MakeSet[RuntimeBreakpoint]()
	}

	currentBreakPointID := allocBreakPointID()
	w.BreakPoints[digest].Add(RuntimeBreakpoint{
		SrcLine:      line,
		BreakpointID: uint64(currentBreakPointID),
	})

	newlyVerifiedID = w.verifyBreakpoints(path)
	for bp := range w.BreakPoints[digest] {
		if bp.BreakpointID == uint64(currentBreakPointID) {
			newBreakpoint = bp
		}
	}

	return
}

// clearBreakpoints removes all the breakpoints tied to the file.
func (w *TransactionWalker) clearBreakpoints(path string) {
	absPath := toAbsoluteFilePath(path, projectRootAbsPath)
	digest, ok := w.DebugResources.PathDigestBiMap.GetValue(absPath)

	if !ok {
		return
	}

	w.BreakPoints[digest] = util.MakeSet[RuntimeBreakpoint]()
}

// verifyBreakpoints verifies breakpoint locations, ensuring breakpoints will
// not be added over empty lines.
func (w *TransactionWalker) verifyBreakpoints(path string) (newlyVerifiedID util.Set[uint64]) {
	absPath := toAbsoluteFilePath(path, projectRootAbsPath)
	newlyVerifiedID = util.MakeSet[uint64]()

	digest, ok := w.DebugResources.PathDigestBiMap.GetValue(absPath)
	if !ok {
		return
	}

	bpSet, ok := w.BreakPoints[digest]
	if !ok {
		return
	}

	bpSlice := maps.Keys(bpSet)
	sourceAssets, ok := w.DebugResources.DigestToSource[digest]
	if !ok {
		return
	}

	oldBpToNewBp := make(map[RuntimeBreakpoint]RuntimeBreakpoint)

	for _, bp := range bpSlice {
		if bp.Verified {
			continue
		}

		newBp := bp
		for true {
			if len(sourceAssets.Source[bp.SrcLine]) == 0 {
				newBp.SrcLine++
			}
			break
		}

		newBp.Verified = true
		oldBpToNewBp[bp] = newBp
	}

	for oldBp, newBp := range oldBpToNewBp {
		delete(bpSet, oldBp)
		bpSet.Add(newBp)
		newlyVerifiedID.Add(newBp.BreakpointID)
	}

	return
}

// updateCurrentLine checks if walk reversely, then attempts to walk forward or
// backward.
func (w *TransactionWalker) updateCurrentLine(reverse bool) bool {
	if reverse {
		return w.Backward()
	}
	return w.Forward()
}

// continueTilSourceAvailable is a helper function in findNextStop, which keep
// going until the execution segment has a source asset (which means we can
// check sourcemap from PC to a line in source code).
func (w *TransactionWalker) continueTilSourceAvailable(reverse bool) {
	digest := w.ExecSegments[w.SegmentIndex].Digest

	// if available, return early.
	if _, ok := w.DebugResources.DigestToSource[digest]; ok {
		return
	}

	for true {
		var stepResult bool
		if reverse {
			stepResult = w.Backward()
		}
		stepResult = w.Forward()

		digest = w.ExecSegments[w.SegmentIndex].Digest
		_, ok := w.DebugResources.DigestToSource[digest]

		if !stepResult || ok {
			break
		}
	}
}

// findNextStop checks if the current PC is mapping to a non-empty line in the
// source code.  If so, it stops forwarding/backwarding; otherwise, it keeps
// going until the PC hits a breakpoint or a concrete line.
// Return true if stopped on a step or stopped on a breakpoint.
func (w *TransactionWalker) findNextStop(reverse bool, stopOnStep bool) bool {
	for {
		w.continueTilSourceAvailable(reverse)

		digest := w.ExecSegments[w.SegmentIndex].Digest
		sourceAssets := w.DebugResources.DigestToSource[digest]

		pc := (*w.currentExecUnits())[w.PCIndex].Pc
		srcLineNum, srcLineOK := sourceAssets.SourceMap.GetLineForPc(int(pc))
		breakpoints, breakpointsOK := w.BreakPoints[digest]

		if srcLineOK && breakpointsOK {
			for bp := range breakpoints {
				if bp.SrcLine == uint64(srcLineNum) {
					return true
				}
			}
		}

		if srcLineOK {
			if len(sourceAssets.Source[srcLineNum]) > 0 {
				break
			}
		}

		var stepResult bool
		if reverse {
			stepResult = w.Backward()
		}
		stepResult = w.Forward()

		if !stepResult {
			break
		}
	}
	return stopOnStep
}

// step walks forward/backward a step in execution tape.
func (w *TransactionWalker) step(reverse bool) {
	if w.updateCurrentLine(reverse) {
		w.findNextStop(reverse, true)
	}
}

// continueTilBreak continues running until it stops at a breakpoint.
func (w *TransactionWalker) continueTilBreak(reverse bool) {
	for {
		if !w.updateCurrentLine(reverse) {
			break
		}
		if w.findNextStop(reverse, false) {
			break
		}
	}
}

// TODO start
