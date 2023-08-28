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
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/go-dap"

	"github.com/algorand/go-algorand/crypto"
	v2 "github.com/algorand/go-algorand/daemon/algod/api/server/v2"
	"github.com/algorand/go-algorand/data/transactions/logic"
)

// NOTE: this file records debugger needed resources to run DAP service.
//       The functionalities here loads TEAL source(map)s from file-system at
//       the start of server booting, and provide correct TEAL source(map)s
//       for execution trace by hash of executed bytecodes.

// TEALSourceUnitJSON is the struct tagged with JSON fields to load the TEAL
// source descriptions from transaction group sources description file.
type TEALSourceUnitJSON struct {
	SourcePath    string `json:"source"`
	SourceMapPath string `json:"sourcemap"`
	BytecodeHash  string `json:"bytecode-hash"`
}

// TxnGroupDescriptionJSON is the struct that deserialize JSON file information
// to a slice of TEALSourceUnitJSON, each tie a unique bytecode hash to certain
// TEAL source(map) on the file system.
type TxnGroupDescriptionJSON struct {
	TxnGroupResources []TEALSourceUnitJSON `json:"txn-group-resources"`
}

// TEALCodeAsset is the struct converted from TEALSourceUnitJSON, which should
// be mapped from a program bytecode hash.  This struct contains source(map)
// and the source(map) names for an executed bytecode.
type TEALCodeAsset struct {
	SourceMap        logic.SourceMap
	TEALSrcMapFSPath string
	TEALSourceFSPath string
}

// toTEALCodeAsset converts an instance of TEALSourceUnitJSON into an instance
// of TEALCodeAsset, and checks the existence of sourcemap on file system.
func (jsonUnit TEALSourceUnitJSON) toTEALCodeAsset() (TEALCodeAsset, error) {
	srcMapAbsPath := toAbsoluteFilePath(
		jsonUnit.SourceMapPath, projectRootAbsPath,
	)
	srcAbsPath := toAbsoluteFilePath(jsonUnit.SourcePath, projectRootAbsPath)

	srcMapBytes, err := os.ReadFile(srcMapAbsPath)
	if err != nil {
		return TEALCodeAsset{}, err
	}

	var sourceMap logic.SourceMap
	if err = json.Unmarshal(srcMapBytes, &sourceMap); err != nil {
		return TEALCodeAsset{}, err
	}

	return TEALCodeAsset{
		SourceMap:        sourceMap,
		TEALSrcMapFSPath: srcMapAbsPath,
		TEALSourceFSPath: srcAbsPath,
	}, nil
}

// HashToSource is a type alias for map from hash of executed program bytecode
// to TEAL source code assets, including sourcemap and file system location.
type HashToSource map[crypto.Digest]TEALCodeAsset

// TEALPathToDigest takes a path to a TEAL source file, converts it into an
// absolute path, and finds the corresponding bytecode hash.
func (txnGSrc HashToSource) TEALPathToDigest(path string) crypto.Digest {
	for k, v := range txnGSrc {
		if toAbsoluteFilePath(path, projectRootAbsPath) == v.TEALSourceFSPath {
			return k
		}
	}
	return crypto.Digest{}
}

// MakeHashToSource loads transaction group source description file from file
// system by txnJsonPath path to an TxnGroupDescriptionJSON instance, and
// returns HashToSource (and error).
func MakeHashToSource(txnJsonPath string) (hashToSource HashToSource, err error) {
	// NOTE: we make the assumption that this txnJsonPath is absolute path.
	txnJsonPath = toAbsoluteFilePath(txnJsonPath, projectRootAbsPath)
	fileBytes, err := os.ReadFile(txnJsonPath)
	if err != nil {
		return
	}

	var jsonLoadedStruct TxnGroupDescriptionJSON
	if err = json.Unmarshal(fileBytes, &jsonLoadedStruct); err != nil {
		return
	}

	var (
		digest      crypto.Digest
		sourceAsset TEALCodeAsset
	)
	for _, sourceUnit := range jsonLoadedStruct.TxnGroupResources {
		digest, err = crypto.DigestFromString(sourceUnit.BytecodeHash)
		if err != nil {
			return
		}

		if _, ok := hashToSource[digest]; ok {
			return HashToSource{}, fmt.Errorf(
				"duplicated program bytecode hash: %s", digest.String(),
			)
		}

		sourceAsset, err = sourceUnit.toTEALCodeAsset()
		if err != nil {
			return
		}
		hashToSource[digest] = sourceAsset
	}

	return
}

// TxnGroupIterState is an enumeration for transaction group iteration state.
type TxnGroupIterState uint64

const (
	// TxnGroupIterBeforeStart stands for the idle state of iteration before
	// walking through the transaction group.
	TxnGroupIterBeforeStart TxnGroupIterState = iota
	// TxnGroupIterLSig means debugger is walking through logic sig executions
	// in the transaction group.
	TxnGroupIterLSig
	// TxnGroupIterTransaction suggests debugger is traversing transactions in
	// the transaction group.
	TxnGroupIterTransaction
)

// TxnPathUnit is a unit in TxnPath, which is a slice (stack) of TxnPathUnit's.
// A TxnPathUnit contains following elements:
//
//   - OrderIndex is the index of transaction in transaction group, which may be
//     an inner transaction group.
//
//   - PC is the program counter of executed bytecode.
//
// which allows debug server to walk back and forth in the transaction group.
type TxnPathUnit struct {
	OrderIndex uint64
	PC         uint64
}

// TxnPath is a "transaction path":
// e.g. [OrderIndex=0, OrderIndex=0, OrderIndex=1]
// means the 2nd inner txn of the 1st inner txn of the 1st txn.
type TxnPath []TxnPathUnit

// SrcBreakpointLocations stores breakpoint locations w.r.t., TEAL sources,
// indexed by hash of bytecode hashes, for one bytecode hash is only tied to
// only one TEAL source, and vice versa.
type SrcBreakpointLocations map[crypto.Digest][]dap.BreakpointLocation

// PCBreakpointLocations stores PCs over each program bytecode from breakpoint
// locations over source codes.
type PCBreakpointLocations map[crypto.Digest][]uint64

// DebuggerState is the struct that records all the debug assets that should be
// read from the file system, including:
//
//   - BytecodeHashToSource, which is loaded from transaction group description
//     file, describing the TEAL source(map)s tied to each hash of executed
//     byte code.
//
//   - SimulationResponse is the deserialized simulation response from the dump
//     of the simulation response on file system.
//
//   - TxnGroupTraverseState is an enumeration of transaction group iteration
//     state, which first iterate through all logic signature executions,
//     then traverse through transaction group execution.
//
//   - TransactionPath is the path points the debugger to the transaction that
//     it should look at while debugging, together with each PC in the program
//     bytecodes.
//
//   - BreakpointsOverSources keeps track of breakpoints tied to each available
//     TEAL sources, and eventually we map them to PCs in execution traces.
//
//   - BreakpointsOverPCs keeps track of breakpoints over program bytecodes.
//
// During debugging transaction group, there will be only 1 single instance of
// DebuggerState in server that the automata work on.
type DebuggerState struct {
	BytecodeHashToSource   HashToSource
	SimulationResponse     v2.PreEncodedSimulateResponse
	TxnGroupTraverseState  TxnGroupIterState
	TransactionPath        TxnPath
	BreakpointsOverSources SrcBreakpointLocations
	BreakpointsOverPCs     PCBreakpointLocations
	// TODO initial state extracted from simulate response,
	// TODO after initial state PR merged
}

// LoadSimulateRespDump loads simulation response dump from file system
func LoadSimulateRespDump(simRespPath string) (v2.PreEncodedSimulateResponse, error) {
	simRespPath = toAbsoluteFilePath(simRespPath, projectRootAbsPath)
	simRespBytes, err := os.ReadFile(simRespPath)
	if err != nil {
		return v2.PreEncodedSimulateResponse{}, err
	}

	var simResp v2.PreEncodedSimulateResponse
	if err = json.Unmarshal(simRespBytes, &simResp); err != nil {
		return v2.PreEncodedSimulateResponse{}, err
	}
	return simResp, nil
}

// MakeDebuggerState loads all the resources of a debugger needed from the
// file system.  This method should be called before server starts serving.
func MakeDebuggerState(txnJsonPath, simRespPath string) (DebuggerState, error) {
	hashToSources, err := MakeHashToSource(txnJsonPath)
	if err != nil {
		return DebuggerState{}, err
	}

	simResp, err := LoadSimulateRespDump(simRespPath)
	if err != nil {
		return DebuggerState{}, err
	}

	return DebuggerState{
		BytecodeHashToSource: hashToSources,
		SimulationResponse:   simResp,
	}, nil
}

// Forward is a method that updates debugger state by one step ahead,
// i.e., forward one opcode execution.
// Return true if hitting a breakpoint.
func (state *DebuggerState) Forward() (hitBreakpoint bool) {
	// TODO update DebuggerState by state machine
	return
}

// Backward is a method that updates debugger state by one step backward,
// i.e., revert one opcode execution.
// Return true if hitting a breakpoint.
func (state *DebuggerState) Backward() (hitBreakpoint bool) {
	// TODO update DebuggerState by state machine
	return
}
