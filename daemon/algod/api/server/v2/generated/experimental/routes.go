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
	"Q2Oh9OCs0uISKxm8//j/AgAA//+rPU/A68oAAA==",
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
