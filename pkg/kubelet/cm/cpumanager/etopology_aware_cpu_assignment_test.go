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
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

func TestAssignment(t *testing.T) {
	testCases := []struct {
		description   string
		topo          *topology.CPUTopology
		availableCPUs cpuset.CPUSet
		sockets       int
		cores         int
		threads       int
		expErr        string
		expResult     []int
	}{
		{
			"single socket HT, emulated topology 1:2:2, 1 socket free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			1,
			2,
			2,
			"",
			[]int{0, 4, 1, 5},
		},
		{
			"single socket HT, emulated topology 1:2:2, 1 socket - 1 cpu",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 2, 3, 4, 5, 6, 7),
			1,
			2,
			2,
			"",
			[]int{0, 4, 2, 6},
		},
		{
			"single socket HT, emulated topology 1:2:1, 1 socket - 1 cpu",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 2, 3, 4, 5, 6, 7),
			1,
			2,
			1,
			"",
			[]int{0, 2},
		},
		{
			"single socket HT, emulated topology 1:2:2, 1 full core, 2 partial cores",
			topoSingleSocketHT,
			cpuset.NewCPUSet(2, 5, 6, 7),
			1,
			2,
			2,
			"",
			[]int{2, 6, 5, 7},
		},
		{
			"dual socket HT, emulated topology 1:2:2, full sockets",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11),
			1,
			2,
			2,
			"",
			[]int{0, 6, 2, 8},
		},
		{
			"dual socket HT, emulated topology 1:2:2, second socket supports more groups",
			topoDualSocketHT,
			cpuset.NewCPUSet(2, 3, 4, 5, 8, 9, 11),
			1,
			2,
			2,
			"",
			[]int{3, 9, 5, 11},
		},
		{
			"dual socket HT, emulated topology 1:2:2, second socket support more groups",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 3, 4, 5, 8, 9, 10, 11),
			1,
			2,
			2,
			"",
			[]int{3, 9, 5, 11},
		},
		{
			"dual socket HT, 2 emulated sockets, 2nd socket is more free",
			topoDualSocketHT,
			cpuset.NewCPUSet(2, 3, 4, 5, 8, 9, 11),
			2,
			1,
			2,
			"",
			[]int{3, 9, 2, 8},
		},
		{
			"dual socket HT, emulated topology 1:1:2, same groups 2nd socket is more free",
			topoDualSocketHT,
			cpuset.NewCPUSet(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11),
			1,
			1,
			2,
			"",
			[]int{1, 7},
		},
		{
			"dual socket HT, emulated topology 1:1:2, 2nd socket - more groups, less cpus",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 4, 5, 8, 11),
			1,
			1,
			2,
			"",
			[]int{5, 11},
		},
		{
			"dual socket HT, 2 emulated sockets, exact match",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 3, 4, 5, 6, 9, 10, 11),
			2,
			2,
			2,
			"",
			[]int{0, 6, 4, 10, 3, 9, 5, 11},
		},
		{
			"dual socket HT, 2 emulated sockets, not exact match",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 4, 5, 6, 9, 10, 11),
			2,
			2,
			2,
			"",
			[]int{0, 6, 4, 10, 5, 11, 1, 9},
		},
		{
			"dual socket HT, 2 emulated sockets, mapped to 1st socket",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 3, 4, 5, 6, 10),
			2,
			1,
			2,
			"",
			[]int{0, 6, 4, 10},
		},
		{
			"dual socket HT, emulated topology 2:1:2, emulated socket is split",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 2, 8, 11),
			2,
			1,
			2,
			"",
			[]int{2, 8, 0, 11},
		},
	}

	for _, tc := range testCases {
		result, err := takeByETopology(tc.topo, tc.availableCPUs, tc.sockets, tc.cores, tc.threads)
		if tc.expErr != "" && err.Error() != tc.expErr {
			t.Errorf("expected error to be [%v] but it was [%v] in test \"%s\"", tc.expErr, err, tc.description)
		}
		if !reflect.DeepEqual(result, tc.expResult) {
			t.Errorf("expected result %v to equal %v in test \"%s\"", result, tc.expResult, tc.description)
		}
	}
}

