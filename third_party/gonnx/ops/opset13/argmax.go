package opset13

import (
	"github.com/advancedclimatesystems/gonnx/onnx"
	"github.com/advancedclimatesystems/gonnx/ops"
	"gorgonia.org/tensor"
)

const (
	MinArgMaxInputs = 1
	MaxArgMaxInputs = 1
)

// ArgMax represents the ONNX argmax operator.
type ArgMax struct {
	axis     int
	keepdims bool
}

// newArgMax creates a new argmax operator.
func newArgMax() ops.Operator {
	return &ArgMax{
		axis:     0,
		keepdims: true,
	}
}

// Init initializes the argmax operator.
func (a *ArgMax) Init(n *onnx.NodeProto) error {
	for _, attr := range n.GetAttribute() {
		switch attr.GetName() {
		case "axis":
			a.axis = int(attr.GetI())
		case "keepdims":
			a.keepdims = attr.GetI() == 1
		case "select_last_index":
			if attr.GetI() != 0 {
				return ops.ErrUnsupportedAttribute(attr.GetName(), a)
			}
		default:
			return ops.ErrInvalidAttribute(attr.GetName(), a)
		}
	}

	return nil
}

// Apply applies the argmax operator.
func (a *ArgMax) Apply(inputs []tensor.Tensor) ([]tensor.Tensor, error) {
	data := inputs[0]
	shape := data.Shape()
	rank := len(shape)

	axis := a.axis
	if axis < 0 {
		axis += rank
	}

	if axis < 0 || axis > rank-1 {
		return nil, ops.ErrAxisOutOfRange(rank, rank, a.axis)
	}

	outShape := append([]int{}, shape[:axis]...)

	if a.keepdims {
		outShape = append(outShape, 1)
	}

	outShape = append(outShape, shape[axis+1:]...)

	if len(outShape) == 0 {
		outShape = []int{1}
	}

	out := tensor.New(tensor.WithShape(outShape...), tensor.Of(tensor.Int64))

	it := out.Iterator()
	it.Reset()

	for !it.Done() {
		coords := it.Coord()

		// Base coordinates of the data tensor for this output element:
		// the axis coordinate scans separately; when keepdims=0 the output
		// coordinates after the axis are offset by one.
		dataCoords := make([]int, 0, rank)
		for i := 0; i < rank; i++ {
			switch {
			case i == axis:
				dataCoords = append(dataCoords, 0)
			case i < axis:
				dataCoords = append(dataCoords, coords[i])
			case a.keepdims:
				dataCoords = append(dataCoords, coords[i])
			default:
				dataCoords = append(dataCoords, coords[i-1])
			}
		}

		bestIdx := 0
		first, err := data.At(dataCoords...)
		if err != nil {
			return nil, err
		}
		bestVal, err := argMaxValue(first)
		if err != nil {
			return nil, err
		}

		for i := 1; i < shape[axis]; i++ {
			dataCoords[axis] = i

			raw, err := data.At(dataCoords...)
			if err != nil {
				return nil, err
			}

			v, err := argMaxValue(raw)
			if err != nil {
				return nil, err
			}

			if v > bestVal {
				bestVal = v
				bestIdx = i
			}
		}

		if err := out.SetAt(int64(bestIdx), coords...); err != nil {
			return nil, err
		}

		if _, err := it.Next(); err != nil {
			return nil, err
		}
	}

	return []tensor.Tensor{out}, nil
}

// argMaxValue converts a numeric tensor element to float64 for comparison.
func argMaxValue(v interface{}) (float64, error) {
	switch x := v.(type) {
	case float32:
		return float64(x), nil
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case int8:
		return float64(x), nil
	case int16:
		return float64(x), nil
	case int32:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case uint8:
		return float64(x), nil
	case uint16:
		return float64(x), nil
	case uint32:
		return float64(x), nil
	case uint64:
		return float64(x), nil
	default:
		return 0, ops.ErrTypeAssert("numeric type", v)
	}
}

// ValidateInputs validates the inputs that will be given to Apply for this operator.
func (a *ArgMax) ValidateInputs(inputs []tensor.Tensor) ([]tensor.Tensor, error) {
	return ops.ValidateInputs(a, inputs)
}

// GetMinInputs returns the minimum number of input tensors this operator expects.
func (a *ArgMax) GetMinInputs() int {
	return MinArgMaxInputs
}

// GetMaxInputs returns the maximum number of input tensors this operator expects.
func (a *ArgMax) GetMaxInputs() int {
	return MaxArgMaxInputs
}

// GetInputTypeConstraints returns a list. Every element represents a set of allowed tensor dtypes
// for the corresponding input tensor.
func (a *ArgMax) GetInputTypeConstraints() [][]tensor.Dtype {
	return [][]tensor.Dtype{
		{tensor.Uint8, tensor.Uint16, tensor.Uint32, tensor.Uint64, tensor.Int8, tensor.Int16, tensor.Int32, tensor.Int64, tensor.Float32, tensor.Float64},
	}
}

// String implements the stringer interface, and can be used to format errors or messages.
func (a *ArgMax) String() string {
	return "argmax operator"
}
