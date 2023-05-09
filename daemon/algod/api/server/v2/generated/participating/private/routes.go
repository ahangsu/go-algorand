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
	// Return a list of participation keys
	// (GET /v2/participation)
	GetParticipationKeys(ctx echo.Context) error
	// Add a participation key to the node
	// (POST /v2/participation)
	AddParticipationKey(ctx echo.Context) error
	// Delete a given participation key by ID
	// (DELETE /v2/participation/{participation-id})
	DeleteParticipationKeyByID(ctx echo.Context, participationId string) error
	// Get participation key info given a participation ID
	// (GET /v2/participation/{participation-id})
	GetParticipationKeyByID(ctx echo.Context, participationId string) error
	// Append state proof keys to a participation key
	// (POST /v2/participation/{participation-id})
	AppendKeys(ctx echo.Context, participationId string) error
}

// ServerInterfaceWrapper converts echo contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler ServerInterface
}

// GetParticipationKeys converts echo context to params.
func (w *ServerInterfaceWrapper) GetParticipationKeys(ctx echo.Context) error {
	var err error

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetParticipationKeys(ctx)
	return err
}

// AddParticipationKey converts echo context to params.
func (w *ServerInterfaceWrapper) AddParticipationKey(ctx echo.Context) error {
	var err error

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.AddParticipationKey(ctx)
	return err
}

// DeleteParticipationKeyByID converts echo context to params.
func (w *ServerInterfaceWrapper) DeleteParticipationKeyByID(ctx echo.Context) error {
	var err error
	// ------------- Path parameter "participation-id" -------------
	var participationId string

	err = runtime.BindStyledParameterWithLocation("simple", false, "participation-id", runtime.ParamLocationPath, ctx.Param("participation-id"), &participationId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter participation-id: %s", err))
	}

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.DeleteParticipationKeyByID(ctx, participationId)
	return err
}

// GetParticipationKeyByID converts echo context to params.
func (w *ServerInterfaceWrapper) GetParticipationKeyByID(ctx echo.Context) error {
	var err error
	// ------------- Path parameter "participation-id" -------------
	var participationId string

	err = runtime.BindStyledParameterWithLocation("simple", false, "participation-id", runtime.ParamLocationPath, ctx.Param("participation-id"), &participationId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter participation-id: %s", err))
	}

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetParticipationKeyByID(ctx, participationId)
	return err
}

// AppendKeys converts echo context to params.
func (w *ServerInterfaceWrapper) AppendKeys(ctx echo.Context) error {
	var err error
	// ------------- Path parameter "participation-id" -------------
	var participationId string

	err = runtime.BindStyledParameterWithLocation("simple", false, "participation-id", runtime.ParamLocationPath, ctx.Param("participation-id"), &participationId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter participation-id: %s", err))
	}

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.AppendKeys(ctx, participationId)
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

	router.GET(baseURL+"/v2/participation", wrapper.GetParticipationKeys, m...)
	router.POST(baseURL+"/v2/participation", wrapper.AddParticipationKey, m...)
	router.DELETE(baseURL+"/v2/participation/:participation-id", wrapper.DeleteParticipationKeyByID, m...)
	router.GET(baseURL+"/v2/participation/:participation-id", wrapper.GetParticipationKeyByID, m...)
	router.POST(baseURL+"/v2/participation/:participation-id", wrapper.AppendKeys, m...)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
	"TN4AAA==",
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
