package opset13

import (
	"sort"

	"github.com/advancedclimatesystems/gonnx/onnx"
	"github.com/advancedclimatesystems/gonnx/ops"
	"gorgonia.org/tensor"
)

const (
	MinUnsqueezeInputs = 1
	MaxUnsqueezeInputs = 2
)

// Unsqueeze represents the ONNX unsqueeze operator.
type Unsqueeze struct {
	axes    []int
	hasAxes bool
}

// newUnsqueeze creates a new unsqueeze operator.
func newUnsqueeze() ops.Operator {
	return &Unsqueeze{}
}

// Init initializes the unsqueeze operator.
func (u *Unsqueeze) Init(n *onnx.NodeProto) error {
	// Opset < 13 passes axes as an attribute instead of an input tensor.
	for _, attr := range n.GetAttribute() {
		if attr.GetName() == "axes" {
			ints := attr.GetInts()
			u.axes = make([]int, len(ints))
			for i, v := range ints {
				u.axes[i] = int(v)
			}
			u.hasAxes = true
		} else {
			return ops.ErrInvalidAttribute(attr.GetName(), u)
		}
	}

	return nil
}

// Apply applies the unsqueeze operator.
func (u *Unsqueeze) Apply(inputs []tensor.Tensor) ([]tensor.Tensor, error) {
	dataShape := inputs[0].Shape()

	var axes []int
	if len(inputs) > 1 && inputs[1] != nil {
		var err error
		axes, err = ops.AnyToIntSlice(inputs[1].Data())
		if err != nil {
			return nil, err
		}
	} else if u.hasAxes {
		axes = append([]int{}, u.axes...)
	} else {
		return nil, ops.ErrInvalidInput("axes are required (attribute or input tensor)", u)
	}

	outputRank := len(dataShape) + len(axes)

	if !ops.AllInRange(axes, -outputRank, outputRank-1) {
		return nil, ops.ErrNotAllAxesInRange(outputRank, outputRank)
	}

	// negative entries should be offset by the rank of the output tensor
	// i.e. -1 -> outputRank - 1, -outputrank -> 0
	ops.OffsetArrayIfNegative(axes, outputRank)

	sort.Ints(axes)

	if ops.HasDuplicates(axes) {
		return nil, ops.ErrInvalidInput("axes cannot have duplicate entries after offset", u)
	}

	newShape := insertOnes(dataShape, axes)

	out, ok := inputs[0].Clone().(tensor.Tensor)
	if !ok {
		return nil, ops.ErrTypeAssert("tensor.Tensor", inputs[0].Clone())
	}

	return []tensor.Tensor{out}, out.Reshape(newShape...)
}

// ValidateInputs validates the inputs that will be given to Apply for this operator.
func (u *Unsqueeze) ValidateInputs(inputs []tensor.Tensor) ([]tensor.Tensor, error) {
	return ops.ValidateInputs(u, inputs)
}

// GetMinInputs returns the minimum number of input tensors this operator expects.
func (u *Unsqueeze) GetMinInputs() int {
	return MinUnsqueezeInputs
}

// GetMaxInputs returns the maximum number of input tensors this operator expects.
func (u *Unsqueeze) GetMaxInputs() int {
	return MaxUnsqueezeInputs
}

// GetInputTypeConstraints returns a list. Every element represents a set of allowed tensor dtypes
// for the corresponding input tensor.
func (u *Unsqueeze) GetInputTypeConstraints() [][]tensor.Dtype {
	return [][]tensor.Dtype{ops.AllTypes, {tensor.Int64}}
}

// String implements the stringer interface, and can be used to format errors or messages.
func (u *Unsqueeze) String() string {
	return "unsqueeze operator"
}

// Creates a new array, which is `original` with ones added at the indices specified by `indices`
// `indices` may not contain duplicates, the elements are assumed to be in the range 0 <= x < N
// and should be sorted in increasing order.
// Is done in a single pass through the new array with length: len(original) + len(indices).
func insertOnes(original, indices []int) []int {
	N := len(indices) + len(original)

	// Pre-allocate the output shape
	newShape := make([]int, N)

	originalIdx := 0
	indicesIdx := 0

	for i := 0; i < N; i++ {
		if indicesIdx < len(indices) && indices[indicesIdx] == i {
			newShape[i] = 1
			indicesIdx++
		} else {
			newShape[i] = original[originalIdx]
			originalIdx++
		}
	}

	return newShape
}
