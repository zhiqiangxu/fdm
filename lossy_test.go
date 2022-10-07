package fdm

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type testHeader struct {
	H  uint64
	P  common.Hash
	CS map[int]int
}

func (th *testHeader) Hash() (h common.Hash) {
	sha := crypto.NewKeccakState()
	thBytes, _ := json.Marshal(th)
	err := rlp.Encode(sha, thBytes)
	if err != nil {
		panic(err)
	}
	sha.Read(h[:])
	return
}

func (h *testHeader) ParentHash() common.Hash {
	return h.P
}

func (h *testHeader) Height() uint64 {
	return h.H
}

func TestLossyForkDetector(t *testing.T) {

	state := map[int]int{}
	reorgFunc := func(old, new []*Snapshot[map[int]int, common.Hash]) {
		for _, snapshot := range old {
			for k, v := range snapshot.preState {
				state[k] = v
			}
		}
		for _, snapshot := range new {
			for k, v := range snapshot.afterState {
				state[k] = v
			}
		}
	}
	snaps := make(map[common.Hash]*Snapshot[map[int]int, common.Hash])
	snapFunc := func(hash common.Hash) *Snapshot[map[int]int, common.Hash] {
		snapshot := *snaps[hash]
		snapshot.preState = nil
		return &snapshot
	}
	preStateFunc := func(afterState map[int]int) map[int]int {
		m := make(map[int]int)
		for k := range afterState {
			m[k] = state[k]
		}
		return m
	}
	fd := New(100, reorgFunc, snapFunc, preStateFunc)

	header1 := &testHeader{H: 1, CS: map[int]int{1: 1}}
	header2 := &testHeader{H: 2, P: header1.Hash(), CS: map[int]int{1: 2}}
	header2f := &testHeader{H: 2, P: header1.Hash(), CS: map[int]int{1: 3}}
	header3f := &testHeader{H: 3, P: header2f.Hash(), CS: map[int]int{1: 4}}
	snap1 := &Snapshot[map[int]int, common.Hash]{header: header1, hash: header1.Hash(), afterState: header1.CS}
	snap2 := &Snapshot[map[int]int, common.Hash]{header: header2, hash: header2.Hash(), preState: header1.CS, afterState: header2.CS}
	snap2f := &Snapshot[map[int]int, common.Hash]{header: header2f, hash: header2f.Hash(), preState: header1.CS, afterState: header2f.CS}
	snap3f := &Snapshot[map[int]int, common.Hash]{header: header3f, hash: header3f.Hash(), preState: header2f.CS, afterState: header3f.CS}
	snaps[snap1.hash] = snap1
	snaps[snap2.hash] = snap2
	snaps[snap2f.hash] = snap2f
	snaps[snap3f.hash] = snap3f
	fd.Submit(snap1)
	fd.Submit(snap2)
	fd.Submit(snap3f)

	if state[1] != 4 {
		t.Fatal()
	}
	if fd.snapshots[0] != nil {
		t.Fatal()
	}
	if fd.offset != 3 {
		t.Fatal(fd.offset)
	}

	type testcase struct {
		i        int
		snapshot *Snapshot[map[int]int, common.Hash]
	}
	cases := []testcase{
		{i: 1, snapshot: snap1},
		{i: 2, snapshot: snap2f},
		{i: 3, snapshot: snap3f},
	}
	for _, c := range cases {
		if fd.snapshots[c.i].hash != c.snapshot.hash {
			t.Fatal()
		}
		if fd.snapshots[c.i].header.Hash() != c.snapshot.hash {
			t.Fatal()
		}
		if c.snapshot.header.Hash() != c.snapshot.hash {
			t.Fatal()
		}

		if !reflect.DeepEqual(fd.snapshots[c.i].preState, c.snapshot.preState) {
			t.Fatal()
		}
		if !reflect.DeepEqual(fd.snapshots[c.i].afterState, c.snapshot.afterState) {
			t.Fatal(c.i, fd.snapshots[c.i].afterState, c.snapshot.afterState)
		}
	}

}
