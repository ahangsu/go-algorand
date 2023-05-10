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
	"AAA=",
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
