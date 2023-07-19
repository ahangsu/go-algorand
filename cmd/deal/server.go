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

/*
 * main-routine
 * |
 * | server.DAStartServing()
 * |     connectionWG sync.WaitGroup
 * |     serverShut chan struct (len 1)
 * |
 * |     for loop spawning goroutines for handling connections <--------------------------+
 * |                                                                                      |
 * |     on conn, err := listener.Accept(); err == nil (if err abort, Accept is blocking) |
 * |                                                                                      |
 * *----> handleConnection(conn)                                                          |
 * |      |     connectionWG.Add(1)                                                       |
 * |      |     defer connectionWG.Done()                                                 |
 * |      |                                                                               |
 * |      |     requestWG sync.WaitGroup                                                  |
 * |      |     termRequest chan struct (len 1)                                           |
 * |      |     defer func() {                                                            |
 * |      |         if len(termRequest) > 0:                                              |
 * |      |             no block write to serverShut                                      |
 * |      |      }                                                                        |
 * |      |                                                                               |
 * |      |     for loop spawning goroutines for handling requests <---+                  |
 * |      |                                                            |                  |
 * |      *----> handleRequest(req)                                    |                  |
 * |      |      |     requestWG.Add(1)                                |                  |
 * |      |      |     defer requestWG.Done()                          |                  |
 * |      |      |                                                     |                  |
 * |      |      |     sendWG sync.WaitGroup                           |                  |
 * |      |      |                                                     |                  |
 * |      |      |     ***HANDLE REQUEST DOES ITS OWN THING ...***     |                  |
 * |      |      |                                                     |                  |
 * |      |      |     if terminate related request:                   |                  |
 * |      |      |         non-block write to termRequest              |                  |
 * |      |      |     sendWG.wait()                                   |                  |
 * |      |      |                                                     |                  |
 * |      x <----* handleRequest goroutine end here                    |                  |
 * |      |                                                            |                  |
 * |      |     if len(serverShut) > 0 or read from conn EOF:          |                  |
 * |      |         requestWG.Wait()                                   |                  |
 * |      |         break the spawning for-loop for handling requests  |                  |
 * |      ? -----------------------------------------------------------+                  |
 * x <----* handleConnection goroutine end here                                           |
 * |                                                                                      |
 * |     if len(serverShut) > 0:                                                          |
 * |         connectionWG.Wait()                                                          |
 * |         break the spawning for-loop for handling connections                         |
 * ? -------------------------------------------------------------------------------------+
 * |
 * o server.DAStopServing(), clean things up
 */

type ServerConfig struct {
	Port       string
	ServerShut chan struct{}
}

type DebugAdapterServerInterface interface {
	DAStartServing()
	DAStopServing()
}
