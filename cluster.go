/*
 * Filename: /Users/bao/code/allhic/cluster.go
 * Path: /Users/bao/code/allhic
 * Created Date: Saturday, April 21st 2018, 4:09:48 pm
 * Author: bao
 *
 * Copyright (c) 2018 Haibao Tang
 */

package allhic

import (
	"fmt"
	"sort"
	"strings"
)

// merge is a generic type that stores the merges
type merge struct {
	a     int
	b     int
	score float64
}

// Clusters stores all the contig IDs per cluster
type Clusters map[int][]int

// clusterLen helps sorting based on the length of a cluster
type clusterLen struct {
	cID    int
	length int
}

// Cluster performs the hierarchical clustering
// This function is a re-implementation of the AHClustering() function in LACHESIS
func (r *Partitioner) Cluster() {

	// TODO: Skip contigs that are too small or irrelevant
	// LACHESIS also skips contigs that are thought to be centromeric
	G := r.matrix
	nclusters := r.K
	N := len(r.contigs)

	// Auxiliary data structures to facilitate cluster merging
	clusterID := make([]int, N)
	clusterSize := make([]int, 2*N)
	clusterExists := make([]bool, 2*N)
	nonSingletonClusters := 0

	nContigsSkipped := 0
	// Initially all contigs in their own cluster
	for i, contig := range r.contigs {
		if contig.skip {
			clusterID[i] = -1
			nContigsSkipped++
			continue
		}
		clusterID[i] = i
		clusterSize[i] = 1
		clusterExists[i] = true
	}
	nNonSkipped := N - nContigsSkipped
	if nNonSkipped == 0 {
		log.Noticef("There are no informative contigts for clustering. Contigs are either SHORT or REPETITVE.")
	}
	log.Noticef("Clustering starts with %d (%d informative) contigs with target of %d clusters",
		N, nNonSkipped, nclusters)

	// mergeScores has all possible pairwise merge scores
	// We keep a slice containing all potential merges. Best to use a priority queue.
	// The original LACHESIS implementation used a C++ multimap with its use similar
	// to a priority queue, however, the performance benefit is not obvious since we
	// need to perform updates to all merges (remove old merges and insert new merges)
	merges := []*merge{}

	for i := 0; i < N; i++ {
		if r.contigs[i].skip {
			continue
		}
		for j := i + 1; j < N; j++ {
			if !r.contigs[j].skip && G[i][j] > MinAvgLinkage {
				merges = append(merges, &merge{
					a:     i,
					b:     j,
					score: float64(G[i][j]),
				})
			}
		}
	}

	nMerges := 0
	// The core hierarchical clustering
	for {
		if len(merges) == 0 {
			log.Notice("No more merges to do since the queue is empty")
			break
		}
		bestMerge := merges[0]
		// Step 1. Find the pairs of the clusters with the highest merge score
		for _, merge := range merges {
			if merge.score > bestMerge.score {
				bestMerge = merge
			}
		}

		// Step 2. Merge the contig pair
		newClusterID := N + nMerges

		clusterExists[bestMerge.a] = false
		clusterExists[bestMerge.b] = false
		clusterExists[newClusterID] = true
		clusterSize[newClusterID] = clusterSize[bestMerge.a] + clusterSize[bestMerge.b]
		if bestMerge.a < N {
			nonSingletonClusters++
		}
		if bestMerge.b < N {
			nonSingletonClusters++
		}
		nonSingletonClusters--

		var newCluster []int
		for i := 0; i < N; i++ {
			if clusterID[i] == bestMerge.a || clusterID[i] == bestMerge.b {
				clusterID[i] = newClusterID
				newCluster = append(newCluster, i)
			}
		}

		nMerges++

		// Step 3. Calculate new score entries for the new cluster
		// Remove all used clusters
		newMerges := []*merge{}
		for _, merge := range merges {
			if clusterExists[merge.a] && clusterExists[merge.b] {
				newMerges = append(newMerges, merge)
			} else {
				// fmt.Println("Ignore", merge)
			}
		}

		// Add all merges with the new cluster
		totalLinkageByCluster := make([]int64, 2*N)
		for i := 0; i < N; i++ {
			cID := clusterID[i]
			if cID == newClusterID { // No need to calculate linkages within cluster
				continue
			}
			if cID == -1 { // This happens if contig is skipped
				continue
			}
			for _, j := range newCluster {
				totalLinkageByCluster[cID] += G[i][j]
			}
		}

		for i := 0; i < 2*N; i++ {
			if totalLinkageByCluster[i] <= 0 {
				continue
			}
			if !clusterExists[i] {
				log.Errorf("Cluster %d does not exist", i)
			}
			// Average linkage
			avgLinkage := float64(totalLinkageByCluster[i]) / float64(clusterSize[i]) /
				float64(clusterSize[newClusterID])

			if avgLinkage < MinAvgLinkage {
				continue
			}

			p := &merge{
				a:     min(i, newClusterID),
				b:     max(i, newClusterID),
				score: avgLinkage,
			}
			newMerges = append(newMerges, p)
			// fmt.Println("Insert", p)
		}

		// Analyze the current clusters if enough merges occurred
		if nMerges > N/2 && nonSingletonClusters <= nclusters {
			if nonSingletonClusters == nclusters {
				log.Noticef("%d merges made so far; this leaves %d clusters, and so we'r done!",
					nMerges, nonSingletonClusters)
				break
			}
		}
		log.Noticef("Merge #%d: Clusters\t%d + %d -> %d, Linkage = %g",
			nMerges, bestMerge.a, bestMerge.b, newClusterID, bestMerge.score)

		merges = newMerges
	}

	r.setClusters(clusterID)
}

// setClusters assigns contigs into clusters per clusterID
// When there are contigs skipped (either SHORT or REPETITIVE), we assign the skipped contigs based
// on how well they match non-skipped contigs. Each skipped contig needs to link to a cluster with
// at least NonInformativeRatio times as many links as any other cluster.
func (r *Partitioner) setClusters(clusterID []int) {
	clusters := Clusters{}
	for i, cID := range clusterID {
		if i == cID {
			continue
		}
		clusters[cID] = append(clusters[cID], i)
	}
	r.clusters = clusters

	if !(NonInformativeRatio == 0 || NonInformativeRatio > 1) {
		log.Errorf("NonInformative Ratio needs to either 0 or > 1")
	}

	// Now try to recover previously skipped contigs
	if NonInformativeRatio == 0 {
		r.sortClusters()
		return
	}

	N := len(r.contigs)
	// NonInformativeRatio > 1
	// Loop through all skipped contigs. Determine the cluster with largest average linkage.
	for i := 0; i < N; i++ {
		if clusterID[i] != -1 {
			continue
		}
	}

	r.sortClusters()
	fmt.Println(r.clusters)
	return
}

// findClusterLinkages
func (r *Partitioner) findClusterLinkage(cID int) {

}

// sortClusters reorder the cluster by total length
func (r *Partitioner) sortClusters() {
	clusterLens := []*clusterLen{}
	for cID, cl := range r.clusters {
		c := &clusterLen{
			cID:    cID,
			length: 0,
		}
		for _, ci := range cl {
			c.length += r.contigs[ci].length
		}
		clusterLens = append(clusterLens, c)
	}

	// Reorder the clusters based on the size
	sort.Slice(clusterLens, func(i, j int) bool {
		return clusterLens[i].length > clusterLens[j].length
	})

	newClusters := Clusters{}
	for i, cl := range clusterLens {
		newClusters[i] = r.clusters[cl.cID]
	}
	r.clusters = newClusters
}

// printClusters shows the contents of the clusters
func (r *Partitioner) printClusters() {
	for _, ids := range r.clusters {
		names := make([]string, len(ids))
		for i, id := range ids {
			names[i] = r.contigs[id].name
		}
		sort.Strings(names)
		fmt.Println(len(names), strings.Join(names, ","))
	}
}
