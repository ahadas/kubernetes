/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cpumanager

import (
	"fmt"

	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

///// Arik ///////
func (a *cpuAccumulator) maxSocket() int {
	socketIDs := a.details.Sockets().ToSlice()
	maxSocketID := socketIDs[0]
	maxSocketCpus := a.details.CPUsInSocket(maxSocketID).Size()
	for socket := range socketIDs[1:] {
		socketCpus := a.details.CPUsInSocket(socket).Size()
		if maxSocketCpus < socketCpus {
			maxSocketID = socket
			maxSocketCpus = socketCpus
		}
	}
	return maxSocketID
}

func (a *cpuAccumulator) socketToNumOfFreeCpus() map[int]int {
	mm := make(map[int]int)
	for _, socketID := range a.details.Sockets().ToSlice() {
		mm[socketID] = a.details.CPUsInSocket(socketID).Size()
	}
	return mm
}

type SocketAssignment struct {
	esocket int
	socket  int
	eval	SocketEvaluation
}

type SocketEvaluation struct {
	cpus   []int
	groups int
	free   int
}

func takeBy(topo *topology.CPUTopology, availableCPUs cpuset.CPUSet, sockets int, cores int, threads int) (cpuset.CPUSet, error) {
	acc := newCPUAccumulator(topo, availableCPUs, sockets * cores * threads)
	if acc.isSatisfied() {
		return acc.result, nil
	}
	if acc.isFailed() {
		return cpuset.NewCPUSet(), fmt.Errorf("not enough cpus available to satisfy request")
	}

	esocketToNumOfCpus := make(map[int]int)
	ecpusPerSocket := cores * threads
	for i := 0; i < sockets; i++ {
		esocketToNumOfCpus[i] = ecpusPerSocket
	}

	// socketToNumOfCpus := acc.socketToNumOfFreeCpus()
	var assignments []SocketAssignment

	for true {
		maxESocket, numOfCpus := maxSocket(esocketToNumOfCpus)
		if numOfCpus > threads {
			numOfCpus -= numOfCpus % threads
		}

		sockets := findSockets(acc, numOfCpus)
		maxSocket := sockets[0]
		maxEval := eval(acc.details, maxSocket, numOfCpus, threads)
		for _, socket := range sockets[1:] {
			eval := eval(acc.details, socket, numOfCpus, threads)
			if eval.groups > maxEval.groups ||
				(eval.groups == maxEval.groups && (eval.free > maxEval.free)) {
				maxSocket = socket
				maxEval = eval
			}
		}

		assignment := SocketAssignment{
			esocket:  maxESocket,
			socket:   maxSocket,
			eval:     maxEval,
		}
		assignments = append(assignments, assignment)

		acc.take(cpuset.NewCPUSet(maxEval.cpus...))
		if acc.isSatisfied() {
			break
		}
	}

	debug := make(map[int][]int)
	for _, assignment := range assignments {
		debug[assignment.esocket] = append(debug[assignment.esocket], assignment.eval.cpus...)
	}
	fmt.Printf("assignment: %v\n", debug)

	return acc.result, nil
}

func findSockets(acc *cpuAccumulator, numOfCpus int) []int {
	var moreOrEqual []int
	var less []int
	var maxLess int
	for k, v := range acc.socketToNumOfFreeCpus() {
		if numOfCpus <= v {
			moreOrEqual = append(moreOrEqual, k)
		} else {
			if v > maxLess {
				less = []int{k}
				maxLess = v
			} else if v == maxLess {
				less = append(less, k)
			}
		}
	}
	if len(moreOrEqual) > 0 {
		return moreOrEqual
	} else {
		// needs sorting to prioritize sockets that we already took cpus from
		return less
	}
}

func maxSocket(esocketToNumOfCpus map[int]int) (int, int) {
	var maxESocket int
	max := -1
	for k, v := range esocketToNumOfCpus {
		if max < v {
			maxESocket = k
			max = v
		}
	}
	return maxESocket, max
}

func eval(details topology.CPUDetails, socket int, needed int, groupSize int) SocketEvaluation {
	var allCpus []int
	groups := 0
	for needed > 0 {
		coreID := coreWithMaxFreeCpus(&details, socket)
		if coreID == -1 {
			break
		}
		coreCpus := cpuset.NewBuilder()
		for j, cpu := range details.CPUsInCore(coreID).ToSlice() {
			coreCpus.Add(cpu)
			needed -= 1
			if j + 1 == groupSize {
				groups += 1
				break
			}
		}
		details = details.KeepOnly(details.CPUs().Difference(coreCpus.Result()))
		allCpus = append(allCpus, coreCpus.Result().ToSlice()...)
	}
	return SocketEvaluation {
		cpus: allCpus,
		groups: groups,
		free: details.CPUsInSocket(socket).Size(),
	}
}

func coreWithMaxFreeCpus(details *topology.CPUDetails, socket int) int {
	coreIDs := details.CoresInSocket(socket).ToSlice()
	if len(coreIDs) == 0 {
		return -1
	}
	maxCore := coreIDs[0]
	maxCoreCpus := details.CPUsInCore(maxCore).Size()
	for _, core := range coreIDs[1:] {
		coreCpus := details.CPUsInCore(core).Size()
		if maxCoreCpus < coreCpus {
			maxCore = core
			maxCoreCpus = coreCpus
		}
	}
	return maxCore
}

