// Package data provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/algorand/oapi-codegen DO NOT EDIT.
package data

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	. "github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated/model"
	"github.com/algorand/oapi-codegen/pkg/runtime"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
)

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// Removes minimum sync round restriction from the ledger.
	// (DELETE /v2/ledger/sync)
	UnsetSyncRound(ctx echo.Context) error
	// Returns the minimum sync round the ledger is keeping in cache.
	// (GET /v2/ledger/sync)
	GetSyncRound(ctx echo.Context) error
	// Given a round, tells the ledger to keep that round in its cache.
	// (POST /v2/ledger/sync/{round})
	SetSyncRound(ctx echo.Context, round uint64) error
}

// ServerInterfaceWrapper converts echo contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler ServerInterface
}

// UnsetSyncRound converts echo context to params.
func (w *ServerInterfaceWrapper) UnsetSyncRound(ctx echo.Context) error {
	var err error

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.UnsetSyncRound(ctx)
	return err
}

// GetSyncRound converts echo context to params.
func (w *ServerInterfaceWrapper) GetSyncRound(ctx echo.Context) error {
	var err error

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetSyncRound(ctx)
	return err
}

// SetSyncRound converts echo context to params.
func (w *ServerInterfaceWrapper) SetSyncRound(ctx echo.Context) error {
	var err error
	// ------------- Path parameter "round" -------------
	var round uint64

	err = runtime.BindStyledParameterWithLocation("simple", false, "round", runtime.ParamLocationPath, ctx.Param("round"), &round)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter round: %s", err))
	}

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.SetSyncRound(ctx, round)
	return err
}

// This is a simple interface which specifies echo.Route addition functions which
// are present on both echo.Echo and echo.Group, since we want to allow using
// either of them for path registration
type EchoRouter interface {
	CONNECT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	HEAD(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	OPTIONS(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PATCH(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	TRACE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
}

// RegisterHandlers adds each server route to the EchoRouter.
func RegisterHandlers(router EchoRouter, si ServerInterface, m ...echo.MiddlewareFunc) {
	RegisterHandlersWithBaseURL(router, si, "", m...)
}

// Registers handlers, and prepends BaseURL to the paths, so that the paths
// can be served under a prefix.
func RegisterHandlersWithBaseURL(router EchoRouter, si ServerInterface, baseURL string, m ...echo.MiddlewareFunc) {

	wrapper := ServerInterfaceWrapper{
		Handler: si,
	}

	router.DELETE(baseURL+"/v2/ledger/sync", wrapper.UnsetSyncRound, m...)
	router.GET(baseURL+"/v2/ledger/sync", wrapper.GetSyncRound, m...)
	router.POST(baseURL+"/v2/ledger/sync/:round", wrapper.SetSyncRound, m...)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
	"FwAA//80DC6Z8tIAAA==",
}

// GetSwagger returns the content of the embedded swagger specification file
// or error if failed to decode
func decodeSpec() ([]byte, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %s", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}

	return buf.Bytes(), nil
}

var rawSpec = decodeSpecCached()

// a naive cached of a decoded swagger spec
func decodeSpecCached() func() ([]byte, error) {
	data, err := decodeSpec()
	return func() ([]byte, error) {
		return data, err
	}
}

// Constructs a synthetic filesystem for resolving external references when loading openapi specifications.
func PathToRawSpec(pathToFile string) map[string]func() ([]byte, error) {
	var res = make(map[string]func() ([]byte, error))
	if len(pathToFile) > 0 {
		res[pathToFile] = rawSpec
	}

	return res
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	var resolvePath = PathToRawSpec("")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(loader *openapi3.Loader, url *url.URL) ([]byte, error) {
		var pathToFile = url.String()
		pathToFile = path.Clean(pathToFile)
		getSpec, ok := resolvePath[pathToFile]
		if !ok {
			err1 := fmt.Errorf("path not found: %s", pathToFile)
			return nil, err1
		}
		return getSpec()
	}
	var specData []byte
	specData, err = rawSpec()
	if err != nil {
		return
	}
	swagger, err = loader.LoadFromData(specData)
	if err != nil {
		return
	}
	return
}
