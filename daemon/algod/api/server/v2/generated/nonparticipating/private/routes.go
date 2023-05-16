// Package private provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/algorand/oapi-codegen DO NOT EDIT.
package private

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
	// Aborts a catchpoint catchup.
	// (DELETE /v2/catchup/{catchpoint})
	AbortCatchup(ctx echo.Context, catchpoint string) error
	// Starts a catchpoint catchup.
	// (POST /v2/catchup/{catchpoint})
	StartCatchup(ctx echo.Context, catchpoint string) error

	// (POST /v2/shutdown)
	ShutdownNode(ctx echo.Context, params ShutdownNodeParams) error
}

// ServerInterfaceWrapper converts echo contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler ServerInterface
}

// AbortCatchup converts echo context to params.
func (w *ServerInterfaceWrapper) AbortCatchup(ctx echo.Context) error {
	var err error
	// ------------- Path parameter "catchpoint" -------------
	var catchpoint string

	err = runtime.BindStyledParameterWithLocation("simple", false, "catchpoint", runtime.ParamLocationPath, ctx.Param("catchpoint"), &catchpoint)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter catchpoint: %s", err))
	}

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.AbortCatchup(ctx, catchpoint)
	return err
}

// StartCatchup converts echo context to params.
func (w *ServerInterfaceWrapper) StartCatchup(ctx echo.Context) error {
	var err error
	// ------------- Path parameter "catchpoint" -------------
	var catchpoint string

	err = runtime.BindStyledParameterWithLocation("simple", false, "catchpoint", runtime.ParamLocationPath, ctx.Param("catchpoint"), &catchpoint)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter catchpoint: %s", err))
	}

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.StartCatchup(ctx, catchpoint)
	return err
}

// ShutdownNode converts echo context to params.
func (w *ServerInterfaceWrapper) ShutdownNode(ctx echo.Context) error {
	var err error

	ctx.Set(Api_keyScopes, []string{""})

	// Parameter object where we will unmarshal all parameters from the context
	var params ShutdownNodeParams
	// ------------- Optional query parameter "timeout" -------------

	err = runtime.BindQueryParameter("form", true, false, "timeout", ctx.QueryParams(), &params.Timeout)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter timeout: %s", err))
	}

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.ShutdownNode(ctx, params)
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

	router.DELETE(baseURL+"/v2/catchup/:catchpoint", wrapper.AbortCatchup, m...)
	router.POST(baseURL+"/v2/catchup/:catchpoint", wrapper.StartCatchup, m...)
	router.POST(baseURL+"/v2/shutdown", wrapper.ShutdownNode, m...)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
	"sl1v6vXH6/8TAAD//zd/iyam1gAA",
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
