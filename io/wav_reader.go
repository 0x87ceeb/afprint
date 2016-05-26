// implements reading of WAV files (only as floats for the moment)
// check http://www-mmsp.ece.mcgill.ca/Documents/AudioFormats/WAVE/WAVE.html for format description

package files

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math"
)

const (
	wavFormatPCM       = 1
	wavFormatIEEEFloat = 3
)

// Header containing Wav fmt chunk data.
type FormatChunk struct {
	FormatTag      uint16
	NumChannels    uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
}

// Wav file descriptor
type WavFile struct {
	FormatChunk
	ChunksCount int
	r           io.Reader
}

// Reads the WAV file from r.
func Open(r io.Reader) (*WavFile, error) {
	var w WavFile
	header := make([]byte, 16)
	if _, err := io.ReadFull(r, header[:12]); err != nil {
		return nil, err
	}
	if string(header[0:4]) != "RIFF" {
		return nil, fmt.Errorf("wav: missing RIFF")
	}
	if string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("wav: missing WAVE")
	}
	hasFmt := false
	for {
		if _, err := io.ReadFull(r, header[:8]); err != nil {
			return nil, err
		}
		sz := binary.LittleEndian.Uint32(header[4:])
		switch typ := string(header[:4]); typ {
		case "fmt ":
			if sz < 16 {
				return nil, fmt.Errorf("wav: bad fmt size")
			}
			f := make([]byte, sz)
			if _, err := io.ReadFull(r, f); err != nil {
				return nil, err
			}
			if err := binary.Read(bytes.NewBuffer(f), binary.LittleEndian, &w.FormatChunk); err != nil {
				return nil, err
			}
			switch w.FormatTag {
			case wavFormatPCM:
			case wavFormatIEEEFloat:
			default:
				return nil, fmt.Errorf("wav: unknown audio format: %02x", w.FormatTag)
			}
			hasFmt = true
		case "data":
			if !hasFmt {
				return nil, fmt.Errorf("wav: unknown chunk format")
			}
			w.ChunksCount = int(sz) / int(w.BitsPerSample) * 8
			w.r = io.LimitReader(r, int64(sz))
			return &w, nil
		default:
			io.CopyN(ioutil.Discard, r, int64(sz))
		}
	}
}

// Reads N chunks returning the data as []float32
func (w *WavFile) Read(n int) ([]float32, error) {
	var data interface{}
	var f []float32

	switch w.FormatTag {
	case wavFormatPCM:
		switch w.BitsPerSample {
		case 8:
			data = make([]uint8, n)
		case 16:
			data = make([]int16, n)
		default:
			return nil, fmt.Errorf("wav: unknown bits per sample: %v", w.BitsPerSample)
		}
	case wavFormatIEEEFloat:
		data = make([]float32, n)
	default:
		return nil, fmt.Errorf("wav: unknown audio format")
	}

	if err := binary.Read(w.r, binary.LittleEndian, data); err != nil {
		return nil, err
	}

	switch data := data.(type) {
	case []uint8:
		f = make([]float32, len(data))
		for i, v := range data {
			f[i] = float32(v) / math.MaxUint8
		}
	case []int16:
		f = make([]float32, len(data))
		for i, v := range data {
			f[i] = (float32(v) - math.MinInt16) / (math.MaxInt16 - math.MinInt16)
		}
	case []float32:
		f = data
	default:
		return nil, fmt.Errorf("wav: unknown type: %T", data)
	}
	return f, nil
}
