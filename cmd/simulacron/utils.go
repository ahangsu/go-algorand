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
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/algorand/go-algorand/crypto"
	v2 "github.com/algorand/go-algorand/daemon/algod/api/server/v2"
	"github.com/algorand/go-algorand/protocol"

	"github.com/algorand/go-algorand-sdk/logic"
	"github.com/google/go-dap"
)

/******************************************************************************
 * BI-DIRECTIONAL MAP: CONSTRUCTED FOR SEARCHING BETWEEN:                     *
 * - HASH DIGEST OF EXECUTED BYTE CODES                                       *
 * - FILE SYSTEM ABSOLUTE PATH OF TEAL SOURCE CODES FOR THE BYTE CODES        *
 ******************************************************************************/

// BidirectionalMap is defined for mapping between
// - hash digest of executed byte codes
// - file system absolute path of TEAL source codes for the byte codes.
type BidirectionalMap[K, V comparable] struct {
	forward map[K]V
	inverse map[V]K
}

// MakeBidirectionalMap constructs an empty BidirectionalMap instance.
func MakeBidirectionalMap[K, V comparable]() BidirectionalMap[K, V] {
	return BidirectionalMap[K, V]{
		forward: make(map[K]V),
		inverse: make(map[V]K),
	}
}

// Add assigns a key-value pair in the bidirectional map instance.
func (bMap BidirectionalMap[K, V]) Add(k K, v V) {
	bMap.forward[k] = v
	bMap.inverse[v] = k
}

// ContainsKey checks the existence of key in the bidirectional map.
func (bMap BidirectionalMap[K, V]) ContainsKey(k K) bool {
	_, ok := bMap.forward[k]
	return ok
}

// ContainsValue checks the existence of value in the bidirectional map.
func (bMap BidirectionalMap[K, V]) ContainsValue(v V) bool {
	_, ok := bMap.inverse[v]
	return ok
}

// GetValue gets value tied to the key in the bidirectional map.
func (bMap BidirectionalMap[K, V]) GetValue(k K) (V, bool) {
	if !bMap.ContainsKey(k) {
		var defaultV V
		return defaultV, false
	}
	return bMap.forward[k], true
}

// GetKey gets key tied to the value in the bidirectional map.
func (bMap BidirectionalMap[K, V]) GetKey(v V) (K, bool) {
	if !bMap.ContainsValue(v) {
		var defaultK K
		return defaultK, false
	}
	return bMap.inverse[v], true
}

/******************************************************************************
 * FILE SYSTEM RELATED CODE:                                                  *
 * LOAD SIMULATE RESPONSE AND AUXILIARY TXN GROUP SOURCES SPECIFICAITON FILE. *
 * STORE IN DATA STRUCTURE, CONVERT INTO EXECUTION TAPE STRUCT FOR DEBUGGER.  *
 ******************************************************************************/

// toAbsoluteFilePath attempts to convert path to absolute path, with respect to
// rootPath, and the assumption here is that rootPath is an absolute path.
func toAbsoluteFilePath(path, rootPath string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(rootPath, path)
}

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
	Source           []string
	SourceMap        logic.SourceMap
	TEALSourceFSPath string
	TEALSrcMapFSPath string
}

// readSourceLines reads a whole file and returns a slice of its lines.
func readSourceLines(path string) (lines []string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// toTEALCodeAsset converts an instance of TEALSourceUnitJSON into an instance
// of TEALCodeAsset, and checks the existence of sourcemap on file system.
func (jsonUnit TEALSourceUnitJSON) toTEALCodeAsset() (TEALCodeAsset, error) {
	// convert to absolute paths
	srcMapAbsPath := toAbsoluteFilePath(
		jsonUnit.SourceMapPath, projectRootAbsPath,
	)
	srcAbsPath := toAbsoluteFilePath(jsonUnit.SourcePath, projectRootAbsPath)

	// read the TEAL source
	srcLines, err := readSourceLines(srcAbsPath)

	// read the sourcemap
	srcMapBytes, err := os.ReadFile(srcMapAbsPath)
	if err != nil {
		return TEALCodeAsset{}, err
	}

	var sourceMap logic.SourceMap
	if err = json.Unmarshal(srcMapBytes, &sourceMap); err != nil {
		return TEALCodeAsset{}, err
	}

	// return resulting stuff
	return TEALCodeAsset{
		Source:           srcLines,
		SourceMap:        sourceMap,
		TEALSrcMapFSPath: srcMapAbsPath,
		TEALSourceFSPath: srcAbsPath,
	}, nil
}

// DebugResources indices hash digest of executed bytecode to TEALCodeAsset.
// Additionally, it maintains a bidirectional map between hash digest and the
// absolute path to TEAL source code.
type DebugResources struct {
	// PathDigestBiMap is the bidirectional map between hash digest and the
	// absolute pat to the TEAL source code.
	PathDigestBiMap BidirectionalMap[string, crypto.Digest]
	// DigestToSource maps from hash digest to TEALCodeAsset
	DigestToSource map[crypto.Digest]TEALCodeAsset
}

// MakeDebugResources loads transaction group source description file from file
// system by txnJsonPath path to an TxnGroupDescriptionJSON instance, and
// returns DebugResources (and error).
func MakeDebugResources(
	txnJsonPath, rootPath string) (resources DebugResources, err error) {

	txnJsonPath = toAbsoluteFilePath(txnJsonPath, rootPath)
	fileBytes, err := os.ReadFile(txnJsonPath)
	if err != nil {
		return
	}

	var jsonLoadedStruct TxnGroupDescriptionJSON
	if err = json.Unmarshal(fileBytes, &jsonLoadedStruct); err != nil {
		return
	}

	resources = DebugResources{
		PathDigestBiMap: MakeBidirectionalMap[string, crypto.Digest](),
		DigestToSource:  make(map[crypto.Digest]TEALCodeAsset),
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

		if resources.PathDigestBiMap.ContainsValue(digest) {
			return DebugResources{}, fmt.Errorf(
				"duplicated program bytecode hash: %s", digest.String(),
			)
		}

		sourceAsset, err = sourceUnit.toTEALCodeAsset()
		if err != nil {
			return
		}

		resources.DigestToSource[digest] = sourceAsset
		resources.PathDigestBiMap.Add(sourceAsset.TEALSourceFSPath, digest)
	}

	return
}

// ReadSimulateResponse loads simulation response dump from file system.
func ReadSimulateResponse(
	respPath, rootPath string) (resp v2.PreEncodedSimulateResponse, err error) {

	respPath = toAbsoluteFilePath(respPath, rootPath)
	simRespBytes, err := os.ReadFile(respPath)
	if err != nil {
		return
	}

	if err = protocol.DecodeReflect(simRespBytes, &resp); err != nil {
		if err = json.Unmarshal(simRespBytes, &resp); err != nil {
			return
		}
		return
	}
	return
}

/******************************************************************************
 * SERVER RELATED UTIL FUNCTION                                               *
 ******************************************************************************/

// TerminateServer is a non-block write to ServerShut channel to signal other
// goroutines that the server is shutting down.  If the channel is already
// buffered, then we don't attempt to block it, for the signal is already there.
func (d *DebugAdapterServer) TerminateServer() {
	select {
	case d.Config.ServerShut <- struct{}{}:
	default:
	}
}

// OSTerminateHandle catches OS termination signal, and calls OSTerminateHandle
// to non-block write to ServerShut, to trigger shutdown internal to server.
func (d *DebugAdapterServer) OSTerminateHandle() {
	ch := make(chan os.Signal, 1)
	// Block until a signal is received.
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ch:
		d.TerminateServer()
	case <-d.Config.ServerShut:
		// kinda dumb, gets from ServerShut channel and write back to it again.
		d.TerminateServer()
	}
}

func newEvent(event string) *dap.Event {
	return &dap.Event{
		ProtocolMessage: dap.ProtocolMessage{
			Seq:  0,
			Type: "event",
		},
		Event: event,
	}
}

func newResponse(requestSeq int, command string) *dap.Response {
	return &dap.Response{
		ProtocolMessage: dap.ProtocolMessage{
			Seq:  0,
			Type: "response",
		},
		Command:    command,
		RequestSeq: requestSeq,
		Success:    true,
	}
}

func newErrorResponse(requestSeq int, command string, message string) *dap.ErrorResponse {
	er := &dap.ErrorResponse{}
	er.Response = *newResponse(requestSeq, command)
	er.Success = false
	er.Message = "unsupported"
	er.Body.Error.Format = message
	er.Body.Error.Id = 12345
	return er
}
