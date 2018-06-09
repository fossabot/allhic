/*
 * Filename: /Users/bao/code/allhic/graph.go
 * Path: /Users/bao/code/allhic
 * Created Date: Monday, June 4th 2018, 11:37:27 pm
 * Author: bao
 *
 * Copyright (c) 2018 Haibao Tang
 */

package allhic

import (
	"fmt"
	"math"
	"sort"
)

// Edge is between two nodes in a graph
type Edge struct {
	a, b   *Node
	weight float64
}

// updateGraph takes input a list of paths and fuse the nodes
func (r *Anchorer) updateGraph(G Graph) Graph {
	return G
}

// calculateEdges re-calibrates the edge weight
// Steps are:
// 1 - calculate the link density as links divided by the product of two contigs
// 2 - calculate the confidence as the weight divided by the second largest edge
func (r *Anchorer) makeConfidenceGraph(G Graph) Graph {
	twoLargest := map[*Node][]float64{}

	for a, nb := range G {
		first, second := 0.0, 0.0
		for b, score := range nb {
			// TODO: Limit > 1 links here
			score /= float64(a.path.length) * float64(b.path.length)
			if score > first {
				first, second = score, first
			} else if score <= first && score > second {
				second = score
			}
			G[a][b] = score
		}
		twoLargest[a] = []float64{first, second}
	}

	confidenceGraph := make(Graph)
	// Now a second pass to compute confidence
	for a, nb := range G {
		for b := range nb {
			secondLargest := getSecondLargest(twoLargest[a], twoLargest[b])
			G[a][b] /= secondLargest
			if G[a][b] > 1 {
				if _, aok := confidenceGraph[a]; aok {
					confidenceGraph[a][b] = G[a][b]
				} else {
					confidenceGraph[a] = map[*Node]float64{b: G[a][b]}
				}
			}
		}
	}
	return confidenceGraph
}

// Get the second largest number without sorting
// a, b are both 2-item arrays
func getSecondLargest(a, b []float64) float64 {
	A := append(a, b...)
	sort.Float64s(A)
	largest, secondLargest := A[3], A[2]
	// Some edge will appear twice in this list so need to remove it
	if largest == secondLargest && A[1] > 0 { // Is precision an issue here?
		secondLargest = A[1]
	}

	return secondLargest
}

// generatePathAndCycle makes new paths by merging the unique extensions
// in the graph
func (r *Anchorer) generatePathAndCycle(G Graph) {
	fmt.Println(G)
	visited := map[*Node]bool{}
	var isCycle bool
	nodeToPath := make(map[*Node]*Path)
	for a := range G {
		if _, ok := visited[a]; ok {
			continue
		}
		path1, path2 := []Edge{}, []Edge{}
		path1, isCycle = dfs(G, a, path1, visited, true)
		if isCycle {
			path1 = breakCycle(path1)
			printPath(path1, nodeToPath)
			continue
		}
		delete(visited, a)
		path2, _ = dfs(G, a, path2, visited, false)

		path1 = append(reversePath(path1), path2...)
		printPath(path1, nodeToPath)
	}
}

// printPath converts a single edge path into a node path
func printPath(path []Edge, nodeToPath map[*Node]*Path) {
	p := []string{}
	tag := ""
	// s := Path{}
	// orientation := 0
	for _, edge := range path {
		if edge.weight == 0 { // Sister edge
			if edge.a.end == 1 {
				tag = "-"
			} else {
				tag = ""
			}
			// Special care needed for reverse orientation
			for _, contig := range edge.a.path.contigs {
				p = append(p, tag+contig.name)
			}
		}
	}
	fmt.Println(path)
	fmt.Println(p)
}

// reversePath reverses a single edge path into its reverse direction
func reversePath(path []Edge) []Edge {
	ans := []Edge{}
	for i := len(path) - 1; i >= 0; i-- {
		ans = append(ans, Edge{
			path[i].b, path[i].a, path[i].weight,
		})
	}
	return ans
}

// breakCycle breaks a single edge path into two edge paths
func breakCycle(path []Edge) []Edge {
	minI, minWeight := 0, math.MaxFloat64
	for i, edge := range path {
		if edge.weight > 1 && edge.weight < minWeight {
			minI, minWeight = i, edge.weight
		}
	}
	return append(path[minI+1:], path[:minI]...)
}

// dfs visits the nodes in DFS order
// Return the path and if the path is a cycle
func dfs(G Graph, a *Node, path []Edge, visited map[*Node]bool, visitSister bool) ([]Edge, bool) {
	if _, ok := visited[a]; ok { // A cycle
		return path, true
	}

	visited[a] = true
	// Alternating between sister and non-sister edges
	if visitSister {
		path = append(path, Edge{
			a, a.sister, 0,
		})
		return dfs(G, a.sister, path, visited, false)
	}
	if nb, ok := G[a]; ok {
		var b *Node
		var weight float64
		for b, weight = range nb {
			break
		}
		path = append(path, Edge{
			a, b, weight,
		})
		return dfs(G, b, path, visited, true)
	}
	return path, false
}
