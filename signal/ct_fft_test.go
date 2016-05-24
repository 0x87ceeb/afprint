package signal

import (
	"reflect"
	"testing"
)

func TestWithNonPowerOf2(t *testing.T) {
	inp := []float32{1, 1, 1}
	exp := []complex128{}

	out := FFT(inp)

	if !reflect.DeepEqual(out, exp) {
		t.Error(out)
		t.Fail()
	}
}

func TestCTFFTAllZeros(t *testing.T) {
	inp := []float32{0, 0, 0, 0}
	out := toComplex(inp)
	exp := []complex128{complex(0, 0), complex(0, 0), complex(0, 0), complex(0, 0)}

	ctFFT(inp, out, len(out), 1)

	if !reflect.DeepEqual(out, exp) {
		t.Fail()
	}
}

// test recursion stop
func TestRecursion1(t *testing.T) {
	inp := []float32{1, 0, 0, 0}
	out := toComplex(inp)
	exp := []complex128{complex(1, 0), complex(0, 0), complex(0, 0), complex(0, 0)}

	ctFFT(inp, out, 1, 1)

	if !reflect.DeepEqual(out, exp) {
		t.Fail()
	}
}

func TestPowerOf2(t *testing.T) {
	if !isPowerof2(4) {
		t.Fail()
	}

	if isPowerof2(3) {
		t.Fail()
	}
}
