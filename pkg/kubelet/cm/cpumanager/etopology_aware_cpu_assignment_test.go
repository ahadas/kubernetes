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
		expResult     cpuset.CPUSet
	}{
		{
			"single socket HT, 1 socket free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			1,
			2,
			2,
			"",
			cpuset.NewCPUSet(0, 1, 4, 5),
		},
		{
			"single socket HT, 1 socket - 1 cpu",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, /*1,*/ 2, 3, 4, 5, 6, 7),
			1,
			2,
			2,
			"",
			cpuset.NewCPUSet(0, 2, 4, 6),
		},
		{
			"single socket HT, 1 socket - 1 cpu",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, /*1,*/ 2, 3, 4, 5, 6, 7),
			1,
			2,
			1,
			"",
			cpuset.NewCPUSet(0, 2),
		},
		{
			"single socket HT, 1 socket - 1 cpu",
			topoSingleSocketHT,
			cpuset.NewCPUSet(/*0, 1,*/ 2, /*3, 4,*/ 5, 6, 7),
			1,
			2,
			2,
			"",
			cpuset.NewCPUSet(2, 6, 5, 7),
		},
		{
			"dual TBD",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11),
			1,
			2,
			2,
			"",
			cpuset.NewCPUSet(0, 6, 2, 8),
		},
		{
			"dual TBD2",
			topoDualSocketHT,
			cpuset.NewCPUSet(/*0, 1,*/ 2, 3, 4, 5,/* 6, 7,*/ 8, 9,/* 10,*/ 11),
			1,
			2,
			2,
			"",
			cpuset.NewCPUSet(3, 9, 5, 11),
		},
		{
			"dual TBD3",
			topoDualSocketHT,
			cpuset.NewCPUSet(/*0, 1,*/ 2, 3, 4, 5,/* 6, 7,*/ 8, 9,/* 10,*/ 11),
			2,
			1,
			2,
			"",
			cpuset.NewCPUSet(3, 9, 2, 8),
		},
		{
			"dual TBD4",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, /*1, 2,*/ 3, 4, 5, 6, /*7, 8,*/ 9, 10, 11),
			2,
			2,
			2,
			"",
			cpuset.NewCPUSet(0, 6, 4, 10, 3, 9, 5, 11),
		},
		{
			"dual TBD5",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, /*2, 3,*/ 4, 5, 6, /*7, 8,*/ 9, 10, 11),
			2,
			2,
			2,
			"",
			cpuset.NewCPUSet(0, 6, 4, 10, 5, 11, 1, 9),
		},
		{
			"dual TBD6",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, /*1, 2, */3, 4, 5, /*6, 7,*/ 8, 9, 10, 11),
			1,
			2,
			2,
			"",
			cpuset.NewCPUSet(3, 9, 5, 11),
		},
		{
			"dual TBD7",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, /*2,*/ 3, 4, 5, 6, /*7, 8, 9,*/ 10,/* 11*/),
			2,
			1,
			2,
			"",
			cpuset.NewCPUSet(0, 6, 4, 10),
		},
		{
			"dual TBD8",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, /*1,*/ 2, /*3, 4, 5, 6, 7, */8, /*9, 10,*/ 11),
			2,
			1,
			2,
			"",
			cpuset.NewCPUSet(2, 8, 0, 11),
		},
		{
			"dual TBD9",
			topoDualSocketHT,
			cpuset.NewCPUSet(/*0, */1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11),
			1,
			1,
			2,
			"",
			cpuset.NewCPUSet(1, 7),
		},
		{
			"dual TBD10",
			topoDualSocketHT,
			cpuset.NewCPUSet(/*0, */1, 2, /*3,*/ 4, 5, 6, 7, 8, /*9,*/ 10, 11),
			1,
			1,
			2,
			"",
			cpuset.NewCPUSet(2, 8),
		},
	}

	for _, tc := range testCases {
		result, err := takeBy(tc.topo, tc.availableCPUs, tc.sockets, tc.cores, tc.threads)
		if tc.expErr != "" && err.Error() != tc.expErr {
			t.Errorf("expected error to be [%v] but it was [%v] in test \"%s\"", tc.expErr, err, tc.description)
		}
		if !result.Equals(tc.expResult) {
			t.Errorf("expected result [%s] to equal [%s] in test \"%s\"", result, tc.expResult, tc.description)
		}
	}
}

