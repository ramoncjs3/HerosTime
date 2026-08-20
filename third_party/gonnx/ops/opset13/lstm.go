package opset13

import (
	"github.com/advancedclimatesystems/gonnx/onnx"
	"github.com/advancedclimatesystems/gonnx/ops"
	"gorgonia.org/tensor"
)

const (
	MinLSTMInputs = 3
	MaxLSTMInputs = 8
)

// LSTM represents the ONNX lstm operator.
type LSTM struct {
	activationAlpha []float32
	activationBeta  []float32
	activations     []string
	direction       ops.SequenceProcessDirection
	hiddenSize      int
	inputForget     bool

	outputs []string
}

// newLSTM creates a new lstm operator.
func newLSTM() ops.Operator {
	return &LSTM{
		activations: []string{"sigmoid", "tanh", "tanh"},
		direction:   ops.Forward,
		inputForget: false,
		outputs:     []string{"Y", "Y_h", "Y_c"},
	}
}

// Init initializes the lstm operator.
func (l *LSTM) Init(n *onnx.NodeProto) error {
	for _, attr := range n.GetAttribute() {
		switch attr.GetName() {
		case ops.ActivationAlphaAttr:
			l.activationAlpha = attr.GetFloats()
		case ops.ActivationBetaAttr:
			l.activationBeta = attr.GetFloats()
		case ops.ActivationsAttr:
			activations := []string{}
			for _, activation := range attr.GetStrings() {
				activations = append(activations, string(activation))
			}

			l.activations = activations
		case ops.ClipAttr:
			return ops.ErrUnsupportedAttribute(attr.GetName(), l)
		case ops.DirectionAttr:
			l.direction = ops.SequenceProcessDirection(attr.GetS())
			switch l.direction {
			case ops.Forward, ops.Reverse, ops.Bidirectional:
			default:
				return ops.ErrUnsupportedAttribute(attr.GetName(), l)
			}
		case ops.HiddenSizeAttr:
			l.hiddenSize = int(attr.GetI())
		case "input_forget":
			l.inputForget = attr.GetI() == 1
		default:
			return ops.ErrInvalidAttribute(attr.GetName(), l)
		}
	}

	l.outputs = n.GetOutput()

	return nil
}

// Apply applies the lstm operator.
func (l *LSTM) Apply(inputs []tensor.Tensor) ([]tensor.Tensor, error) {
	if inputs[4] != nil {
		return nil, ops.ErrUnsupportedInput("sequence_lens", l)
	}

	X := inputs[0]
	seqLength := X.Shape()[0]
	batchSize := X.Shape()[1]

	nDirections := 1
	if l.direction == ops.Bidirectional {
		nDirections = 2
	}

	fActivation, err := ops.GetActivation(l.activations[0])
	if err != nil {
		return nil, err
	}

	gActivation, err := ops.GetActivation(l.activations[1])
	if err != nil {
		return nil, err
	}

	hActivation, err := ops.GetActivation(l.activations[2])
	if err != nil {
		return nil, err
	}

	P := inputs[7]

	dirYs := make([]tensor.Tensor, 0, nDirections)
	dirHs := make([]tensor.Tensor, 0, nDirections)
	dirCs := make([]tensor.Tensor, 0, nDirections)

	for dir := 0; dir < nDirections; dir++ {
		reversed := l.direction == ops.Reverse || (l.direction == ops.Bidirectional && dir == 1)

		Wi, Wo, Wf, Wc, err := l.getWeights(inputs[1], dir)
		if err != nil {
			return nil, err
		}

		Ri, Ro, Rf, Rc, err := l.getWeights(inputs[2], dir)
		if err != nil {
			return nil, err
		}

		B := inputs[3]
		if B == nil {
			// 8 is the number of bias matrices required by ONNX definition.
			nBiasMatrices := 8
			B = ops.ZeroTensor(nDirections, nBiasMatrices*l.hiddenSize)
		}

		Wbi, Wbo, Wbf, Wbc, Rbi, Rbo, Rbf, Rbc, err := l.getBiases(B, dir)
		if err != nil {
			return nil, err
		}

		Ht, err := l.initialState(inputs[5], dir, batchSize)
		if err != nil {
			return nil, err
		}

		Ct, err := l.initialState(inputs[6], dir, batchSize)
		if err != nil {
			return nil, err
		}

		var Pi, Po, Pf tensor.Tensor
		if P != nil {
			Pi, Po, Pf, err = l.getPeepholes(P, dir)
			if err != nil {
				return nil, err
			}
		}

		// ySeq[i] holds the hidden state of the current direction at
		// time step i, regardless of the processing order.
		ySeq := make([]tensor.Tensor, seqLength)

		for step := 0; step < seqLength; step++ {
			t := step
			if reversed {
				t = seqLength - 1 - step
			}

			Xt, err := X.Slice(ops.NewSlicer(t, t+1), nil, nil)
			if err != nil {
				return nil, err
			}

			it, err := l.gateCalculation(Xt, Wi, Wbi, Ht, Ri, Rbi, Pi, Ct, fActivation)
			if err != nil {
				return nil, err
			}

			ft, err := l.gateCalculation(Xt, Wf, Wbf, Ht, Rf, Rbf, Pf, Ct, fActivation)
			if err != nil {
				return nil, err
			}

			ct, err := l.gateCalculation(Xt, Wc, Wbc, Ht, Rc, Rbc, nil, nil, gActivation)
			if err != nil {
				return nil, err
			}

			Ct, err = l.cellCalculation(ft, it, ct, Ct)
			if err != nil {
				return nil, err
			}

			ot, err := l.gateCalculation(Xt, Wo, Wbo, Ht, Ro, Rbo, Po, Ct, fActivation)
			if err != nil {
				return nil, err
			}

			Ht, err = l.hiddenCalculation(ot, Ct, hActivation)
			if err != nil {
				return nil, err
			}

			ySeq[t] = Ht
		}

		Y := ySeq[0]
		if len(ySeq) > 1 {
			Y, err = tensor.Concat(0, Y, ySeq[1:]...)
			if err != nil {
				return nil, err
			}
		}

		Yh, ok := Ht.Clone().(tensor.Tensor)
		if !ok {
			return nil, ops.ErrTypeAssert("tensor.Tensor", Ht.Clone())
		}

		Yc, ok := Ct.Clone().(tensor.Tensor)
		if !ok {
			return nil, ops.ErrTypeAssert("tensor.Tensor", Ct.Clone())
		}

		dirYs = append(dirYs, Y)
		dirHs = append(dirHs, Yh)
		dirCs = append(dirCs, Yc)
	}

	Y, Yh, Yc, err := l.assembleOutputs(dirYs, dirHs, dirCs, seqLength, batchSize, nDirections)
	if err != nil {
		return nil, err
	}

	// ONNX defines the LSTM outputs positionally as Y, Y_h, Y_c; the names
	// in the graph do not necessarily match those keys.
	ordered := []tensor.Tensor{Y, Yh, Yc}

	result := []tensor.Tensor{}
	for i := range l.outputs {
		if i < len(ordered) {
			result = append(result, ordered[i])
		} else {
			result = append(result, nil)
		}
	}

	return result, nil
}

// initialState returns the initial hidden (or cell) state for a single
// direction with shape [batch, hidden].
func (l *LSTM) initialState(input tensor.Tensor, dir, batchSize int) (tensor.Tensor, error) {
	if input == nil {
		return ops.ZeroTensor(batchSize, l.hiddenSize), nil
	}

	// The state tensor has shape [num_directions, batch, hidden]; slicing
	// the direction dimension also drops it (gorgonia slice semantics).
	sliced, err := input.Slice(ops.NewSlicer(dir, dir+1), nil, nil)
	if err != nil {
		return nil, err
	}

	state := sliced.Materialize()
	if err := state.Reshape(batchSize, l.hiddenSize); err != nil {
		return nil, err
	}

	return state, nil
}

// assembleOutputs combines the per-direction results into the final Y, Y_h
// and Y_c tensors, inserting the num_directions dimension as specified by ONNX.
func (l *LSTM) assembleOutputs(
	dirYs, dirHs, dirCs []tensor.Tensor,
	seqLength, batchSize, nDirections int,
) (tensor.Tensor, tensor.Tensor, tensor.Tensor, error) {
	if nDirections == 1 {
		Y := dirYs[0]
		if err := Y.Reshape(seqLength, 1, batchSize, l.hiddenSize); err != nil {
			return nil, nil, nil, err
		}

		if err := dirHs[0].Reshape(1, batchSize, l.hiddenSize); err != nil {
			return nil, nil, nil, err
		}

		if err := dirCs[0].Reshape(1, batchSize, l.hiddenSize); err != nil {
			return nil, nil, nil, err
		}

		return Y, dirHs[0], dirCs[0], nil
	}

	for _, Y := range dirYs {
		if err := Y.Reshape(seqLength, 1, batchSize, l.hiddenSize); err != nil {
			return nil, nil, nil, err
		}
	}

	Y, err := tensor.Concat(1, dirYs[0], dirYs[1:]...)
	if err != nil {
		return nil, nil, nil, err
	}

	Yh, err := tensor.Concat(0, dirHs[0], dirHs[1:]...)
	if err != nil {
		return nil, nil, nil, err
	}

	Yc, err := tensor.Concat(0, dirCs[0], dirCs[1:]...)
	if err != nil {
		return nil, nil, nil, err
	}

	return Y, Yh, Yc, nil
}

// ValidateInputs validates the inputs that will be given to Apply for this operator.
func (l *LSTM) ValidateInputs(inputs []tensor.Tensor) ([]tensor.Tensor, error) {
	return ops.ValidateInputs(l, inputs)
}

// GetMinInputs returns the minimum number of input tensors this operator expects.
func (l *LSTM) GetMinInputs() int {
	return MinLSTMInputs
}

// GetMaxInputs returns the maximum number of input tensors this operator expects.
func (l *LSTM) GetMaxInputs() int {
	return MaxLSTMInputs
}

// GetInputTypeConstraints returns a list. Every element represents a set of allowed tensor dtypes
// for the corresponding input tensor.
func (l *LSTM) GetInputTypeConstraints() [][]tensor.Dtype {
	return [][]tensor.Dtype{
		{tensor.Float32, tensor.Float64},
		{tensor.Float32, tensor.Float64},
		{tensor.Float32, tensor.Float64},
		{tensor.Float32, tensor.Float64},
		{tensor.Int32},
		{tensor.Float32, tensor.Float64},
		{tensor.Float32, tensor.Float64},
		{tensor.Float32, tensor.Float64},
	}
}

// String implements the stringer interface, and can be used to format errors or messages.
func (l *LSTM) String() string {
	return "lstm operator"
}

// gateCalculation performs a standard gate calculation for an LSTM gate defined as:
//
//	o = f(Xt*(W^T) + Wb + H*(R^T) + Rb + P (.) C)
//
// Where:
//   - 'f()' is an activation function
//   - 'Xt' is the input tensor
//   - 'W' is the input weight
//   - 'Wb' is the input bias
//   - 'H' is the hidden tensor
//   - 'R' is the hidden weight tensor
//   - 'Rb' is the hidden bias
//   - 'P' are peephole weights (optional, can be nil)
//   - 'C' is the cell state
//   - '(.)' is element-wise multiplication
//
// 'o' is the result tensor that is returned.
// This calculation can be used for the forget gate, input gate, cell gate
// and output gate calculations.
func (l *LSTM) gateCalculation(
	Xt, W, Wb, H, R, Rb, P, C tensor.Tensor, activation ops.Activation,
) (tensor.Tensor, error) {
	gemm := &Gemm{transA: false, transB: true, alpha: 1.0, beta: 1.0}

	inputCalc, err := gemm.Apply([]tensor.Tensor{Xt, W, Wb})
	if err != nil {
		return nil, err
	}

	hiddenCalc, err := gemm.Apply([]tensor.Tensor{H, R, Rb})
	if err != nil {
		return nil, err
	}

	output, err := tensor.Add(inputCalc[0], hiddenCalc[0])
	if err != nil {
		return nil, err
	}

	if P != nil {
		C, broadcastedP, err := ops.UnidirectionalBroadcast(C, P)
		if err != nil {
			return nil, err
		}

		peepholeActivation, err := tensor.Mul(broadcastedP, C)
		if err != nil {
			return nil, err
		}

		output, err = tensor.Add(output, peepholeActivation)
		if err != nil {
			return nil, err
		}
	}

	return activation(output)
}

// cellCalculation performs the calculation of the LSTM cell update defined by:
//
//	Ct = ft (.) Ct-1 + it (.) ct
//
// Where 'ft' is the forget gate activation at time t, (.) denotes element-wise
// multiplication, 'Ct-1' is the cell state at time t-1, 'it' denotes the input
// gate activation at time t and 'ct' denotes the cell state activation at time t (which)
// is not the same as Ct or Ct-1).
func (l *LSTM) cellCalculation(ft, it, ct, Ct tensor.Tensor) (tensor.Tensor, error) {
	cellForget, err := tensor.Mul(ft, Ct)
	if err != nil {
		return nil, err
	}

	cellInput, err := tensor.Mul(it, ct)
	if err != nil {
		return nil, err
	}

	return tensor.Add(cellForget, cellInput)
}

// hiddenCalculation performs the calculation of the new LSTM hidden state defined as:
//
//	Ht = ot (.) h(Ct)
//
// Where Ht is the new hidden state at time t, 'ot' is the output gate at time t, (.) denotes
// element-wise multiplication, 'h()' denotes the activation function and 'Ct' denotes
// the cell state at time t.
func (l *LSTM) hiddenCalculation(ot, Ct tensor.Tensor, activation ops.Activation) (tensor.Tensor, error) {
	cellActivated, err := activation(Ct)
	if err != nil {
		return nil, err
	}

	return tensor.Mul(ot, cellActivated)
}

// getWeights splits the weights for a single direction out of tensor W and
// splits that into 4 weight matrices.
// The W tensor, by GONNX definition, has 3 dimensions with 4 weight
// tensors in it per direction (8 if bidirectional).
func (l *LSTM) getWeights(W tensor.Tensor, dir int) (Wi, Wo, Wf, Wh tensor.Tensor, err error) {
	nWeightMatrices := 4
	nWeightDimensions := 3

	weights, err := extractMatricesAt(W, nWeightMatrices, nWeightDimensions, l.hiddenSize, dir)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return weights[0], weights[1], weights[2], weights[3], nil
}

// getBiases splits the biases for a single direction out of tensor B and
// splits that into 8 bias matrices.
// The B tensor, by GONNX definition, has 2 dimensions with 8 bias
// tensors in it per direction (16 if bidirectional).
func (l *LSTM) getBiases(B tensor.Tensor, dir int) (Wbi, Wbo, Wbf, Wbc, Rbi, Rbo, Rbf, Rbc tensor.Tensor, err error) {
	nBiasMatrices := 8
	nBiasDimensions := 2

	b, err := extractMatricesAt(B, nBiasMatrices, nBiasDimensions, l.hiddenSize, dir)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	return b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7], nil
}

// getPeepholes splits the peephole weights for a single direction out of
// tensor P and splits that into 3 peephole matrices.
// The P tensor, by GONNX definition, has 2 dimensions with 3 peephole
// tensors in it per direction (6 if bidirectional).
func (l *LSTM) getPeepholes(P tensor.Tensor, dir int) (Pi, Po, Pf tensor.Tensor, err error) {
	nPeepholeMatrices := 3
	nPeepholeDimensions := 2

	p, err := extractMatricesAt(P, nPeepholeMatrices, nPeepholeDimensions, l.hiddenSize, dir)
	if err != nil {
		return nil, nil, nil, err
	}

	return p[0], p[1], p[2], nil
}

// extractMatricesAt extracts a given number of matrices from the direction
// slice of tensor M. It is like ops.ExtractMatrices, but selects an
// arbitrary direction dimension instead of only the first.
func extractMatricesAt(M tensor.Tensor, nMatrices, nDimensions, hiddenSize, dir int) ([]tensor.Tensor, error) {
	matrices := make([]tensor.Tensor, nMatrices)

	for i := 0; i < nMatrices; i++ {
		allSlices := make([]tensor.Slice, nDimensions)
		allSlices[0] = ops.NewSlicer(dir, dir+1)
		allSlices[1] = ops.NewSlicer(i*hiddenSize, (i+1)*hiddenSize)

		for j := 2; j < nDimensions; j++ {
			allSlices[j] = nil
		}

		m, err := M.Slice(allSlices...)
		if err != nil {
			return nil, err
		}

		matrices[i] = m
	}

	return matrices, nil
}
