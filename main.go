package main

import (
	"bitbucket.com/kmihaylov/afprint/io"
	"bitbucket.com/kmihaylov/afprint/signal"
	"encoding/gob"
	"fmt"
	"math"
	"math/cmplx"
	"os"
	"path/filepath"
)

const CHUNK = 4096

type chunkid struct {
	song string
	seq  int
}

type chunkfft []complex128

type freqdb map[string][]chunkfft
type printsdb map[string][]chunkid

type fp_settings struct {
	distance      int
	weight        int
	alg_type      string
	zoomer        float64
	damper        float64
	min_freq      int
	max_freq      int
	points_ignore []int
	points_count  int
}

// Encode via Gob to file
func Save(path string, object interface{}) error {
	file, err := os.Create(path)
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(object)
	}
	file.Close()
	return err
}

// Decode Gob file
func Load(path string, object interface{}) error {
	file, err := os.Open(path)
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(object)
	}
	file.Close()
	return err
}

func getRange(freq int, fps fp_settings) int {
	step := (fps.max_freq - fps.min_freq) / fps.points_count
	z := (freq - fps.min_freq) / step
	if z < 0 {
		z = 0
	}
	if z > fps.points_count-1 {
		z = fps.points_count - 1
	}
	RANGE := []int{40, 80, 120, 180, 300}
	i := 0
	for RANGE[i] < freq {
		i++
	}
	return z
}

func ToComplex(x []float32) []complex128 {
	y := make([]complex128, len(x))
	for n, v := range x {
		y[n] = complex(float64(v), 0)
	}
	return y
}

func fingerprint(chunk []complex128, fps fp_settings) string {
	max := make([]int, fps.points_count)
	maxf := make([]int, fps.points_count)

	for freq := fps.min_freq; freq < fps.max_freq; freq++ {
		// mag := cmplx.Abs(chunk[freq])
		mag := fps.zoomer * math.Log(cmplx.Abs(chunk[freq])+1)
		index := getRange(freq, fps)
		if max[index] < int(mag) {
			max[index] = int(mag - math.Mod(mag, fps.damper))
			maxf[index] = freq
		}
	}
	for _, p := range fps.points_ignore {
		max[p] = 0
		maxf[p] = 0
	}

	//fmt.Printf("%v\n", max)
	//fmt.Printf("%v\n", maxf)
	if fps.alg_type == "freq" { // match by frequency
		return fmt.Sprintf("%v", maxf)
	} else { // match by magnitude
		return fmt.Sprintf("%v", max)
	}
}

func parsesong(filename string, freqstore *freqdb) error {
	testWav, err := os.Open(filename)
	if err != nil {
		return err
	}

	// read the wav file
	wavReader, err := files.New(testWav)
	checkErr(err)

	frames := wavReader.Samples / CHUNK

	chunks := make([]chunkfft, frames)

	for i := 0; i < frames; i++ {
		d, err := wavReader.ReadFloats(CHUNK)
		checkErr(err)
		c := ToComplex(d)
		signal.CTFFT(d, c, len(d), 1)
		chunks[i] = c
		// add to freqstore
	}
	(*freqstore)[filename] = chunks

	return nil
}

func indexsong(filename string, freqstore *freqdb, pdb *printsdb, fps fp_settings) error {
	frames := (*freqstore)[filename]

	for i := 0; i < len(frames); i++ {
		c := frames[i]
		fp := fingerprint(c, fps)
		(*pdb)[fp] = append((*pdb)[fp], chunkid{filename, i})
	}

	return nil
}

func match(filename string, pdb *printsdb, fps fp_settings) (int, string) {
	testWav, err := os.Open(filename)
	checkErr(err)
	wavReader, err := files.New(testWav)

	checkErr(err)

	frames := wavReader.Samples / CHUNK

	high_score := 0
	high_name := ""

	scores := make(map[string]int)

	last_matches := make(map[string]chunkid)

	for frame_id := 0; frame_id < frames; frame_id++ {
		d, err := wavReader.ReadFloats(CHUNK)

		checkErr(err)

		c := ToComplex(d)
		signal.CTFFT(d, c, len(d), 1)
		fp := fingerprint(c, fps)

		matched := (*pdb)[fp]

		songs_matched := make(map[string]struct{})

		for i := range matched {
			chunk := matched[i]
			if _, already_matched := songs_matched[chunk.song]; already_matched {
				//fmt.Printf("skipping  match: %v\n", chunk)
				continue
			}
			// _ = "breakpoint"

			if last_matched := last_matches[chunk.song]; last_matched.seq < chunk.seq && chunk.seq < last_matched.seq+fps.distance {
				last_matches[chunk.song] = chunk
				weight := fps.weight / (chunk.seq - last_matched.seq)
				//fmt.Printf("%v Accepted match: %v. Score: %v  (%v)\n", frame_id, chunk, weight, chunk.seq-last_matched.seq)
				//fmt.Printf("fp: %v\n", fp)

				v1, has_score := scores[chunk.song]

				if has_score {
					scores[chunk.song] = v1 + weight
				} else {
					scores[chunk.song] = weight
				}
				if high_score < scores[chunk.song] {
					high_score = scores[chunk.song]
					high_name = chunk.song
				}
				songs_matched[chunk.song] = struct{}{}
			}
		}
	}

	fmt.Println("Scores ", scores, fps)
	return high_score, high_name
}

func tester(files []string, samplename string, freq_store freqdb, fps fp_settings) {
	// build new printstore with the settings
	prints_store := make(printsdb)
	for _, f := range files {
		indexsong(f, &freq_store, &prints_store, fps)
	}
	//match
	hs, hn := match(samplename, &prints_store, fps)
	fmt.Printf("Score: %v %v \n", hs, hn)
}

func main() {

	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: tagid <db dir> sample.wav\n")
		os.Exit(1)
	}
	freq_store := make(freqdb)

	samplename := os.Args[2]

	files, _ := filepath.Glob(os.Args[1] + "/*.wav")
	// build the freq db so that we can explore better algos
	for _, f := range files {
		fmt.Println("Parsing ", f)
		parsesong(f, &freq_store)
	}

	distances := []int{1, 5, 10, 100, 500, 1000, 3000}
	weights := []int{1, 5, 10, 100, 250, 500, 1000}
	dampers := []float64{1, 2, 3, 4, 5, 6, 10}
	zoomers := []float64{1, 2, 5, 10, 20, 40, 100}
	var pointsi [][]int
	pointsi = append(pointsi, []int{0, 4})
	pointsi = append(pointsi, []int{1, 4})
	pointsi = append(pointsi, []int{2, 4})
	pointsi = append(pointsi, []int{3, 4})
	pointsi = append(pointsi, []int{0})
	pointsi = append(pointsi, []int{1})
	pointsi = append(pointsi, []int{2})
	pointsi = append(pointsi, []int{3})
	pointsi = append(pointsi, []int{5})

	fs := []fp_settings{}
	for _, p := range pointsi {
		for _, z := range zoomers {
			for _, ds := range dampers {
				for _, w := range weights {
					for _, d := range distances {
						fs = append(fs, fp_settings{alg_type: "freq", distance: d, weight: w, damper: ds, zoomer: z, min_freq: 40, max_freq: 300, points_ignore: p, points_count: 5})
						fs = append(fs, fp_settings{alg_type: "mag", distance: d, weight: w, damper: ds, zoomer: z, min_freq: 40, max_freq: 300, points_ignore: p, points_count: 5})
					}
				}
			}
		}
	}
	for i := range fs {
		tester(files, samplename, freq_store, fs[i])
	}

}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
