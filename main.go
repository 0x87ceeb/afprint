package main

import (
	"bitbucket.com/kmihaylov/afprint/io"
	"bitbucket.com/kmihaylov/afprint/signal"
	"encoding/gob"
	"fmt"
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

func getRange(freq int) int {
	RANGE := []int{40, 80, 120, 180, 300}
	i := 0
	for RANGE[i] < freq {
		i++
	}
	return i
}

func ToComplex(x []float32) []complex128 {
	y := make([]complex128, len(x))
	for n, v := range x {
		y[n] = complex(float64(v), 0)
	}
	return y
}

func fingerprint(chunk []complex128) string {
	const DAMPER = 2
	const ZOOMER = 30
	max := []float64{0, 0, 0, 0, 0}
	maxf := []int{0, 0, 0, 0, 0}
	for freq := 40; freq < 300; freq++ {
		//mag := ZOOMER * math.Log(cmplx.Abs(chunk[freq])+1)
		mag := cmplx.Abs(chunk[freq])
		index := getRange(freq)
		if max[index] < mag {
			max[index] = mag
			maxf[index] = freq
		}
	}
	//return fmt.Sprintf("%v%v%v", max[0]-math.Mod(max[0], DAMPER), max[2]-math.Mod(max[2], DAMPER), max[3]-math.Mod(max[3], DAMPER))
	return fmt.Sprintf("%v.%v.%v", maxf[1], maxf[2], maxf[3])
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

func indexsong(filename string, freqstore *freqdb, pdb *printsdb) error {
	frames := (*freqstore)[filename]

	for i := 0; i < len(frames); i++ {
		c := frames[i]
		fp := fingerprint(c)
		(*pdb)[fp] = append((*pdb)[fp], chunkid{filename, i})
	}

	return nil
}

func match(filename string, pdb *printsdb) int {
	const CHUNK = 4096

	testWav, err := os.Open(filename)
	checkErr(err)
	wavReader, err := files.New(testWav)

	checkErr(err)

	frames := wavReader.Samples / CHUNK

	score := 0

	scores := make(map[string]int)

	last_matches := make(map[string]chunkid)

	for i := 0; i < frames; i++ {
		d, err := wavReader.ReadFloats(CHUNK)

		checkErr(err)

		c := ToComplex(d)
		signal.CTFFT(d, c, len(d), 1)
		fp := fingerprint(c)

		matched := (*pdb)[fp]
		for i := range matched {
			chunk := matched[i]

			// _ = "breakpoint"

			if last_matched := last_matches[chunk.song]; last_matched.seq < chunk.seq && chunk.seq < last_matched.seq+10000 {
				last_matches[chunk.song] = chunk
				weight := 250.0 / (chunk.seq - last_matched.seq)
				fmt.Printf("Accepted match: %v. Score: %v  (%v)\n", chunk, weight, chunk.seq-last_matched.seq)
				fmt.Printf("fp: %v\n", fp)

				v1, has_score := scores[chunk.song]

				if has_score {
					score = v1 + weight
				} else {
					score = weight
				}
				scores[chunk.song] = score
			}
		}
	}

	fmt.Println("Scores ", scores)
	return score
}

func tester(files []string, samplename string, freq_store freqdb) {
	fmt.Println("testing with settings : ...")
	// build new printstore with random settings
	prints_store := make(printsdb)
	for _, f := range files {
		fmt.Println("Indexing ", f)
		indexsong(f, &freq_store, &prints_store)
	}
	//match with random settings§p
	fmt.Println("Score: ", match(samplename, &prints_store))
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

	// save the freqdb to gob
	fmt.Println("saving")
	// Save("freqdb.gob", freq_store)
	fmt.Println("saved")

	// load from gob
	fmt.Println("loading")
	//freq_store1 := make(freqdb)
	//Load("freqdb.gob", freq_store1)
	fmt.Println("done")
	tester(files, samplename, freq_store)
	//tester(files, samplename, freq_store1)

}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
