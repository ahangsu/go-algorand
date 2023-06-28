// Package experimental provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/algorand/oapi-codegen DO NOT EDIT.
package experimental

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"

	. "github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated/model"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
)

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// Returns OK if experimental API is enabled.
	// (GET /v2/experimental)
	ExperimentalCheck(ctx echo.Context) error
}

// ServerInterfaceWrapper converts echo contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler ServerInterface
}

// ExperimentalCheck converts echo context to params.
func (w *ServerInterfaceWrapper) ExperimentalCheck(ctx echo.Context) error {
	var err error

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.ExperimentalCheck(ctx)
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

	router.GET(baseURL+"/v2/experimental", wrapper.ExperimentalCheck, m...)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
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
