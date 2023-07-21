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

// This file defines helpers and request handlers for a dummy server
// that accepts DAP requests and responds with dummy or error responses.
// Fake-supported requests:
// - initialize
// - launch
// - setBreakpoints
// - setExceptionBreakpoints
// - configurationDone
// - threads
// - stackTrace
// - scopes
// - variables
// - continue
// - disconnect
// All other requests result in ErrorResponse's.
//
// The server uses the following goroutines:
// - "main" goroutine accepts client connections one by one and
//   handles them serially by reading and decoding incoming requests
//   and dispatching each one to a new goroutine for further
//   processing.
// - per-request goroutines process each request as if
//   letting fake debugger take over. They send events and responses
//   via the sender goroutine.
// - sender goroutine listens for messages to send and
//   writes them to the client connection.
//

package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/go-dap"
)

type Session struct {
	// rw is used to read requests and write events/responses
	rw *bufio.ReadWriter

	// sendQueue is used to capture messages from multiple request
	// processing goroutines while writing them to the client connection
	// from a single goroutine via sendFromQueue. We must keep track of
	// the multiple channel senders with a wait group to make sure we do
	// not close this channel prematurely. Closing this channel will signal
	// the sendFromQueue goroutine that it can exit.
	sendQueue chan dap.Message
	sendWg    sync.WaitGroup

	// stopDebug is used to notify long-running handlers to stop stopDebug.
	processing chan struct{}
}

type DebugAdapterServer struct {
	listener net.Listener
	Config   *ServerConfig

	ConnectionWg *sync.WaitGroup
}

func NewServer(config *ServerConfig) (*DebugAdapterServer, error) {
	listener, err := net.Listen("tcp", ":"+config.Port)
	if err != nil {
		return nil, err
	}
	return &DebugAdapterServer{
		listener:     listener,
		Config:       config,
		ConnectionWg: new(sync.WaitGroup),
	}, nil
}

type DebugAdapterClientServerConnection struct {
	Server        *DebugAdapterServer
	RequestWG     *sync.WaitGroup
	TerminateChan chan struct{}
}

func NewDebugAdapterClientServerConnection(server *DebugAdapterServer) *DebugAdapterClientServerConnection {
	return &DebugAdapterClientServerConnection{
		Server:        server,
		RequestWG:     new(sync.WaitGroup),
		TerminateChan: make(chan struct{}, 1),
	}
}

type RequestHandler struct {
	Connection *DebugAdapterClientServerConnection
	MessageWG  *sync.WaitGroup
	sendQueue  chan dap.Message
}

func MakeRequestHandler(connection *DebugAdapterClientServerConnection) *RequestHandler {
	return &RequestHandler{
		Connection: connection,
		MessageWG:  new(sync.WaitGroup),
		sendQueue:  make(chan dap.Message),
	}
}

func (d *DebugAdapterServer) DAStartServing() {
	go func() {
		for {
			conn, err := d.listener.Accept()
			if err != nil {
				log.Printf("DAP: connection failed by %s", err.Error())
				continue
			}
			defer conn.Close()
			log.Println("Started server at", d.listener.Addr())

			// TODO run the session, should have a wait group
			// to wait for all process stop
			go handleConnection(conn)
		}
	}()
}

func (d *DebugAdapterServer) DAStopServing() {
	log.Println("DAP server is stopping...")
	defer log.Println("DAP server is stopped")

	d.listener.Close()
}

var shutdownChannel chan struct{}

func init() {
	shutdownChannel = make(chan struct{}, 1)
}

// server starts a server that listens on a specified port
// and blocks indefinitely. This server can accept multiple
// client connections at the same time.
func server(port string) error {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Println("Started server at", listener.Addr())

outer:
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Connection failed:", err)
			continue
		}
		log.Println("Accepted connection from", conn.RemoteAddr())
		// Handle multiple client connections concurrently
		go handleConnection(conn)

		select {
		case <-shutdownChannel:
			log.Println("closing shutdown channel")
			break outer
		default:
		}
	}
	close(shutdownChannel)
	return nil
}

// handleConnection handles a connection from a single client.
// It reads and decodes the incoming data and dispatches it
// to per-request processing goroutines. It also launches the
// sender goroutine to send resulting messages over the connection
// back to the client.
func handleConnection(conn net.Conn) {
	debugSession := DebugSession{
		rw:        bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		sendQueue: make(chan dap.Message),
		stopDebug: make(chan struct{}),
		server:    nil,
	}
	go debugSession.sendFromQueue()

	for {
		// TODO should break out when handling terminate request
		err := debugSession.handleRequest()
		// TODO(polina): check for connection vs decoding error?
		if err != nil {
			var protocolError *dap.BaseProtocolError
			if err == io.EOF {
				log.Println("No more data to read:", err)
				break
			} else if errors.As(err, &protocolError) {
				// TODO protocol error, should probably carry over in the loop
			} else {
				// There maybe more messages to process, but
				// we will start with the strict behavior of only accepting
				// expected inputs.
				log.Fatal("Server error: ", err)
			}
		}
	}

	log.Println("Closing connection from", conn.RemoteAddr())
	close(debugSession.stopDebug)
	debugSession.sendWg.Wait()
	close(debugSession.sendQueue)
	conn.Close()
}

func (ds *DebugSession) handleRequest() error {
	log.Println("Reading request...")
	request, err := dap.ReadProtocolMessage(ds.rw.Reader)
	if err != nil {
		return err
	}
	log.Printf("Received request\n\t%#v\n", request)
	ds.sendWg.Add(1)
	go func() {
		ds.dispatchRequest(request)
		ds.sendWg.Done()
	}()
	return nil
}

// dispatchRequest launches a new goroutine to process each request
// and send back events and responses.
func (ds *DebugSession) dispatchRequest(request dap.Message) {
	switch request := request.(type) {
	case *dap.InitializeRequest:
		ds.onInitializeRequest(request)
	case *dap.LaunchRequest:
		ds.onLaunchRequest(request)
	case *dap.AttachRequest:
		ds.onAttachRequest(request)
	case *dap.DisconnectRequest:
		ds.onDisconnectRequest(request)
	case *dap.TerminateRequest:
		ds.onTerminateRequest(request)
	case *dap.RestartRequest:
		ds.onRestartRequest(request)
	case *dap.SetBreakpointsRequest:
		ds.onSetBreakpointsRequest(request)
	case *dap.SetFunctionBreakpointsRequest:
		ds.onSetFunctionBreakpointsRequest(request)
	case *dap.SetExceptionBreakpointsRequest:
		ds.onSetExceptionBreakpointsRequest(request)
	case *dap.ConfigurationDoneRequest:
		ds.onConfigurationDoneRequest(request)
	case *dap.ContinueRequest:
		ds.onContinueRequest(request)
	case *dap.NextRequest:
		ds.onNextRequest(request)
	case *dap.StepInRequest:
		ds.onStepInRequest(request)
	case *dap.StepOutRequest:
		ds.onStepOutRequest(request)
	case *dap.StepBackRequest:
		ds.onStepBackRequest(request)
	case *dap.ReverseContinueRequest:
		ds.onReverseContinueRequest(request)
	case *dap.RestartFrameRequest:
		ds.onRestartFrameRequest(request)
	case *dap.GotoRequest:
		ds.onGotoRequest(request)
	case *dap.PauseRequest:
		ds.onPauseRequest(request)
	case *dap.StackTraceRequest:
		ds.onStackTraceRequest(request)
	case *dap.ScopesRequest:
		ds.onScopesRequest(request)
	case *dap.VariablesRequest:
		ds.onVariablesRequest(request)
	case *dap.SetVariableRequest:
		ds.onSetVariableRequest(request)
	case *dap.SetExpressionRequest:
		ds.onSetExpressionRequest(request)
	case *dap.SourceRequest:
		ds.onSourceRequest(request)
	case *dap.ThreadsRequest:
		ds.onThreadsRequest(request)
	case *dap.TerminateThreadsRequest:
		ds.onTerminateThreadsRequest(request)
	case *dap.EvaluateRequest:
		ds.onEvaluateRequest(request)
	case *dap.StepInTargetsRequest:
		ds.onStepInTargetsRequest(request)
	case *dap.GotoTargetsRequest:
		ds.onGotoTargetsRequest(request)
	case *dap.CompletionsRequest:
		ds.onCompletionsRequest(request)
	case *dap.ExceptionInfoRequest:
		ds.onExceptionInfoRequest(request)
	case *dap.LoadedSourcesRequest:
		ds.onLoadedSourcesRequest(request)
	case *dap.DataBreakpointInfoRequest:
		ds.onDataBreakpointInfoRequest(request)
	case *dap.SetDataBreakpointsRequest:
		ds.onSetDataBreakpointsRequest(request)
	case *dap.ReadMemoryRequest:
		ds.onReadMemoryRequest(request)
	case *dap.DisassembleRequest:
		ds.onDisassembleRequest(request)
	case *dap.CancelRequest:
		ds.onCancelRequest(request)
	case *dap.BreakpointLocationsRequest:
		ds.onBreakpointLocationsRequest(request)
	default:
		log.Fatalf("Unable to process %#v", request)
	}
}

// send lets the sender goroutine know via a channel that there is
// a message to be sent to client. This is called by per-request
// goroutines to send events and responses for each request and
// to notify of events triggered by the fake debugger.
func (ds *DebugSession) send(message dap.Message) { ds.sendQueue <- message }

// sendFromQueue is to be run in a separate goroutine to listen on a
// channel for messages to send back to the client. It will
// return once the channel is closed.
func (ds *DebugSession) sendFromQueue() {
	for message := range ds.sendQueue {
		dap.WriteProtocolMessage(ds.rw.Writer, message)
		log.Printf("Message sent\n\t%#v\n", message)
		ds.rw.Flush()
	}
}

// -----------------------------------------------------------------------
// Very Fake Debugger
//

// The debugging session will keep track of how many breakpoints
// have been set. Once start-up is done (i.e. configurationDone
// request is processed), it will "stop" at each breakpoint one by
// one, and once there are no more, it will trigger a terminated event.
type DebugSession struct {
	// rw is used to read requests and write events/responses
	rw *bufio.ReadWriter

	// sendQueue is used to capture messages from multiple request
	// processing goroutines while writing them to the client connection
	// from a single goroutine via sendFromQueue. We must keep track of
	// the multiple channel senders with a wait group to make sure we do
	// not close this channel prematurely. Closing this channel will signal
	// the sendFromQueue goroutine that it can exit.
	sendQueue chan dap.Message
	sendWg    sync.WaitGroup

	// stopDebug is used to notify long-running handlers to stop processing.
	stopDebug chan struct{}

	// bpSet is a counter of the remaining breakpoints that the debug
	// session is yet to stop at before the program terminates.
	bpSet    int
	bpSetMux sync.Mutex

	server *DebugAdapterServerInterface
}

// doContinue allows fake program execution to continue when the program
// is started or unpaused. It simulates events from the debug session
// by "stopping" on a breakpoint or terminating if there are no more
// breakpoints. Safe to use concurrently.
func (ds *DebugSession) doContinue() {
	var e dap.Message
	ds.bpSetMux.Lock()
	if ds.bpSet == 0 {
		// Pretend that the program is running.
		// The delay will allow for all in-flight responses
		// to be sent before termination.
		time.Sleep(1000 * time.Millisecond)
		e = &dap.TerminatedEvent{
			Event: *newEvent("terminated"),
		}
	} else {
		e = &dap.StoppedEvent{
			Event: *newEvent("stopped"),
			Body:  dap.StoppedEventBody{Reason: "breakpoint", ThreadId: 1, AllThreadsStopped: true},
		}
		ds.bpSet--
	}
	ds.bpSetMux.Unlock()
	ds.send(e)
}

// -----------------------------------------------------------------------
// Request Handlers
//
// Below is a dummy implementation of the request handlers.
// They take no action, but just return dummy responses.
// A real debug adaptor would call the debugger methods here
// and use their results to populate each response.

func (ds *DebugSession) onInitializeRequest(request *dap.InitializeRequest) {
	response := &dap.InitializeResponse{}
	response.Response = *newResponse(request.Seq, request.Command)
	response.Body.SupportsConfigurationDoneRequest = true
	response.Body.SupportsFunctionBreakpoints = false
	response.Body.SupportsConditionalBreakpoints = false
	response.Body.SupportsHitConditionalBreakpoints = false
	response.Body.SupportsEvaluateForHovers = false
	response.Body.ExceptionBreakpointFilters = []dap.ExceptionBreakpointsFilter{}
	// might be true, given that we provide reverse data
	// reverse data includes stuff removed from stack, previous state info
	response.Body.SupportsStepBack = false
	response.Body.SupportsSetVariable = false
	// unless we export frame info of execution
	response.Body.SupportsRestartFrame = false
	response.Body.SupportsGotoTargetsRequest = false
	response.Body.SupportsStepInTargetsRequest = false
	response.Body.SupportsCompletionsRequest = false
	response.Body.CompletionTriggerCharacters = []string{}
	response.Body.SupportsModulesRequest = false
	response.Body.AdditionalModuleColumns = []dap.ColumnDescriptor{}
	response.Body.SupportedChecksumAlgorithms = []dap.ChecksumAlgorithm{}
	// should be true
	response.Body.SupportsRestartRequest = false
	response.Body.SupportsExceptionOptions = false
	response.Body.SupportsValueFormattingOptions = false
	response.Body.SupportsExceptionInfoRequest = false
	response.Body.SupportTerminateDebuggee = false
	response.Body.SupportsDelayedStackTraceLoading = false
	response.Body.SupportsLoadedSourcesRequest = false
	response.Body.SupportsLogPoints = false
	response.Body.SupportsTerminateThreadsRequest = false
	response.Body.SupportsSetExpression = false
	response.Body.SupportsTerminateRequest = false
	response.Body.SupportsDataBreakpoints = false
	response.Body.SupportsReadMemoryRequest = false
	response.Body.SupportsDisassembleRequest = false
	// should be true
	response.Body.SupportsCancelRequest = false
	response.Body.SupportsBreakpointLocationsRequest = false
	// This is a fake set up, so we can start "accepting" configuration
	// requests for setting breakpoints, etc from the client at any time.
	// Notify the client with an 'initialized' event. The client will end
	// the configuration sequence with 'configurationDone' request.
	e := &dap.InitializedEvent{Event: *newEvent("initialized")}
	ds.send(e)
	ds.send(response)
}

func (ds *DebugSession) onLaunchRequest(request *dap.LaunchRequest) {
	// This is where a real debug adaptor would check the soundness of the
	// arguments (e.g. program from launch.json) and then use them to launch the
	// debugger and attach to the program.
	response := &dap.LaunchResponse{}
	response.Response = *newResponse(request.Seq, request.Command)
	ds.send(response)
}

func (ds *DebugSession) onAttachRequest(request *dap.AttachRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "AttachRequest is not yet supported"))
}

func (ds *DebugSession) onDisconnectRequest(request *dap.DisconnectRequest) {
	response := &dap.DisconnectResponse{}
	response.Response = *newResponse(request.Seq, request.Command)
	ds.send(response)
	shutdownChannel <- struct{}{}
}

func (ds *DebugSession) onTerminateRequest(request *dap.TerminateRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "TerminateRequest is not yet supported"))
	shutdownChannel <- struct{}{}
}

func (ds *DebugSession) onRestartRequest(request *dap.RestartRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "RestartRequest is not yet supported"))
}

func (ds *DebugSession) onSetBreakpointsRequest(request *dap.SetBreakpointsRequest) {
	response := &dap.SetBreakpointsResponse{}
	response.Response = *newResponse(request.Seq, request.Command)
	response.Body.Breakpoints = make([]dap.Breakpoint, len(request.Arguments.Breakpoints))
	for i, b := range request.Arguments.Breakpoints {
		response.Body.Breakpoints[i].Line = b.Line
		response.Body.Breakpoints[i].Verified = true
		ds.bpSetMux.Lock()
		ds.bpSet++
		ds.bpSetMux.Unlock()
	}
	ds.send(response)
}

func (ds *DebugSession) onSetFunctionBreakpointsRequest(request *dap.SetFunctionBreakpointsRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "SetFunctionBreakpointsRequest is not yet supported"))
}

func (ds *DebugSession) onSetExceptionBreakpointsRequest(request *dap.SetExceptionBreakpointsRequest) {
	response := &dap.SetExceptionBreakpointsResponse{}
	response.Response = *newResponse(request.Seq, request.Command)
	ds.send(response)
}

func (ds *DebugSession) onConfigurationDoneRequest(request *dap.ConfigurationDoneRequest) {
	// This would be the place to check if the session was configured to
	// stop on entry and if that is the case, to issue a
	// stopped-on-breakpoint event. This being a mock implementation,
	// we "let" the program continue after sending a successful response.
	e := &dap.ThreadEvent{Event: *newEvent("thread"), Body: dap.ThreadEventBody{Reason: "started", ThreadId: 1}}
	ds.send(e)
	response := &dap.ConfigurationDoneResponse{}
	response.Response = *newResponse(request.Seq, request.Command)
	ds.send(response)
	ds.doContinue()
}

func (ds *DebugSession) onContinueRequest(request *dap.ContinueRequest) {
	response := &dap.ContinueResponse{}
	response.Response = *newResponse(request.Seq, request.Command)
	ds.send(response)
	ds.doContinue()
}

func (ds *DebugSession) onNextRequest(request *dap.NextRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "NextRequest is not yet supported"))
}

func (ds *DebugSession) onStepInRequest(request *dap.StepInRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "StepInRequest is not yet supported"))
}

func (ds *DebugSession) onStepOutRequest(request *dap.StepOutRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "StepOutRequest is not yet supported"))
}

func (ds *DebugSession) onStepBackRequest(request *dap.StepBackRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "StepBackRequest is not yet supported"))
}

func (ds *DebugSession) onReverseContinueRequest(request *dap.ReverseContinueRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "ReverseContinueRequest is not yet supported"))
}

func (ds *DebugSession) onRestartFrameRequest(request *dap.RestartFrameRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "RestartFrameRequest is not yet supported"))
}

func (ds *DebugSession) onGotoRequest(request *dap.GotoRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "GotoRequest is not yet supported"))
}

func (ds *DebugSession) onPauseRequest(request *dap.PauseRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "PauseRequest is not yet supported"))
}

func (ds *DebugSession) onStackTraceRequest(request *dap.StackTraceRequest) {
	response := &dap.StackTraceResponse{}
	response.Response = *newResponse(request.Seq, request.Command)
	response.Body = dap.StackTraceResponseBody{
		StackFrames: []dap.StackFrame{
			{
				Id:     1000,
				Source: &dap.Source{Name: "hello.go", Path: "/Users/foo/go/src/hello/hello.go", SourceReference: 0},
				Line:   5,
				Column: 0,
				Name:   "main.main",
			},
		},
		TotalFrames: 1,
	}
	ds.send(response)
}

func (ds *DebugSession) onScopesRequest(request *dap.ScopesRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "ScopeRequest is not yet supported"))
}

func (ds *DebugSession) onVariablesRequest(request *dap.VariablesRequest) {
	select {
	case <-ds.stopDebug:
		return
	// simulate long-running processing to make this handler
	// respond to this request after the next request is received
	case <-time.After(100 * time.Millisecond):
		response := &dap.VariablesResponse{}
		response.Response = *newResponse(request.Seq, request.Command)
		response.Body = dap.VariablesResponseBody{
			Variables: []dap.Variable{{Name: "i", Value: "18434528", EvaluateName: "i", VariablesReference: 0}},
		}
		ds.send(response)
	}
}

func (ds *DebugSession) onSetVariableRequest(request *dap.SetVariableRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "setVariableRequest is not yet supported"))
}

func (ds *DebugSession) onSetExpressionRequest(request *dap.SetExpressionRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "SetExpressionRequest is not yet supported"))
}

func (ds *DebugSession) onSourceRequest(request *dap.SourceRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "SourceRequest is not yet supported"))
}

func (ds *DebugSession) onThreadsRequest(request *dap.ThreadsRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "ThreadRequest is not yet supported"))
}

func (ds *DebugSession) onTerminateThreadsRequest(request *dap.TerminateThreadsRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "TerminateRequest is not yet supported"))
}

func (ds *DebugSession) onEvaluateRequest(request *dap.EvaluateRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "EvaluateRequest is not yet supported"))
}

func (ds *DebugSession) onStepInTargetsRequest(request *dap.StepInTargetsRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "StepInTargetRequest is not yet supported"))
}

func (ds *DebugSession) onGotoTargetsRequest(request *dap.GotoTargetsRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "GotoTargetRequest is not yet supported"))
}

func (ds *DebugSession) onCompletionsRequest(request *dap.CompletionsRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "CompletionRequest is not yet supported"))
}

func (ds *DebugSession) onExceptionInfoRequest(request *dap.ExceptionInfoRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "ExceptionRequest is not yet supported"))
}

func (ds *DebugSession) onLoadedSourcesRequest(request *dap.LoadedSourcesRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "LoadedRequest is not yet supported"))
}

func (ds *DebugSession) onDataBreakpointInfoRequest(request *dap.DataBreakpointInfoRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "DataBreakpointInfoRequest is not yet supported"))
}

func (ds *DebugSession) onSetDataBreakpointsRequest(request *dap.SetDataBreakpointsRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "SetDataBreakpointsRequest is not yet supported"))
}

func (ds *DebugSession) onReadMemoryRequest(request *dap.ReadMemoryRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "ReadMemoryRequest is not yet supported"))
}

func (ds *DebugSession) onDisassembleRequest(request *dap.DisassembleRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "DisassembleRequest is not yet supported"))
}

func (ds *DebugSession) onCancelRequest(request *dap.CancelRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "CancelRequest is not yet supported"))
}

func (ds *DebugSession) onBreakpointLocationsRequest(request *dap.BreakpointLocationsRequest) {
	ds.send(newErrorResponse(request.Seq, request.Command, "BreakpointLocationsRequest is not yet supported"))
}
