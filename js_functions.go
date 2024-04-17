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
	runtime *goja.Runtime
	fn      goja.Callable
}

func newGojaContext(ctx context.Context, srcPath, fnName string) (*gojaContext, error) {
	sdk.Logger(ctx).Debug().Msgf("check if JS function can be initialized with %v", srcPath)

	// todo wrong context (comes from Open(), should be from Read())
	runtime, err := newRuntime(sdk.Logger(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed initializing JS runtime: %w", err)
	}

	src, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading file %v: %w", srcPath, err)
	}

	fn, err := newFunction(runtime, string(src), fnName)
	if err != nil {
		return nil, fmt.Errorf("failed initializing function %q: %w", fnName, err)
	}

	return &gojaContext{
		runtime: runtime,
		fn:      fn,
	}, nil
}

type jsRequestBuilder struct {
	gojaCtx *gojaContext
	cfg     map[string]string
}

func newJSRequestBuilder(ctx context.Context, cfg map[string]string, srcPath string) (*jsRequestBuilder, error) {
	gojaCtx, err := newGojaContext(ctx, srcPath, getRequestDataFn)
	if err != nil {
		return nil, err
	}

	return &jsRequestBuilder{gojaCtx: gojaCtx, cfg: cfg}, nil
}

func (r *jsRequestBuilder) build(
	previousResponseData map[string]any,
	position sdk.Position,
) (*Request, error) {
	if r.gojaCtx == nil {
		return nil, errors.New("getRequestData function has not been initialized")
	}

	result, err := r.gojaCtx.fn(
		goja.Undefined(),
		r.gojaCtx.runtime.ToValue(r.cfg),
		r.gojaCtx.runtime.ToValue(previousResponseData),
		r.gojaCtx.runtime.ToValue(position),
	)
	if err != nil {
		return nil, err
	}

	rd, ok := result.Export().(*Request)
	if !ok {
		return nil, fmt.Errorf("js function expected to return %T, but returned: %T", &Request{}, result)
	}

	return rd, nil
}

type jsResponseParser struct {
	gojaCtx *gojaContext
}

func (r *jsResponseParser) parse(responseBytes []byte) (*Response, error) {
	if r.gojaCtx == nil {
		return nil, errors.New("parseResponse function has not been initialized")
	}

	result, err := r.gojaCtx.fn(goja.Undefined(), r.gojaCtx.runtime.ToValue(responseBytes))
	if err != nil {
		return nil, err
	}

	rd, ok := result.Export().(*Response)
	if !ok {
		return nil, fmt.Errorf("js function expected to return %T, but returned: %T", &Response{}, result)
	}

	return rd, nil
}

func newJSResponseParser(ctx context.Context, srcPath string) (*jsResponseParser, error) {
	gojaCtx, err := newGojaContext(ctx, srcPath, parseResponseFn)
	if err != nil {
		return nil, err
	}

	return &jsResponseParser{gojaCtx: gojaCtx}, nil
}

func newRuntime(logger *zerolog.Logger) (*goja.Runtime, error) {
	rt := goja.New()
	require.NewRegistry().Enable(rt)
	url.Enable(rt)

	runtimeHelpers := map[string]interface{}{
		"logger":         &logger,
		"Record":         newRecord(rt),
		"RawData":        newRawData(rt),
		"StructuredData": newStructuredData(rt),
		"Request":        newRequestData(rt),
		"Response":       newResponseData(rt),
	}

	for name, helper := range runtimeHelpers {
		if err := rt.Set(name, helper); err != nil {
			return nil, fmt.Errorf("failed to set helper %q: %w", name, err)
		}
	}

	return rt, nil
}

func newFunction(runtime *goja.Runtime, src string, fnName string) (goja.Callable, error) {
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

func newRawData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		var r sdk.RawData
		if len(call.Arguments) > 0 {
			r = sdk.RawData(call.Argument(0).String())
		}
		// We need to return a pointer to make the returned object mutable.
		return runtime.ToValue(r).ToObject(runtime)
	}
}

func newStructuredData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		// TODO accept arguments
		// We return a map[string]interface{} struct, however because we are
		// not changing call.This instanceof will not work as expected.

		r := make(map[string]interface{})
		return runtime.ToValue(r).ToObject(runtime)
	}
}

func newRecord(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
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

func newRequestData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		r := Request{}
		// We need to return a pointer to make the returned object mutable.
		return runtime.ToValue(&r).ToObject(runtime)
	}
}

func newResponseData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		r := Response{
			CustomData: map[string]any{},
		}
		// We need to return a pointer to make the returned object mutable.
		return runtime.ToValue(&r).ToObject(runtime)
	}
}
