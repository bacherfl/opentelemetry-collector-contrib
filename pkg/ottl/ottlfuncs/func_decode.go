package ottlfuncs

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"golang.org/x/text/encoding/ianaindex"
)

type DecodeArguments[K any] struct {
	Target   ottl.Getter[K]
	Encoding string
}

func NewDecodeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Decode", &DecodeArguments[K]{}, createDecodeFunction[K])
}

func createDecodeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DecodeArguments[K])
	if !ok {
		return nil, fmt.Errorf("DecodeFactory args must be of type *DecodeArguments[K]")
	}

	return Decode(args.Target, args.Encoding)
}

func Decode[K any](target ottl.Getter[K], encoding string) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		var stringValue string

		switch v := val.(type) {
		case []byte:
			stringValue = string(v)
		case *string:
			stringValue = *v
		case string:
			stringValue = v
		default:
			return nil, fmt.Errorf("unsupported type provided to Decode function: %T", v)
		}

		switch encoding {
		case "base64":
			// base64 is not in IANA index, so we have to deal with this encoding separately
			decodedBytes, err := base64.StdEncoding.DecodeString(stringValue)
			if err != nil {
				return nil, fmt.Errorf("could not decode: %w", err)
			}
			return string(decodedBytes), nil
		default:
			e, err := ianaindex.IANA.Encoding(encoding)
			if err != nil {
				return nil, fmt.Errorf("could not get encoding for %s: %w", encoding, err)
			}
			if e == nil {
				// for some encodings a nil error and a nil encoding is returned, so we need to double check
				// if the encoding is actually set here
				return nil, fmt.Errorf("no decoder available for encoding: %s", encoding)
			}
			decodedString, err := e.NewDecoder().String(stringValue)
			if err != nil {
				return nil, fmt.Errorf("could not decode: %w", err)
			}
			return decodedString, nil
		}
	}, nil
}
