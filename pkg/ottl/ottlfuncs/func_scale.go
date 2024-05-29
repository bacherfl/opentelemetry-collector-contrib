package ottlfuncs

import (
	"context"
	"errors"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type ScaleArguments[K any] struct {
	Value      ottl.Getter[K]
	Multiplier float64
}

func NewScaleFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Scale", &ScaleArguments[K]{}, createScaleFunction[K])
}

func createScaleFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ScaleArguments[K])

	if !ok {
		return nil, fmt.Errorf("ScaleFactory args must be of type *ScaleArguments[K]")
	}

	return Scale(args.Value, args.Multiplier)
}

func Scale[K any](value ottl.Getter[K], multiplier float64) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		get, err := value.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch get.(type) {
		case float64:
			return get.(float64) * multiplier, nil
		case int64:
			return float64(get.(int64)) * multiplier, nil
		case pmetric.NumberDataPointSlice:
			scaledMetric := pmetric.NewNumberDataPointSlice()
			get.(pmetric.NumberDataPointSlice).CopyTo(scaledMetric)
			scaleMetric(scaledMetric, multiplier)
			return scaledMetric, nil
		case pmetric.HistogramDataPointSlice:
			scaledMetric := pmetric.NewHistogramDataPointSlice()
			get.(pmetric.HistogramDataPointSlice).CopyTo(scaledMetric)
			scaleHistogram(scaledMetric, multiplier)
			return scaledMetric, nil
		case pmetric.ExponentialHistogramDataPointSlice:
			scaledMetric := pmetric.NewExponentialHistogramDataPointSlice()
			get.(pmetric.ExponentialHistogramDataPointSlice).CopyTo(scaledMetric)
			scaleExponentialHistogram(scaledMetric, multiplier)
			return scaledMetric, nil
		default:
			return nil, errors.New("unsupported data type")
		}
	}, nil
}

func scaleHistogram(datapoints pmetric.HistogramDataPointSlice, multiplier float64) {

	for i := 0; i < datapoints.Len(); i++ {
		dp := datapoints.At(i)

		if dp.HasSum() {
			dp.SetSum(dp.Sum() * multiplier)
		}
		if dp.HasMin() {
			dp.SetMin(dp.Min() * multiplier)
		}
		if dp.HasMax() {
			dp.SetMax(dp.Max() * multiplier)
		}

		for bounds, bi := dp.ExplicitBounds(), 0; bi < bounds.Len(); bi++ {
			bounds.SetAt(bi, bounds.At(bi)*multiplier)
		}

		for exemplars, ei := dp.Exemplars(), 0; ei < exemplars.Len(); ei++ {
			exemplar := exemplars.At(ei)
			switch exemplar.ValueType() {
			case pmetric.ExemplarValueTypeInt:
				exemplar.SetIntValue(int64(float64(exemplar.IntValue()) * multiplier))
			case pmetric.ExemplarValueTypeDouble:
				exemplar.SetDoubleValue(exemplar.DoubleValue() * multiplier)
			}
		}
	}
}

func scaleExponentialHistogram(datapoints pmetric.ExponentialHistogramDataPointSlice, multiplier float64) {
	for i := 0; i < datapoints.Len(); i++ {
		dp := datapoints.At(i)

		if dp.HasSum() {
			dp.SetSum(dp.Sum() * multiplier)
		}
		if dp.HasMin() {
			dp.SetMin(dp.Min() * multiplier)
		}
		if dp.HasMax() {
			dp.SetMax(dp.Max() * multiplier)
		}

		for exemplars, ei := dp.Exemplars(), 0; ei < exemplars.Len(); ei++ {
			exemplar := exemplars.At(ei)
			switch exemplar.ValueType() {
			case pmetric.ExemplarValueTypeInt:
				exemplar.SetIntValue(int64(float64(exemplar.IntValue()) * multiplier))
			case pmetric.ExemplarValueTypeDouble:
				exemplar.SetDoubleValue(exemplar.DoubleValue() * multiplier)
			}
		}
	}
}

func scaleMetric(points pmetric.NumberDataPointSlice, multiplier float64) {
	for i := 0; i < points.Len(); i++ {
		dp := points.At(i)
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			dp.SetIntValue(int64(float64(dp.IntValue()) * multiplier))

		case pmetric.NumberDataPointValueTypeDouble:
			dp.SetDoubleValue(dp.DoubleValue() * multiplier)
		default:
		}
	}
}
