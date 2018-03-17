/**
 * Filename: /Users/bao/code/allhic/distribution.go
 * Path: /Users/bao/code/allhic
 * Created Date: Wednesday, March 7th 2018, 1:56:45 pm
 * Author: bao
 *
 * Copyright (c) 2018 Haibao Tang
 */

package allhic

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

var b = [...]uint{0x2, 0xC, 0xF0, 0xFF00, 0xFFFF0000}
var s = [...]uint{1, 2, 4, 8, 16}

// Distribution processes the distribution step
type Distribution struct {
	Bamfile     string
	maxLinkDist int
	binStarts   []int
	links       []int
}

// uintLog2 calculates the integer log2 of a number
func uintLog2(x2 uint) uint {
	var answer uint
	if (x2 & b[4]) != 0 {
		x2 >>= s[4]
		answer |= s[4]
	}
	if (x2 & b[3]) != 0 {
		x2 >>= s[3]
		answer |= s[3]
	}
	if (x2 & b[2]) != 0 {
		x2 >>= s[2]
		answer |= s[2]
	}
	if (x2 & b[1]) != 0 {
		x2 >>= s[1]
		answer |= s[1]
	}
	if (x2 & b[0]) != 0 {
		x2 >>= s[0]
		answer |= s[0]
	}
	return answer
}

// uintLog2Frac calculates the fractional part of log2 of a number
func uintLog2Frac(x float64) uint {
	var l uint
	x2 := x * x
	if x2 > 2 {
		x2 /= 2
		l += 8
	}
	x2 *= x2
	if x2 > 2 {
		x2 /= 2
		l += 4
	}
	x2 *= x2
	if x2 > 2 {
		x2 /= 2
		l += 2
	}
	x2 *= x2
	if x2 > 2 {
		x2 /= 2
		l++
	}
	return l
}

// LinkBin takes a link distance and convert to a binID
func (r *Distribution) LinkBin(dist int) int {
	distOverMin := dist / MinLinkDist
	log2i := uintLog2(uint(distOverMin))
	log2f := uintLog2Frac(float64(dist) / float64(int(MinLinkDist)<<log2i))
	return int(16*log2i + log2f)
}

// BinSize returns the size of each bin
func (r *Distribution) BinSize(i int) int {
	return r.binStarts[i+1] - r.binStarts[i]
}

// Run calls the distribution steps
func (r *Distribution) Run() {
	file, _ := os.Open(r.Bamfile)
	log.Noticef("Parse dist file `%s`", r.Bamfile)
	reader := bufio.NewReader(file)
	for {
		row, err := reader.ReadString('\n')
		row = strings.TrimSpace(row)
		if row == "" && err == io.EOF {
			break
		}
		words := strings.Split(row, "\t")
		for _, link := range strings.Split(words[1], ",") {
			ll, _ := strconv.Atoi(link)
			if ll >= MinLinkDist {
				r.links = append(r.links, ll)
			}
		}
	}
	log.Noticef("Imported %d intra-contig links", len(r.links))

	r.Makebins()
}

// Makebins makes geometric bins and count links that fall in each bin
// This heavily borrows the method form LACHESIS
// https://github.com/shendurelab/LACHESIS/blob/master/src/LinkSizeDistribution.cc
func (r *Distribution) Makebins() {
	// Step 1: make geometrically sized bins
	// We fit 16 bins into each power of 2
	linkRange := math.Log2(float64(MaxLinkDist) / float64(MinLinkDist))
	nBins := int(math.Ceil(linkRange * 16))

	r.maxLinkDist = MinInt
	for _, link := range r.links {
		if link > r.maxLinkDist {
			r.maxLinkDist = link
		}
	}

	for i := 0; 16*i <= nBins; i++ {
		jpower := 1.0
		for j := 0; j < 16 && 16*i+j <= nBins; j++ {
			binStart := MinLinkDist << uint(i)
			r.binStarts = append(r.binStarts, int(float64(binStart)*jpower))
			jpower *= GeometricBinSize
		}
	}
	// fmt.Println(r.binStarts)

	// Step 2: calculate assayable sequence length
	// TODO: this is just rice chromosome array for testing, need to change to real contig sizes
	// based on BAM file header
	var contigLens = [...]int{43270923, 35937250, 36413819, 35502694, 29958434, 31248787,
		29697621, 28443022, 23012720, 23207287, 29021106, 27531856, 633585, 592136}
	// Find the length of assayable intra-contig sequence in each bin
	intraContigLinkRange := math.Log2(float64(r.maxLinkDist) / float64(MinLinkDist))
	nIntraContigBins := int(math.Ceil(intraContigLinkRange * 16))

	binNorms := make([]int, nIntraContigBins)
	nLinks := make([]int, nIntraContigBins)
	for _, contiglen := range contigLens {
		for j := 0; j < nIntraContigBins; j++ {
			r := contiglen - r.binStarts[j]
			if r < 0 {
				break
			}
			binNorms[j] += r
		}
	}
	// fmt.Println(binNorms)

	// Step 3: loop through all links and tabulate the counts
	for _, link := range r.links {
		bin := r.LinkBin(link)
		nLinks[bin]++
	}
	// fmt.Println(nLinks)

	// Step 4: normalize to calculate link density
	linkDensity := make([]float64, nBins)
	for i := 0; i < nIntraContigBins; i++ {
		linkDensity[i] = float64(nLinks[i]) * float64(BinNorm) / float64(binNorms[i]) / float64(r.BinSize(i))
		fmt.Println(i, nLinks[i], binNorms[i], r.binStarts[i], r.BinSize(i), linkDensity[i])
	}
	fmt.Println(linkDensity)

	// Step 5: assume the distribution approximates 1/x for large x
	topBin := nIntraContigBins - 1
	nTopLinks := 0
	nTopLinksNeeded := len(r.links) / 100
	for ; nTopLinks < nTopLinksNeeded; topBin-- {
		nTopLinks += nLinks[topBin]
	}

	avgLinkDensity := 0.0
	for i := topBin; i < nIntraContigBins; i++ {
		avgLinkDensity += linkDensity[i] * float64(r.BinSize(i)) / BinNorm
	}
	avgLinkDensity /= float64(nIntraContigBins - topBin)

	for i := nIntraContigBins - 1; i < nBins; i++ {
		linkDensity[i] = avgLinkDensity * float64(r.BinSize(i)) / BinNorm
	}
	fmt.Println(linkDensity)
}
