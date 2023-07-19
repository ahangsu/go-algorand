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
	"os"
	"os/signal"
	"syscall"
)

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
