// Copyright © 2024 Meroxa, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package http

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
	"github.com/rs/zerolog"
)

var (
	getRequestDataFn = "getRequestData"
	parseResponseFn  = "parseResponse"
)

type Request struct {
	URL string
}

type Response struct {
	CustomData map[string]any
	Records    []*jsRecord
}

// jsRecord is an intermediary representation of sdk.Record that is passed to
// the JavaScript transform. We use this because using sdk.Record would not
// allow us to modify or access certain data (e.g. metadata or structured data).
type jsRecord struct {
	Position  []byte
	Operation string
	Metadata  map[string]string
	Key       any
	Payload   jsPayload
}

type jsPayload struct {
	Before any
	After  any
}

// gojaContext represents one independent goja context.
type gojaContext struct {
	runtime          *goja.Runtime
	getRequestDataFn goja.Callable
	parseResponseFn  goja.Callable
}

type sourceExtension struct {
	// getRequestDataSrc contains the source code of the `getRequestData` function
	getRequestDataSrc string
	// parseResponseSrc contains the source code of the `parseResponse` function
	parseResponseSrc string

	hasGetRequestData    bool
	hasParseResponseData bool

	gojaPool sync.Pool
}

func newSourceExtension() *sourceExtension {
	return &sourceExtension{}
}

func (s *sourceExtension) configure(getRequestDataScript, parseResponseScript string) error {
	if getRequestDataScript != "" {
		getRequestDataSrc, err := os.ReadFile(getRequestDataScript)
		if err != nil {
			return fmt.Errorf("failed reading %v from %v: %w", getRequestDataFn, getRequestDataScript, err)
		}
		s.getRequestDataSrc = string(getRequestDataSrc)
	}

	if parseResponseScript != "" {
		parseResponseSrc, err := os.ReadFile(parseResponseScript)
		if err != nil {
			return fmt.Errorf("failed reading %v from %v: %w", parseResponseFn, parseResponseScript, err)
		}
		s.parseResponseSrc = string(parseResponseSrc)
	}

	return nil
}

func (s *sourceExtension) open(ctx context.Context) error {
	sdk.Logger(ctx).Debug().Msg("check if the runtime and functions can be initialized")
	runtime, err := s.newRuntime(sdk.Logger(ctx))
	if err != nil {
		return fmt.Errorf("failed initializing JS runtime: %w", err)
	}

	getFn, err := s.newFunction(runtime, s.getRequestDataSrc, getRequestDataFn)
	if err != nil {
		return fmt.Errorf("failed initializing function %q: %w", getRequestDataFn, err)
	}
	if getFn != nil {
		s.hasGetRequestData = true
	}

	parseFn, err := s.newFunction(runtime, s.parseResponseSrc, parseResponseFn)
	if err != nil {
		return fmt.Errorf("failed initializing function %q: %w", parseResponseFn, err)
	}
	if parseFn != nil {
		s.hasParseResponseData = true
	}

	sdk.Logger(ctx).Debug().Msg("runtime and functions check: OK")

	s.gojaPool.New = func() any {
		// create a new runtime for the function, so it's executed in a separate goja context
		rt, _ := s.newRuntime(sdk.Logger(ctx))
		getFn, _ := s.newFunction(rt, s.getRequestDataSrc, getRequestDataFn)
		parseFn, _ := s.newFunction(rt, s.parseResponseSrc, parseResponseFn)
		return &gojaContext{
			runtime:          rt,
			getRequestDataFn: getFn,
			parseResponseFn:  parseFn,
		}
	}

	return nil
}

func (s *sourceExtension) getRequestData(
	cfg SourceConfig,
	previousResponseData map[string]any,
	position sdk.Position,
) (*Request, error) {
	gojaCtx := s.gojaPool.Get().(*gojaContext)
	defer s.gojaPool.Put(gojaCtx)

	if gojaCtx.getRequestDataFn == nil {
		return nil, errors.New("getRequestData function has not been initialized")
	}

	fn, err := gojaCtx.getRequestDataFn(
		goja.Undefined(),
		gojaCtx.runtime.ToValue(cfg),
		gojaCtx.runtime.ToValue(previousResponseData),
		gojaCtx.runtime.ToValue(position),
	)
	if err != nil {
		return nil, err
	}

	rd, ok := fn.Export().(*Request)
	if !ok {
		return nil, fmt.Errorf("js function expected to return %T, but returned: %T", &Request{}, fn)
	}

	return rd, nil
}

func (s *sourceExtension) parseResponseData(responseBytes []byte) (*Response, error) {
	gojaCtx := s.gojaPool.Get().(*gojaContext)
	defer s.gojaPool.Put(gojaCtx)

	if gojaCtx.parseResponseFn == nil {
		return nil, errors.New("parseResponse function has not been initialized")
	}

	fn, err := gojaCtx.parseResponseFn(goja.Undefined(), gojaCtx.runtime.ToValue(responseBytes))
	if err != nil {
		return nil, err
	}

	rd, ok := fn.Export().(*Response)
	if !ok {
		return nil, fmt.Errorf("js function expected to return %T, but returned: %T", &Response{}, fn)
	}

	return rd, nil
}

func (s *sourceExtension) newRuntime(logger *zerolog.Logger) (*goja.Runtime, error) {
	rt := goja.New()
	require.NewRegistry().Enable(rt)
	url.Enable(rt)

	runtimeHelpers := map[string]interface{}{
		"logger":         &logger,
		"Record":         s.newRecord(rt),
		"RawData":        s.newRawData(rt),
		"StructuredData": s.newStructuredData(rt),
		"Request":        s.newRequestData(rt),
		"Response":       s.newResponseData(rt),
	}

	for name, helper := range runtimeHelpers {
		if err := rt.Set(name, helper); err != nil {
			return nil, fmt.Errorf("failed to set helper %q: %w", name, err)
		}
	}

	return rt, nil
}

func (s *sourceExtension) newRawData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		var r sdk.RawData
		if len(call.Arguments) > 0 {
			r = sdk.RawData(call.Argument(0).String())
		}
		// We need to return a pointer to make the returned object mutable.
		return runtime.ToValue(r).ToObject(runtime)
	}
}

func (s *sourceExtension) newStructuredData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		// TODO accept arguments
		// We return a map[string]interface{} struct, however because we are
		// not changing call.This instanceof will not work as expected.

		r := make(map[string]interface{})
		return runtime.ToValue(r).ToObject(runtime)
	}
}

func (s *sourceExtension) newRecord(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		// We return a singleRecord struct, however because we are
		// not changing call.This instanceof will not work as expected.

		// JavaScript records are always initialized with metadata
		// so that it's easier to write processor code
		// (without worrying about initializing it every time)
		r := jsRecord{
			Metadata: make(map[string]string),
		}
		// We need to return a pointer to make the returned object mutable.
		return runtime.ToValue(&r).ToObject(runtime)
	}
}

func (s *sourceExtension) newFunction(runtime *goja.Runtime, src string, fnName string) (goja.Callable, error) {
	if src == "" {
		return nil, nil
	}

	prg, err := goja.Compile("", src, false)
	if err != nil {
		return nil, fmt.Errorf("failed to compile script: %w", err)
	}

	_, err = runtime.RunProgram(prg)
	if err != nil {
		return nil, fmt.Errorf("failed to run program: %w", err)
	}

	tmp := runtime.Get(fnName)
	fn, ok := goja.AssertFunction(tmp)
	if !ok {
		return nil, fmt.Errorf("failed to get function %q", fnName)
	}

	return fn, nil
}

func (s *sourceExtension) newRequestData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		r := Request{}
		// We need to return a pointer to make the returned object mutable.
		return runtime.ToValue(&r).ToObject(runtime)
	}
}

func (s *sourceExtension) newResponseData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		r := Response{
			CustomData: map[string]any{},
		}
		// We need to return a pointer to make the returned object mutable.
		return runtime.ToValue(&r).ToObject(runtime)
	}
}
