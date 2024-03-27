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
	"fmt"
	"github.com/rs/zerolog"
	"sync"

	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

var (
	getRequestDataFn = "getRequestData"
	parseResponseFn  = "parseResponse"
)

// jsRecord is an intermediary representation of opencdc.Record that is passed to
// the JavaScript transform. We use this because using opencdc.Record would not
// allow us to modify or access certain data (e.g. metadata or structured data).
type jsRecord struct {
	Position  []byte
	Operation string
	Metadata  map[string]string
	Key       any
	Payload   struct {
		Before any
		After  any
	}
}

// gojaContext represents one independent goja context.
type gojaContext struct {
	runtime          *goja.Runtime
	getRequestDataFn goja.Callable
	parseResponseFn  goja.Callable
}

type jsHelper struct {
	getRequestDataSrc string
	parseResponseSrc  string

	gojaPool sync.Pool
}

func newJSHelper() *jsHelper {
	return &jsHelper{}
}

func (h *jsHelper) Open(ctx context.Context) error {
	runtime, err := h.newRuntime(sdk.Logger(ctx))
	if err != nil {
		return fmt.Errorf("failed initializing JS runtime: %w", err)
	}

	require.
		NewRegistry(require.WithGlobalFolders("/home/haris/node_modules")).
		Enable(runtime)

	_, err = h.newFunction(runtime, h.getRequestDataSrc, getRequestDataFn)
	if err != nil {
		return fmt.Errorf("failed initializing function %q: %w", getRequestDataFn, err)
	}

	_, err = h.newFunction(runtime, h.parseResponseSrc, parseResponseFn)
	if err != nil {
		return fmt.Errorf("failed initializing function %q: %w", parseResponseFn, err)
	}

	h.gojaPool.New = func() any {
		// create a new runtime for the function, so it's executed in a separate goja context
		rt, _ := h.newRuntime(sdk.Logger(ctx))
		getFn, _ := h.newFunction(rt, h.getRequestDataSrc, getRequestDataFn)
		parseFn, _ := h.newFunction(rt, h.getRequestDataSrc, parseResponseFn)
		return &gojaContext{
			runtime:          rt,
			getRequestDataFn: getFn,
			parseResponseFn:  parseFn,
		}
	}

	return nil
}

func (h *jsHelper) getRequestData(ctx context.Context, cfg SourceConfig) (*requestData, error) {
	gojaCtx := h.gojaPool.Get().(*gojaContext)
	defer h.gojaPool.Put(gojaCtx)

	result, err := gojaCtx.getRequestDataFn(goja.Undefined(), gojaCtx.runtime.ToValue(cfg))
	sdk.Logger(ctx).Info().Any("result", result).Msg("got result")
	if err != nil {
		return nil, err
	}

	rd, ok := result.Export().(*requestData)
	if !ok {
		return nil, fmt.Errorf("js function expected to return %T, but returned: %T", &requestData{}, result)
	}

	return rd, nil
}

func (h *jsHelper) newRuntime(logger *zerolog.Logger) (*goja.Runtime, error) {
	rt := goja.New()
	require.NewRegistry().Enable(rt)

	runtimeHelpers := map[string]interface{}{
		"logger":      &logger,
		"RequestData": h.newRequestData(rt),
	}

	for name, helper := range runtimeHelpers {
		if err := rt.Set(name, helper); err != nil {
			return nil, fmt.Errorf("failed to set helper %q: %w", name, err)
		}
	}

	return rt, nil
}

func (h *jsHelper) newFunction(runtime *goja.Runtime, src string, fnName string) (goja.Callable, error) {
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

func (h *jsHelper) newRequestData(runtime *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		// We return a singleRecord struct, however because we are
		// not changing call.This instanceof will not work as expected.

		// JavaScript records are always initialized with metadata
		// so that it's easier to write processor code
		// (without worrying about initializing it every time)
		r := requestData{}
		// We need to return a pointer to make the returned object mutable.
		return runtime.ToValue(&r).ToObject(runtime)
	}
}
