package signal

import (
	"math"
	"math/cmplx"
)

func CTFFT(x []float32, y []complex128, n, s int) {
	if n == 1 {
		y[0] = complex(float64(x[0]), 0)
		return
	}
	CTFFT(x, y, n/2, 2*s)
	CTFFT(x[s:], y[n/2:], n/2, 2*s)
	for k := 0; k < n/2; k++ {
		tf := cmplx.Rect(1, -2*math.Pi*float64(k)/float64(n)) * y[k+n/2]
		y[k], y[k+n/2] = y[k]+tf, y[k]-tf
	}
}
