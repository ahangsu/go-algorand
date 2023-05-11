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

package simulation

import (
	"fmt"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/ledger/ledgercore"
	"github.com/algorand/go-algorand/protocol"
)

// cursorEvalTracer is responsible for maintaining a TxnPath that points to the currently executing
// transaction. The absolutePath() function is used to get this path.
type cursorEvalTracer struct {
	logic.NullEvalTracer

	relativeCursor    []int
	previousInnerTxns []int
}

func (tracer *cursorEvalTracer) BeforeTxnGroup(ep *logic.EvalParams) {
	tracer.relativeCursor = append(tracer.relativeCursor, -1) // will go to 0 in BeforeTxn
}

func (tracer *cursorEvalTracer) BeforeTxn(ep *logic.EvalParams, groupIndex int) {
	top := len(tracer.relativeCursor) - 1
	tracer.relativeCursor[top]++
	tracer.previousInnerTxns = append(tracer.previousInnerTxns, 0)
}

func (tracer *cursorEvalTracer) AfterTxn(ep *logic.EvalParams, groupIndex int, ad transactions.ApplyData, evalError error) {
	tracer.previousInnerTxns = tracer.previousInnerTxns[:len(tracer.previousInnerTxns)-1]
}

func (tracer *cursorEvalTracer) AfterTxnGroup(ep *logic.EvalParams, deltas *ledgercore.StateDelta, evalError error) {
	top := len(tracer.relativeCursor) - 1
	if len(tracer.previousInnerTxns) != 0 {
		tracer.previousInnerTxns[len(tracer.previousInnerTxns)-1] += tracer.relativeCursor[top] + 1
	}
	tracer.relativeCursor = tracer.relativeCursor[:top]
}

func (tracer *cursorEvalTracer) relativeGroupIndex() int {
	top := len(tracer.relativeCursor) - 1
	return tracer.relativeCursor[top]
}

func (tracer *cursorEvalTracer) absolutePath() TxnPath {
	path := make(TxnPath, len(tracer.relativeCursor))
	for i, relativeGroupIndex := range tracer.relativeCursor {
		absoluteIndex := uint64(relativeGroupIndex)
		if i > 0 {
			absoluteIndex += uint64(tracer.previousInnerTxns[i-1])
		}
		path[i] = absoluteIndex
	}
	return path
}

// evalTracer is responsible for populating a Result during a simulation evaluation. It saves
// EvalDelta & inner transaction changes as they happen, so if an error occurs during evaluation, we
// can return a partially-built ApplyData with as much information as possible at the time of the
// error.
type evalTracer struct {
	cursorEvalTracer

	result   *Result
	failedAt TxnPath

	// execTraceStack is used only for PC/Stack/Storage exposure
	execTraceStack []*TransactionTrace
}

func makeEvalTracer(lastRound basics.Round, request Request) *evalTracer {
	result := makeSimulationResult(lastRound, request)
	return &evalTracer{result: &result}
}

func (tracer *evalTracer) handleError(evalError error) {
	if evalError != nil && tracer.failedAt == nil {
		tracer.failedAt = tracer.absolutePath()
	}
}

func (tracer *evalTracer) getApplyDataAtPath(path TxnPath) (*transactions.ApplyData, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("simulator debugger error: path is empty")
	}

	applyDataCursor := &tracer.result.TxnGroups[0].Txns[path[0]].Txn.ApplyData

	for _, index := range path[1:] {
		innerTxns := applyDataCursor.EvalDelta.InnerTxns
		if index >= uint64(len(innerTxns)) {
			return nil, fmt.Errorf("simulator debugger error: index %d out of range with length %d. Full path: %v", index, len(innerTxns), path)
		}
		applyDataCursor = &innerTxns[index].ApplyData
	}

	return applyDataCursor, nil
}

func (tracer *evalTracer) mustGetApplyDataAtPath(path TxnPath) *transactions.ApplyData {
	ad, err := tracer.getApplyDataAtPath(path)
	if err != nil {
		panic(err)
	}
	return ad
}

// Copy the inner transaction group to the ApplyData.EvalDelta.InnerTxns of the calling transaction
func (tracer *evalTracer) populateInnerTransactions(txgroup []transactions.SignedTxnWithAD) {
	applyDataOfCallingTxn := tracer.mustGetApplyDataAtPath(tracer.absolutePath()) // this works because the cursor has not been updated yet by `BeforeTxn`
	applyDataOfCallingTxn.EvalDelta.InnerTxns = append(applyDataOfCallingTxn.EvalDelta.InnerTxns, txgroup...)
}

func (tracer *evalTracer) BeforeTxnGroup(ep *logic.EvalParams) {
	if ep.GetCaller() != nil {
		// If this is an inner txn group, save the txns
		tracer.populateInnerTransactions(ep.TxnGroup)
		tracer.result.TxnGroups[0].AppBudgetAdded += uint64(ep.Proto.MaxAppProgramCost)
	}
	tracer.cursorEvalTracer.BeforeTxnGroup(ep)

	// Currently only supports one (first) txn group
	if ep.PooledApplicationBudget != nil && tracer.result.TxnGroups[0].AppBudgetAdded == 0 {
		tracer.result.TxnGroups[0].AppBudgetAdded = uint64(*ep.PooledApplicationBudget)
	}

	// Override transaction group budget if specified in request, retrieve from tracer.result
	if ep.PooledApplicationBudget != nil {
		tracer.result.TxnGroups[0].AppBudgetAdded += tracer.result.EvalOverrides.ExtraOpcodeBudget
		*ep.PooledApplicationBudget += int(tracer.result.EvalOverrides.ExtraOpcodeBudget)
	}

	// Override runtime related constraints against ep, before entering txn group
	ep.EvalConstants = tracer.result.EvalOverrides.LogicEvalConstants()
}

func (tracer *evalTracer) AfterTxnGroup(ep *logic.EvalParams, deltas *ledgercore.StateDelta, evalError error) {
	tracer.handleError(evalError)
	tracer.cursorEvalTracer.AfterTxnGroup(ep, deltas, evalError)
}

func (tracer *evalTracer) saveApplyData(applyData transactions.ApplyData) {
	applyDataOfCurrentTxn := tracer.mustGetApplyDataAtPath(tracer.absolutePath())
	// Copy everything except the EvalDelta, since that has been kept up-to-date after every op
	evalDelta := applyDataOfCurrentTxn.EvalDelta
	*applyDataOfCurrentTxn = applyData
	applyDataOfCurrentTxn.EvalDelta = evalDelta
}

func (tracer *evalTracer) wouldMakeExecTrace(ep *logic.EvalParams, groupIndex int) bool {
	// If local config is closed for simulation exec trace feature, leave
	if config.GetDefaultLocal().DisableSimulationTraceReturn {
		return false
	}

	// If there is no need of exec trace, leave
	if tracer.result.ExecTraceConfig == NoExecTrace {
		return false
	}

	// if there is the current txn is not logic sig eval or application call, leave
	currentTxn := ep.TxnGroup[groupIndex]
	return currentTxn.Txn.Type == protocol.ApplicationCallTx || !currentTxn.Lsig.Blank()
}

func (tracer *evalTracer) BeforeTxn(ep *logic.EvalParams, groupIndex int) {
	if tracer.wouldMakeExecTrace(ep, groupIndex) {
		// make transaction trace in following section
		currentTxn := ep.TxnGroup[groupIndex]
		traceType := Unknown

		if !currentTxn.Lsig.Blank() {
			traceType = LogicSigTransaction
		} else if currentTxn.Txn.Type == protocol.ApplicationCallTx {
			switch currentTxn.Txn.ApplicationCallTxnFields.OnCompletion {
			case transactions.ClearStateOC:
				traceType = AppCallClearStateTransaction
			default:
				traceType = AppCallApprovalTransaction
			}
		}
		transactionTrace := makeTransactionTrace(traceType)

		var txnTraceStackElem *TransactionTrace

		// The last question is, where should this transaction trace attach to:
		// - if it is a top level transaction, then attach to TxnResult level
		// - if it is an inner transaction, then refer to the stack for latest exec trace, and attach to inner array
		if len(tracer.execTraceStack) == 0 {
			tracer.result.TxnGroups[0].Txns[groupIndex].Trace = &transactionTrace
			txnTraceStackElem = tracer.result.TxnGroups[0].Txns[groupIndex].Trace
		} else {
			lastExecTrace := tracer.execTraceStack[len(tracer.execTraceStack)-1]
			lastExecTrace.InnerTraces = append(lastExecTrace.InnerTraces, transactionTrace)
			txnTraceStackElem = &lastExecTrace.InnerTraces[len(lastExecTrace.InnerTraces)-1]

			innerIndex := uint8(len(lastExecTrace.InnerTraces)) - 1
			stepIndex := uint64(len(lastExecTrace.Trace)) - 1
			if lastExecTrace.StepToInnerMap == nil {
				lastExecTrace.StepToInnerMap = make(map[uint64]uint8)
			}
			lastExecTrace.StepToInnerMap[stepIndex] = innerIndex
		}

		// In both case, we need to add to transaction trace to the stack
		tracer.execTraceStack = append(tracer.execTraceStack, txnTraceStackElem)
	}
	tracer.cursorEvalTracer.BeforeTxn(ep, groupIndex)
}

func (tracer *evalTracer) AfterTxn(ep *logic.EvalParams, groupIndex int, ad transactions.ApplyData, evalError error) {
	tracer.handleError(evalError)
	tracer.saveApplyData(ad)
	// if the current transaction + simulation condition would lead to exec trace making
	// we should clean them up from tracer.execTraceStack.
	if tracer.wouldMakeExecTrace(ep, groupIndex) {
		tracer.execTraceStack = tracer.execTraceStack[:len(tracer.execTraceStack)-1]
	}
	tracer.cursorEvalTracer.AfterTxn(ep, groupIndex, ad, evalError)
}

func (tracer *evalTracer) saveEvalDelta(evalDelta transactions.EvalDelta, appIDToSave basics.AppIndex) {
	applyDataOfCurrentTxn := tracer.mustGetApplyDataAtPath(tracer.absolutePath())
	// Copy everything except the inner transactions, since those have been kept up-to-date when we
	// traced those transactions.
	inners := applyDataOfCurrentTxn.EvalDelta.InnerTxns
	applyDataOfCurrentTxn.EvalDelta = evalDelta
	applyDataOfCurrentTxn.EvalDelta.InnerTxns = inners
}

func (tracer *evalTracer) makeOpcodeTraceUnit(cx *logic.EvalContext) OpcodeTraceUnit {
	return OpcodeTraceUnit{PC: uint64(cx.PC())}
}

func (tracer *evalTracer) BeforeOpcode(cx *logic.EvalContext) {
	if cx.RunMode() != logic.ModeApp {
		// do nothing for LogicSig ops
		return
	}
	groupIndex := tracer.relativeGroupIndex()
	var appIDToSave basics.AppIndex
	if cx.TxnGroup[groupIndex].SignedTxn.Txn.ApplicationID == 0 {
		// app creation
		appIDToSave = cx.AppID()
	}
	defer func() {
		// If local config is closed for simulation exec trace feature, leave
		if config.GetDefaultLocal().DisableSimulationTraceReturn {
			return
		}
		if tracer.result.ExecTraceConfig == NoExecTrace {
			return
		}

		currentTrace := tracer.execTraceStack[len(tracer.execTraceStack)-1]
		currentTrace.Trace = append(currentTrace.Trace, tracer.makeOpcodeTraceUnit(cx))
	}()
	tracer.saveEvalDelta(cx.TxnGroup[groupIndex].EvalDelta, appIDToSave)
}

func (tracer *evalTracer) AfterOpcode(cx *logic.EvalContext, evalError error) {
	if cx.RunMode() != logic.ModeApp {
		// do nothing for LogicSig ops
		return
	}
	tracer.handleError(evalError)
}

func (tracer *evalTracer) AfterProgram(cx *logic.EvalContext, evalError error) {
	if cx.RunMode() != logic.ModeApp {
		// Report cost for LogicSig program and exit
		tracer.result.TxnGroups[0].Txns[cx.GroupIndex()].LogicSigBudgetConsumed = uint64(cx.Cost())
		return
	}

	// Report cost of this program.
	// If it is an inner app call, roll up its cost to the top level transaction.
	tracer.result.TxnGroups[0].Txns[tracer.relativeCursor[0]].AppBudgetConsumed += uint64(cx.Cost())

	tracer.handleError(evalError)
}
