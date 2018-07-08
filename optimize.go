/**
 * Filename: /Users/bao/code/allhic/allhic/optimize.go
 * Path: /Users/bao/code/allhic/allhic
 * Created Date: Tuesday, January 2nd 2018, 10:00:33 pm
 * Author: bao
 *
 * Copyright (c) 2018 Haibao Tang
 */

package allhic

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Optimizer runs the order-and-orientation procedure, given a clmfile
type Optimizer struct {
	REfile    string
	Clmfile   string
	RunGA     bool
	Resume    bool
	Seed      int64
	NPop      int
	NGen      int
	MutProb   float64
	CrossProb float64
}

// Run kicks off the Optimizer
func (r *Optimizer) Run() {
	clm := NewCLM(r.Clmfile, r.REfile)
	tourfile := RemoveExt(r.REfile) + ".tour"

	// Load tourfile if it exists
	if _, err := os.Stat(tourfile); r.Resume && err == nil {
		log.Noticef("Found existing tour file `%s`", tourfile)
		clm.parseTourFile(tourfile)
		// Rename the tour file
		backupTourFile := tourfile + ".sav"
		os.Rename(tourfile, backupTourFile)
		log.Noticef("Backup `%s` to `%s`", tourfile, backupTourFile)
	}

	shuffle := false // If one wants randomized initialization, set this to true
	clm.Activate(shuffle)

	// tourfile logs the intermediate configurations
	log.Noticef("Optimization history logged to `%s`", tourfile)
	fwtour, _ := os.Create(tourfile)
	defer fwtour.Close()
	clm.printTour(fwtour, clm.Tour, "INIT")

	if r.RunGA {
		for phase := 1; phase < 3; phase++ {
			clm.OptimizeOrdering(fwtour, r, phase)
		}
	}

	for phase := 1; ; phase++ {
		tag1, tag2 := clm.OptimizeOrientations(fwtour, phase)
		if tag1 == REJECT && tag2 == REJECT {
			log.Noticef("Terminating ... no more %v", ACCEPT)
			break
		}
	}
	clm.printTour(os.Stdout, clm.Tour, "FINAL")
	log.Notice("Success")
}

// OptimizeOrdering changes the ordering of contigs by Genetic Algorithm
func (r *CLM) OptimizeOrdering(fwtour *os.File, opt *Optimizer, phase int) {
	r.GARun(fwtour, opt, phase)
	// r.pruneTour()
}

// OptimizeOrientations changes the orientations of contigs by using heuristic flipping algorithms.
func (r *CLM) OptimizeOrientations(fwtour *os.File, phase int) (string, string) {
	tag1 := r.flipWhole()
	r.printTour(fwtour, r.Tour, fmt.Sprintf("FLIPWHOLE%d", phase))
	tag2 := r.flipOne()
	r.printTour(fwtour, r.Tour, fmt.Sprintf("FLIPONE%d", phase))
	return tag1, tag2
}

// parseTourFile parses tour file
// Only the last line is retained anc onverted into a Tour
func parseTourFile(filename string) []string {
	f, _ := os.Open(filename)
	log.Noticef("Parse tour file `%s`", filename)
	defer f.Close()

	reader := bufio.NewReader(f)
	var words []string
	for {
		row, err := reader.ReadString('\n')
		row = strings.TrimSpace(row)
		if row == "" && err == io.EOF {
			break
		}
		if row[0] == '>' { // header
			continue
		}
		words = strings.Split(row, " ")
	}
	return words
}

// prepareTour prepares a boilerplate for an empty tour
func (r *CLM) prepareTour() {
	r.Signs = make([]byte, len(r.Tigs))
	for _, tig := range r.Tigs {
		tig.IsActive = false
	}
}

// parseTourFile parses tour file
// Only the last line is retained anc onverted into a Tour
func (r *CLM) parseTourFile(filename string) {
	words := parseTourFile(filename)
	r.prepareTour()

	tigs := []Tig{}
	for _, word := range words {
		tigName, tigOrientation := word[:len(word)-1], word[len(word)-1]
		idx, ok := r.tigToIdx[tigName]
		if !ok {
			log.Errorf("Contig %s not found!", tigName)
			continue
		}
		tigs = append(tigs, Tig{
			Idx: idx,
		})
		r.Signs[idx] = tigOrientation
		r.Tigs[idx].IsActive = true
	}
	r.Tour.Tigs = tigs
	r.printTour(os.Stdout, r.Tour, "INIT")
}

// parseClustersFile parses clusters file
func (r *CLM) parseClustersFile(clustersfile string, group int) {
	recs := ReadCSVLines(clustersfile)
	r.prepareTour()

	rec := recs[group]
	names := strings.Split(rec[2], " ")
	tigs := []Tig{}
	for _, tigName := range names {
		idx, ok := r.tigToIdx[tigName]
		if !ok {
			log.Errorf("Contig %s not found!", tigName)
			continue
		}
		tigs = append(tigs, Tig{
			Idx: idx,
		})
		r.Signs[idx] = '+'
		r.Tigs[idx].IsActive = true
	}
	r.Tour.Tigs = tigs
	r.printTour(os.Stdout, r.Tour, "INIT")
}

// printTour logs the current tour to file
func (r *CLM) printTour(fwtour *os.File, tour Tour, label string) {
	fwtour.WriteString(">" + label + "\n")
	atoms := make([]string, tour.Len())
	for i := 0; i < tour.Len(); i++ {
		idx := tour.Tigs[i].Idx
		atoms[i] = r.Tigs[idx].Name + string(r.Signs[idx])
	}
	fwtour.WriteString(strings.Join(atoms, " ") + "\n")
}
