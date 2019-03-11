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
	"sort"

	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

type cpuAccumulator2 struct {
	acc         *cpuAccumulator
	esockets    int
	ecores      int
	ethreads    int
	assignments map[int][]int
}

func newCPUAccumulator2(topo *topology.CPUTopology, availableCPUs cpuset.CPUSet, sockets int, cores int, threads int) *cpuAccumulator2 {
	accumulator := &cpuAccumulator{
		topo:          topo,
		details:       topo.CPUDetails.KeepOnly(availableCPUs),
		numCPUsNeeded: sockets * cores * threads,
		result:        cpuset.NewCPUSet(),
	}
	return &cpuAccumulator2 {
		acc:           accumulator,
		esockets:      sockets,
		ecores:        cores,
		ethreads:      threads,
		assignments:   make(map[int][]int),
	}
}

func (a *cpuAccumulator2) isSatisfied() bool {
	return a.acc.isSatisfied()
}

func (a *cpuAccumulator2) isFailed() bool {
	return a.acc.isFailed()
}

func (a *cpuAccumulator2) maxSocket() int {
	socketIDs := a.acc.details.Sockets().ToSlice()
	maxSocketID := socketIDs[0]
	maxSocketCpus := a.acc.details.CPUsInSocket(maxSocketID).Size()
	for socket := range socketIDs[1:] {
		socketCpus := a.acc.details.CPUsInSocket(socket).Size()
		if maxSocketCpus < socketCpus {
			maxSocketID = socket
			maxSocketCpus = socketCpus
		}
	}
	return maxSocketID
}

func (a *cpuAccumulator2) socketToNumOfFreeCpus() map[int]int {
	mm := make(map[int]int)
	for _, socketID := range a.acc.details.Sockets().ToSlice() {
		mm[socketID] = a.acc.details.CPUsInSocket(socketID).Size()
	}
	return mm
}

func (a *cpuAccumulator2) emulatedCpusPerSocket() int {
	return a.ecores * a.ethreads
}

func (a *cpuAccumulator2) Add(esocket int, cpus []int) {
	a.assignments[esocket] = append(a.assignments[esocket], cpus...)
	a.acc.take(cpuset.NewCPUSet(cpus...))
}

func (a *cpuAccumulator2) Result() []int {
	result := []int{}
	for i := 0; i < a.esockets; i++ {
		result = append(result, a.assignments[i]...)
	}
	//return cpuset.NewCPUSet(result...)
	return result
}

type SocketEvaluation struct {
	cpus   []int
	groups int
	free   int
}

func takeByETopology(topo *topology.CPUTopology, availableCPUs cpuset.CPUSet, sockets int, cores int, threads int) ([]int, error) {
	acc := newCPUAccumulator2(topo, availableCPUs, sockets, cores, threads)
	if acc.isSatisfied() {
		return acc.Result(), nil
	}
	if acc.isFailed() {
		// return cpuset.NewCPUSet(), fmt.Errorf("not enough cpus available to satisfy request")
		return []int{}, fmt.Errorf("not enough cpus available to satisfy request")
	}

	esocketToNumOfCpus := make(map[int]int)
	for i := 0; i < sockets; i++ {
		esocketToNumOfCpus[i] = acc.emulatedCpusPerSocket()
	}

	for true {
		maxESocket, numOfCpus := maxSocket(esocketToNumOfCpus)
		if numOfCpus > threads {
			numOfCpus -= numOfCpus % threads
		}

		sockets := findSockets(acc, numOfCpus)
		maxSocket := sockets[0]
		maxEval := eval(acc.acc.details, maxSocket, numOfCpus, threads)
		for _, socket := range sockets[1:] {
			eval := eval(acc.acc.details, socket, numOfCpus, threads)
			if eval.groups > maxEval.groups ||
				(eval.groups == maxEval.groups && (eval.free > maxEval.free)) {
				maxSocket = socket
				maxEval = eval
			}
		}
		esocketToNumOfCpus[maxESocket] -= len(maxEval.cpus)
		acc.Add(maxESocket, maxEval.cpus)
		if acc.isSatisfied() {
			break
		}
	}

	return acc.Result(), nil
}

func findSockets(acc *cpuAccumulator2, numOfCpus int) []int {
	moreOrEqual := []int{}
	less := []int{}
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
		sort.Ints(moreOrEqual)
		return moreOrEqual
	} else {
		// needs sorting to prioritize sockets that we already took cpus from
		sort.Ints(less)
		return less
	}
}

func maxSocket(esocketToNumOfCpus map[int]int) (int, int) {
	maxESocket := -1
	max := -1
	for k, v := range esocketToNumOfCpus {
		if max < v {
			maxESocket = k
			max = v
		} else if max == v && k < maxESocket {
			maxESocket = k
		}

	}
	return maxESocket, max
}

func eval(details topology.CPUDetails, socket int, needed int, groupSize int) SocketEvaluation {
	allCpus := []int{}
	groups := 0
	for needed > 0 {
		coreID := nextCore(&details, socket)
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
		free: details.CPUsInSocket(socket).Size(),// - len(allCpus),
	}
}

// Next core in the socket to fill
func nextCore(details *topology.CPUDetails, socket int) int {
	cores := details.CoresInSocket(socket)
	if cores.Size() == 0 {
		return -1
	}
        coreIDs := cores.ToSlice()
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

