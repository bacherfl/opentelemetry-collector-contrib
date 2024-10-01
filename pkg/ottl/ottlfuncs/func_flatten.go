// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type FlattenArguments[K any] struct {
	Target ottl.PMapGetter[K]
	Prefix ottl.Optional[string]
	Depth  ottl.Optional[int64]
}

func NewFlattenFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("flatten", &FlattenArguments[K]{}, createFlattenFunction[K])
}

func createFlattenFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*FlattenArguments[K])

	if !ok {
		return nil, fmt.Errorf("FlattenFactory args must be of type *FlattenArguments[K]")
	}

	return flatten(args.Target, args.Prefix, args.Depth)
}

func flatten[K any](target ottl.PMapGetter[K], p ottl.Optional[string], d ottl.Optional[int64]) (ottl.ExprFunc[K], error) {
	depth := int64(math.MaxInt64)
	if !d.IsEmpty() {
		depth = d.Get()
		if depth < 0 {
			return nil, fmt.Errorf("invalid depth for flatten function, %d cannot be negative", depth)
		}
	}

	var prefix string
	if !p.IsEmpty() {
		prefix = p.Get()
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		m, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		result := pcommon.NewMap()
		flattenMap(m, result, prefix, 0, depth)
		result.MoveTo(m)

		return nil, nil
	}, nil
}

func flattenMap(m pcommon.Map, result pcommon.Map, prefix string, currentDepth, maxDepth int64) {
	if len(prefix) > 0 {
		prefix += "."
	}

	m.Range(func(k string, v pcommon.Value) bool {
		switch {
		case v.Type() == pcommon.ValueTypeMap && currentDepth < maxDepth:
			flattenMap(v.Map(), result, prefix+k, currentDepth+1, maxDepth)
		case v.Type() == pcommon.ValueTypeSlice && currentDepth < maxDepth:
			for i := 0; i < v.Slice().Len(); i++ {
				var key string
				if currentDepth == 0 {
					key = fmt.Sprintf("%v.%v", prefix+k, i)
				} else {
					key = fmt.Sprintf("%v.%v", k, i)
				}
				elem := v.Slice().At(i)
				switch {
				case elem.Type() == pcommon.ValueTypeMap:
					flattenMap(elem.Map(), result, key, currentDepth+1, maxDepth)
				case elem.Type() == pcommon.ValueTypeSlice:
					flattenSlice(elem.Slice(), result, key, currentDepth+1, maxDepth)
				default:
					v.Slice().At(i).CopyTo(result.PutEmpty(key))
				}
			}
		default:
			v.CopyTo(result.PutEmpty(prefix + k))
		}
		return true
	})
}

func flattenSlice(s pcommon.Slice, result pcommon.Map, key string, currentDepth, maxDepth int64) {
	for i := 0; i < s.Len(); i++ {
		key := fmt.Sprintf("%v.%v", key, i)
		elem := s.At(i)
		switch {
		case elem.Type() == pcommon.ValueTypeMap:
			flattenMap(elem.Map(), result, key, currentDepth+1, maxDepth)
		case elem.Type() == pcommon.ValueTypeSlice:
			flattenSlice(elem.Slice(), result, key, currentDepth+1, maxDepth)
		default:
			s.At(i).CopyTo(result.PutEmpty(key))
		}
	}
}
