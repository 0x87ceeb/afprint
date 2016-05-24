package main

import (
	"bitbucket.com/kmihaylov/afprint/io"
	"bitbucket.com/kmihaylov/afprint/signal"
	"flag"
	"fmt"
	"math"
	"math/cmplx"
	"os"
	"path/filepath"
)

const chunkSize = 4096

type chunkid struct {
	song string
	seq  int
}

type chunkfft []complex128

type freqdb map[string][]chunkfft
type printsdb map[string][]chunkid

type fpSettings struct {
	distance     int
	weight       int
	algType      string
	zoomer       float64
	damper       float64
	minFreq      int
	maxFeq       int
	pointsIgnore []int
	pointsCount  int
}

func getRange(freq int, fps fpSettings) int {
	step := (fps.maxFeq - fps.minFreq) / fps.pointsCount
	z := (freq - fps.minFreq) / step
	if z < 0 {
		z = 0
	}
	if z > fps.pointsCount-1 {
		z = fps.pointsCount - 1
	}
	RANGE := []int{40, 80, 120, 180, 300}
	i := 0
	for RANGE[i] < freq {
		i++
	}
	return z
}

func fingerprint(chunk []complex128, fps fpSettings) string {
	max := make([]int, fps.pointsCount)
	maxf := make([]int, fps.pointsCount)

	for freq := fps.minFreq; freq < fps.maxFeq; freq++ {
		// mag := cmplx.Abs(chunk[freq])
		mag := fps.zoomer * math.Log(cmplx.Abs(chunk[freq])+1)
		index := getRange(freq, fps)
		if max[index] < int(mag) {
			max[index] = int(mag - math.Mod(mag, fps.damper))
			maxf[index] = freq
		}
	}
	for _, p := range fps.pointsIgnore {
		max[p] = 0
		maxf[p] = 0
	}

	//fmt.Printf("%v\n", max)
	//fmt.Printf("%v\n", maxf)
	if fps.algType == "freq" { // match by frequency
		return fmt.Sprintf("%v", maxf)
	}
	// Default case : match by magnitude
	return fmt.Sprintf("%v", max)
}

func parsesong(filename string, freqstore *freqdb) {
	testWav, err := os.Open(filename)
	if err != nil {
		return
	}

	// read the wav file
	wavReader, err := files.New(testWav)
	checkErr(err)

	frames := wavReader.Samples / chunkSize

	chunks := make([]chunkfft, frames)

	for i := 0; i < frames; i++ {
		d, err := wavReader.ReadFloats(chunkSize)
		checkErr(err)
		c := signal.FFT(d)
		chunks[i] = c
		// add to freqstore
	}
	(*freqstore)[filename] = chunks
}

func indexsong(filename string, freqstore *freqdb, pdb *printsdb, fps fpSettings) {
	frames := (*freqstore)[filename]

	for i := 0; i < len(frames); i++ {
		c := frames[i]
		fp := fingerprint(c, fps)
		(*pdb)[fp] = append((*pdb)[fp], chunkid{filename, i})
	}
}

func match(filename string, pdb *printsdb, fps fpSettings) (int, string) {
	testWav, err := os.Open(filename)
	if err != nil {
		fmt.Println("cannot open", filename)
		os.Exit(-1)
	}
	wavReader, err := files.New(testWav)

	checkErr(err)

	frames := wavReader.Samples / chunkSize

	highScore := 0
	highName := ""

	scores := make(map[string]int)

	lastMatches := make(map[string]chunkid)

	for frameID := 0; frameID < frames; frameID++ {
		d, err := wavReader.ReadFloats(chunkSize)

		checkErr(err)

		c := signal.FFT(d)
		fp := fingerprint(c, fps)

		matched := (*pdb)[fp]

		songsMatched := make(map[string]struct{})

		for i := range matched {
			chunk := matched[i]
			if _, alreadyMatched := songsMatched[chunk.song]; alreadyMatched {
				//fmt.Printf("skipping  match: %v\n", chunk)
				continue
			}
			// _ = "breakpoint"

			if lastMatched := lastMatches[chunk.song]; lastMatched.seq < chunk.seq && chunk.seq < lastMatched.seq+fps.distance {
				lastMatches[chunk.song] = chunk
				weight := fps.weight / (chunk.seq - lastMatched.seq)
				//fmt.Printf("%v Accepted match: %v. Score: %v  (%v)\n", frame_id, chunk, weight, chunk.seq-last_matched.seq)
				//fmt.Printf("fp: %v\n", fp)

				v1, hasScore := scores[chunk.song]

				if hasScore {
					scores[chunk.song] = v1 + weight
				} else {
					scores[chunk.song] = weight
				}
				if highScore < scores[chunk.song] {
					highScore = scores[chunk.song]
					highName = chunk.song
				}
				songsMatched[chunk.song] = struct{}{}
			}
		}
	}

	fmt.Println("Scores ", scores, fps)
	return highScore, highName
}

func tester(files []string, samplename string, freqStore freqdb, fps fpSettings) {
	// build new printstore with the settings
	printsStore := make(printsdb)
	for _, f := range files {
		indexsong(f, &freqStore, &printsStore, fps)
	}
	//match
	hs, hn := match(samplename, &printsStore, fps)
	fmt.Printf("Score: %v %v \n", hs, hn)
}

func main() {
	// parse the command line arguments
	musicfiles := flag.String("db", "music", "path to where the reference samples are stored")
	samplename := flag.String("sample", "sample.wav", "name of the sample file to be tested")

	flag.Parse()
	freqStore := make(freqdb)

	files, _ := filepath.Glob(*musicfiles + "/*.wav")
	// build the freq db so that we can explore better algos
	for _, f := range files {
		fmt.Println("Parsing ", f)
		parsesong(f, &freqStore)
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

	fs := []fpSettings{}
	for _, p := range pointsi {
		for _, z := range zoomers {
			for _, ds := range dampers {
				for _, w := range weights {
					for _, d := range distances {
						fs = append(fs, fpSettings{algType: "freq", distance: d, weight: w, damper: ds, zoomer: z, minFreq: 40, maxFeq: 300, pointsIgnore: p, pointsCount: 5})
						fs = append(fs, fpSettings{algType: "mag", distance: d, weight: w, damper: ds, zoomer: z, minFreq: 40, maxFeq: 300, pointsIgnore: p, pointsCount: 5})
					}
				}
			}
		}
	}
	for i := range fs {
		tester(files, *samplename, freqStore, fs[i])
	}

}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
