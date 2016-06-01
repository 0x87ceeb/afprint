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

// simple map to hold total score for how good settings perform
type settingsResults map[string]int

// returns the index in the print array for the given frequency
// this allows for experimenting with different frequency bands
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

// fingerprints the songs using either frequency match or magnitude
func fingerprint(chunk []complex128, fps fpSettings) string {
	max := make([]int, fps.pointsCount)
	maxf := make([]int, fps.pointsCount)

	for freq := fps.minFreq; freq < fps.maxFeq; freq++ {
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

	if fps.algType == "freq" { // match by frequency
		return fmt.Sprintf("%v", maxf)
	}
	// Default case : match by magnitude
	return fmt.Sprintf("%v", max)
}

// reads a wav file in chunks, performing fft on each and stores the result in freqdb
func parsesong(filename string, freqstore *freqdb) {
	testWav, err := os.Open(filename)
	if err != nil {
		return
	}

	// read the wav file
	wavReader, err := files.Open(testWav)
	checkErr(err)

	frames := wavReader.ChunksCount / chunkSize

	chunks := make([]chunkfft, frames)

	for i := 0; i < frames; i++ {
		d, err := wavReader.Read(chunkSize)
		checkErr(err)
		c := signal.FFT(d)
		chunks[i] = c
		// add to freqstore
	}
	(*freqstore)[filename] = chunks
}

// perfroms a fingerprint operation on the song, using the stored ffts and the particular settings
func indexsong(filename string, freqstore *freqdb, pdb *printsdb, fps fpSettings) {
	frames := (*freqstore)[filename]

	for i := 0; i < len(frames); i++ {
		c := frames[i]
		fp := fingerprint(c, fps)
		(*pdb)[fp] = append((*pdb)[fp], chunkid{filename, i})
	}
}

// mathes a wav file to a music db. returns a percentage match, the score and the name of the highest match
func match(filename string, pdb *printsdb, fps fpSettings) (int, int, string) {
	testWav, err := os.Open(filename)
	if err != nil {
		fmt.Println("cannot open", filename)
		os.Exit(-1)
	}
	wavReader, err := files.Open(testWav)

	checkErr(err)

	frames := wavReader.ChunksCount / chunkSize

	highScore := 0
	highName := ""

	scores := make(map[string]int)

	lastMatches := make(map[string]chunkid)

	// loop through each frame and increase the total score for the matched songs
	for frameID := 0; frameID < frames; frameID++ {
		d, err := wavReader.Read(chunkSize)

		checkErr(err)

		c := signal.FFT(d)
		fp := fingerprint(c, fps)

		matched := (*pdb)[fp]

		songsMatched := make(map[string]struct{})

		// we usually match more than one chunk. process each
		for i := range matched {
			// we will only accept one match per song. this will be the first one in order to allow more matches next
			chunk := matched[i]
			if _, alreadyMatched := songsMatched[chunk.song]; alreadyMatched {
				continue
			}
			// _ = "breakpoint"

			// only accept matches in order and with fps.distance away from the last one.
			// NOTE: a problen with that algorithm is that it puts too much importance on the order;
			// a wrong match at the end of the song may stop other legitimate matches from being counted
			if lastMatched := lastMatches[chunk.song]; lastMatched.seq < chunk.seq && chunk.seq < lastMatched.seq+fps.distance {
				lastMatches[chunk.song] = chunk
				weight := fps.weight / (chunk.seq - lastMatched.seq)

				v1, hasScore := scores[chunk.song]

				// update the score for the song
				if hasScore {
					scores[chunk.song] = v1 + weight
				} else {
					scores[chunk.song] = weight
				}
				if highScore < scores[chunk.song] {
					highScore = scores[chunk.song]
					highName = chunk.song
				}
				// don't match the song anymore for that chunk
				songsMatched[chunk.song] = struct{}{}
			}
		}
	}
	sum := 0
	for _, value := range scores {
		sum += value
	}

	if sum == 0 {
		return 0, 0, ""
	}
	return 100 * highScore / sum, highScore, highName
}

// utility function to generate differnt settings
func generateSettings() []fpSettings {
	distances := []int{1000, 2000}
	weights := []int{500, 1000}
	dampers := []float64{2, 3}
	zoomers := []float64{2, 10}

	var pointsi [][]int
	pointsi = append(pointsi, []int{4})
	//pointsi = append(pointsi, []int{0, 1})

	fs := []fpSettings{}

	// generate lots of different settings
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

	return fs
}

// genarates a string representation (key) for a setting.
// the key does not have to unique and this allows settings to be explored in groups
func (fs *fpSettings) repr() string {
	return fmt.Sprintf("[a-%v,dist-%v,w-%v,damp-%v,z-%v", fs.algType, fs.distance, fs.weight, fs.damper, fs.zoomer)
}

// utility function to allow for experimenting with differnt settings
func tester(files []string, samplename string, expectedMatch string, freqStore freqdb, sr settingsResults) {
	fmt.Println("Testing ", samplename)

	fs := generateSettings()

	for _, fps := range fs {
		// build new printstore with the settings
		printsStore := make(printsdb)
		for _, f := range files {
			indexsong(f, &freqStore, &printsStore, fps)
		}
		//match
		pc, hs, hn := match(samplename, &printsStore, fps)
		if hn == expectedMatch && pc > 0 {
			fmt.Printf("CORRECT Score: %v(%v) %v - %v\n", pc, hs, hn, fps)
			sr[fps.repr()]++
		} else {
			fmt.Printf(":(  %v(%v) %v - %v\n", pc, hs, hn, fps)
		}
	}
}

func main() {
	// parse the command line arguments
	musicfiles := flag.String("db", "music", "path to where the reference samples are stored")
	samplename := flag.String("sample", "sample.wav", "name of the sample file to be tested")
	// explore diffrent algorithms/settings
	explore := flag.Bool("explore", false, "run in exploration mode")

	flag.Parse()
	freqStore := make(freqdb)

	files, _ := filepath.Glob(*musicfiles + "/*.wav")
	// build the freq db so that we can explore better algos
	for _, f := range files {
		fmt.Println("Parsing ", f)
		parsesong(f, &freqStore)
	}

	// simple evolutionary testing to get best settings
	if *explore {
		// TODO:  store the results in sqlite db for easier analytics
		sc := make(map[string]int)
		tester(files, "samples/04.wav", "music/04.wav", freqStore, sc)
		tester(files, "samples/04_long.wav", "music/04.wav", freqStore, sc)
		tester(files, "samples/04_short.wav", "music/04.wav", freqStore, sc)

		tester(files, "samples/01.wav", "music/01.wav", freqStore, sc)

		tester(files, "samples/08.wav", "music/08.wav", freqStore, sc)

		tester(files, "samples/14.wav", "music/14.wav", freqStore, sc)
		fmt.Printf("Test resultse: %v\n", sc)
	} else {
		// running on uknown sample
		fps := fpSettings{algType: "freq", distance: 1000, weight: 500, damper: 2, zoomer: 4, minFreq: 40, maxFeq: 300, pointsIgnore: []int{0, 4}, pointsCount: 5}
		// build new printstore with the settings
		printsStore := make(printsdb)
		for _, f := range files {
			indexsong(f, &freqStore, &printsStore, fps)
		}
		//match
		pc, hs, hn := match(*samplename, &printsStore, fps)
		fmt.Printf("Score: %v(%v) %v - %v\n", pc, hs, hn, fps)

	}
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
