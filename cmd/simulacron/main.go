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
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	debuggerPort             uint64
	simulationResultFileName string
	txnGroupRootJsonFile     string
	projectRootAbsPath       string
)

func init() {
	rootCmd.PersistentFlags().Uint64Var(
		&debuggerPort, "port", 54321, "Debugger port to listen to")
	rootCmd.PersistentFlags().StringVarP(
		&simulationResultFileName, "simulation-trace-file", "s", "",
		"Simulate trace file to start debug session")
	rootCmd.PersistentFlags().StringVarP(
		&txnGroupRootJsonFile, "txn-root-file", "t", "",
		"Transaction root level application related specification file")
	rootCmd.PersistentFlags().StringVarP(
		&projectRootAbsPath, "root-path", "r", "",
		"Path to the root of transaction group project")
}

// waitForTermination is a blocking function that waits for either
// a SIGINT (Ctrl-C) or SIGTERM (kill -15) OS signal or for disconnectChan
// to be closed by the server when the client disconnects.
// Note that in headless mode, the debugged process is foregrounded
// (to have control of the tty for debugging interactive programs),
// so SIGINT gets sent to the debuggee and not to delve.
func waitForTermination(disconnectChan chan struct{}) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	if runtime.GOOS == "windows" {
		// On windows Ctrl-C sent to inferior process is delivered
		// as SIGINT to delve. Ignore it instead of stopping the server
		// in order to be able to debug signal handlers.
		go func() {
			for range ch {
			}
		}()
		<-disconnectChan
	} else {
		select {
		case <-ch:
		case <-disconnectChan:
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "tealdap",
	Short: "Algorand TEAL Debugger (supporting Debug Adapter Protocol)",
	Long:  `Debug a ...`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("start debugging")
		log.Println("DAP server pid = ", os.Getpid())

		// NOTE: the current implementation handles only one connection to
		// a single editor: it won't make too much sense to support multiple
		// client connection at the same time on a same port.
		config := &ServerConfig{
			Port:       strconv.FormatUint(debuggerPort, 10),
			ServerShut: make(chan struct{}, 1),
		}
		defer close(config.ServerShut)

		// new a dap server here
		dapServer, err := NewServer(config)
		if err != nil {
			log.Fatalf("debugger server initialization error: %s", err.Error())
		}

		go dapServer.OSTerminateHandle()

		dapServer.DAStartServing()
		defer dapServer.DAStopServing()

		log.Println("DAP server exit")
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}
