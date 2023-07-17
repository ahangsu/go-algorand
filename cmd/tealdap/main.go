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
	"log"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var debuggerPort uint64
var simulationResultFileName string

/* =========================================================================== *
 * DISCUSSION ON PASSING INFO WHEN STARTING DA SERVER (ISSUE #5554)            *
 * =========================================================================== *
 * ============                                                                *
 * # Motivation                                                                *
 * ============                                                                *
 * When we kick start the DA server, we want to feed info to the server:       *
 * 1. Simulation trace file that includes execution trace through txn group.   *
 * 2. (Available) Sourcemaps and TEAL sources that are tied to the executed    *
 *    apps/logic-sigs in this transaction group.                               *
 * 3. Application (app / logic) program byte code:                             *
 *    a. On initial state, i.e., before the txn group starts, the sources      *
 *       need to be reported as part of the "initial state": see #5567         *
 *       NOTE: (3.a) should be covered by (1).                                 *
 *    b. As we execute (simulate) the txn group, apps' / logic-sigs' bytecode  *
 *       would be available.  Consider following scenarios:                    *
 *       i. UpdateApplication txn: the approval and clear state program bytes  *
 *          are both available in the txn, while txn is part of simulation     *
 *          result, and thus new program bytes are always avail in txn group.  *
 *       ii. Logic-sig delegation: logic-sigs' bytes are not available on      *
 *           chain, up until one calls it.  The logic-sig bytes should also be *
 *           available as part of txn, contained in simulation result.         *
 *                                                                             *
 * Thus, these 3 components, grouped into following 2 parts:                   *
 * 1. Program byte code for apps / logic-sigs appeared in the txn group exec.  *
 * 2. Sourcemaps and TEAL sources tied to the sourcemaps for programs in the   *
 *    txn group.                                                               *
 *                                                                             *
 * NOTE: the assumption is that, the trace file (simulation response) should   *
 * always hold *ALL* the program bytes, given that simulation can report the   *
 * initial state of the txn group simulation.                                  *
 *                                                                             *
 * =================================                                           *
 * # Questions (and partial answers)                                           *
 * =================================                                           *
 * This leads to following 2 questions:                                        *
 * 1. How to debug programs without source(map)s being available?              *
 *                                                                             *
 *    We can illustrate it in the following graphs:                            *
 *      +-----------------+     maps to    +----------------------------+      *
 *      | TEAL sources    | -------------> | TEAL program bytecodes     |      *
 *      | TEAL sourcemaps |     maps to    | for apps and logic-sigs    |      *
 *      |                 | -------+       |                            |      *
 *      +-----------------+        |       | (Some bytecode may not     |      *
 *                                 +-----> | have source(map) preimage) |      *
 *                                         +----------------------------+      *
 *                                                                             *
 *    We are *INCLINED* to say it won't matter that much, for we assume user   *
 *    would always have their source file available for their debug purpose.   *
 *                                                                             *
 * 2. How to construct the internal representation for the mapping between     *
 *    (source, sourcemap) -> TEAL bytecode?                                    *
 *    This question should be separated into 2:                                *
 *    a. What kind of internal representation of map                           *
 *       (source, sourcemap) -> TEAL bytecode                                  *
 *       that we should choose?                                                *
 *    b. How to construct the internal representation?                         *
 *    The focus might be on (Q 2.a), for (Q 2.b) follows from (Q 2.a).         *
 *                                                                             *
 * Before we answer (Q 2.a), we want to provide some useful facts:             *
 * 1. During simulation time, txns in txn group are executed sequentially.     *
 * 2. Txn group fails if and only if there exists at least one txn fails.      *
 * 3. If we see following                                                      *
 *    a. TEAL source                                                           *
 *    b. Sourcemap between TEAL source and TEAL bytecode                       *
 *    c. TEAL bytecode                                                         *
 *    as equivalent things, then simulation result (trace file) always contain *
 *    more information than client (editor) specified one, conditioned that    *
 *    the simulation succeed.                                                  *
 *                                                                             *
 * From (Fact 1), we can construct a map: txn-group-location -> TEAL bytecode  *
 * (NOTE: or even better, hash of bytecode (H(bytecode)) -> TEAL bytecode)     *
 * from the given simulation result (trace file).                              *
 *                                                                             *
 * From (Fact 3), we think (or hope) the editor can provide information about  *
 * TEAL sources, or more generally, we want to align these TEAL sources        *
 * against the map txn-group-location -> TEAL bytecode.                        *
 *                                                                             *
 * Thus, for simplicity, we assume the editor provide a map:                   *
 * txn-group-location -> TEAL source                                           *
 * (we assume the locations in the map above are at the root level (no inners) *
 * of the txn group, but generally, it can be TEAL source for any location in  *
 * the txn group.)                                                             *
 *                                                                             *
 * Now we see some light in solving both input format (#5554) and reporting    *
 * program initial state (#5567):                                              *
 *                                                                             *
 * We begin by defining the txn-group-location, consisting of following:       *
 * 1. txn-path, [1, 2, 0]-alike array pointing from root to inner calls.       *
 * 2. app-info: (App-or-lsig, (AppID, OnCompletion)).                          *
 * The thinking is: for a txn-path, there may exist an app call delegated to a *
 * logic sig, and the bytecode for the app call vary by OC and AppID.          *
 *                                                                             *
 * We define TEAL-info as (TEAL src, TEAL srcmap, H(bytecode)) for simplicity. *
 *                                                                             *
 * The solution proceeds as follows:                                           *
 * 1. To report program initial state, we assume the trace file contains a map *
 *    txn-group-location -> TEAL-info and set(TEAL-bytecode).                  *
 *    (TEAL-info contains only H(bytecode) to index into set of bytecodes.)    *
 * 2. On the editor side, we consider using a json format file as input, for   *
 *    the input information is structural.                                     *
 *    We assume the input is a json file, consisting of an array of:           *
 *    {                                                                        *
 *        // for group-index, it can only be level 0 with no inner path        *
 *        group-index: uint64,                                                 *
 *        app-or-lsig: enum,                                                   *
 *        on-completion: OC,                                                   *
 *        app-id: uint64,                                                      *
 *        src-location: string,                                                *
 *        srcmap-location: string,                                             *
 *    }                                                                        *
 *    and we can readly construct map txn-group-location -> TEAL-info.         *
 *    (TEAL-info contains src and srcmap, but no H(bytecode).)                 *
 * 3. Finally, we merge the 2 maps from above (trace file and editor side).    *
 *    Merge the TEAL-info at root level, namely full (src, srcmap, hash),      *
 *    then proceed on tagging (src, srcmap) to TEAL-info with same hashes.     *
 * ========================================================================== */

var txnGroupRootJsonFile string

func init() {
	rootCmd.PersistentFlags().Uint64Var(
		&debuggerPort, "port", 54321, "Debugger port to listen to")
	rootCmd.PersistentFlags().StringVar(
		&simulationResultFileName, "simulation-trace-file", "",
		"Simulate trace file to start debug session")
	rootCmd.PersistentFlags().StringVarP(
		&txnGroupRootJsonFile, "txn-root-file", "t", "",
		"Transaction root level application related specification file")
}

var rootCmd = &cobra.Command{
	Use:   "tealdap",
	Short: "Algorand TEAL Debugger (supporting Debug Adapter Protocol)",
	Long:  `Debug a ...`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("start debugging")
		// TODO haven't start server yet, was thinking of testing:
		// how to bring up the server for testing, and bring down after the test

		if err := server(strconv.FormatUint(debuggerPort, 10)); err != nil {
			log.Fatalf("debug error: %s", err.Error())
		}
		// I suppose once we run `launch`, namely dap.LaunchResponse,
		// the server just run all the way to end (if we let through all the stop points).
	},
}

// TODO should consider a few inputs
// - sourcemap (together with source?)
// - app-id(s) tied to the source map
// - simulation result?

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}
