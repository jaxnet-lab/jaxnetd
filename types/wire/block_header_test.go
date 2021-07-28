// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package wire

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"fmt"
	rand2 "math/rand"
	"testing"
	"time"
)

func TestBVersion_ExpansionMade(t *testing.T) {
	tests := []struct {
		name string
		bv   BVersion
		want bool
	}{
		{
			bv:   NewBVersion(1),
			want: false,
		},
		{
			bv:   NewBVersion(1).SetExpansionMade(),
			want: true,
		},
		{
			bv:   NewBVersion(100500).SetExpansionMade(),
			want: true,
		}, {
			bv:   NewBVersion(100500).SetExpansionMade().UnsetExpansionMade(),
			want: false,
		},
		{
			bv:   NewBVersion(42).SetExpansionApproved(),
			want: false,
		},
		{
			bv:   NewBVersion(42).SetExpansionApproved().SetExpansionMade(),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bv.ExpansionMade(); got != tt.want {
				t.Logf("%032x", tt.bv)
				t.Logf("%b", tt.bv)
				t.Errorf("ExpansionMade() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBVersion_ExpansionApproved(t *testing.T) {
	tests := []struct {
		name string
		bv   BVersion
		want bool
	}{
		{
			bv:   NewBVersion(1),
			want: false,
		},
		{
			bv:   NewBVersion(1).SetExpansionApproved(),
			want: true,
		},
		{
			bv: NewBVersion(1).
				SetExpansionApproved().
				UnsetExpansionApproved(),
			want: false,
		},
		{
			bv:   NewBVersion(100500).SetExpansionApproved(),
			want: true,
		},
		{
			bv:   NewBVersion(42).SetExpansionApproved().SetExpansionMade(),
			want: true,
		},
		{
			bv:   NewBVersion(42).SetExpansionMade(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bv.ExpansionApproved(); got != tt.want {
				t.Logf("%032x", tt.bv)
				t.Logf("%b", tt.bv)
				t.Errorf("ExpansionMade() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTreeEncoding(t *testing.T) {
	bh := BeaconHeader{}

	i := 0
	for i < 1000 {
		hashesSize := int(rand2.Int31n(100000) + 1)
		hashes := make([]byte, hashesSize)
		rand.Read(hashes[:])

		codingSize := int(rand2.Int31n(100000) + 1)
		coding := make([]byte, codingSize)
		rand.Read(coding[:])

		bitsSize := rand2.Uint32() + 1
		bh.SetMergedMiningTreeCodingProof(hashes, coding, bitsSize)

		hashes2, coding2, bitsSize2 := bh.MergedMiningTreeCodingProof()
		if bytes.Compare(hashes, hashes2) != 0 {
			t.Error("Hashes not equal at ", i)
		}

		if bytes.Compare(coding, coding2) != 0 {
			t.Error("Coding not equal at ", i)
		}
		if bitsSize != bitsSize2 {
			t.Error("Bits not equal at ", i)
		}
		fmt.Printf("%d. Test ok. Hash %d, Coding %d, %d\n", i, hashesSize, codingSize, bitsSize)
		i++
	}
}

func TestShardHeaderEncoding(t *testing.T) {
	sh := ShardHeader{}
	sh.timestamp = time.Now()
	sh.bCHeader = BeaconHeader{
		bits: 1,
	}
	sh.bits = 3
	sh.mergeMiningNumber = 4
	rand.Read(sh.merkleRoot[:])
	rand.Read(sh.prevBlock[:])
	sh.mergeMiningNumber = 5

	hashes := make([]byte, 400)
	coding := make([]byte, 300)
	var bits uint32 = 222

	sh.bCHeader.SetMergedMiningTreeCodingProof(hashes, coding, bits)

	var b bytes.Buffer
	wr := bufio.NewWriter(&b)
	if err := WriteShardBlockHeader(wr, &sh); err != nil {
		t.Error(err)
		return
	}
	wr.Flush()

	fmt.Printf("%d %d\n", wr.Size(), wr.Available())
	sh2 := ShardHeader{}
	reader := bufio.NewReader(&b)
	if err := readShardBlockHeader(reader, &sh2); err != nil {
		t.Error(err)
		return
	}

	if sh.timestamp.Unix() != sh2.timestamp.Unix() {
		t.Error("Timestamp not equal")
		return
	}

	if sh.bits != sh2.bits {
		t.Error("Bits not equal")
		return
	}

	if sh.mergeMiningNumber != sh2.mergeMiningNumber {
		t.Error("MergeMiningNumber not equal")
		return
	}

	if bytes.Compare(sh.merkleRoot[:], sh2.merkleRoot[:]) != 0 {
		t.Error("Merkle Root not equal")
		return
	}

	if bytes.Compare(sh.prevBlock[:], sh2.prevBlock[:]) != 0 {
		t.Error("prevBlock Root not equal")
		return
	}

	hashes2, coding2, bits2 := sh.bCHeader.MergedMiningTreeCodingProof()

	if bytes.Compare(hashes, hashes2) != 0 {
		t.Error("Proof hashes not equal")
		return
	}

	if bytes.Compare(coding, coding2) != 0 {
		t.Error("Proof coding not equal")
		return
	}

	if bits != bits2 {
		t.Error("Proof bits not equal")
		return
	}
}

func TestBlockShardHeaderEncoding(t *testing.T) {

	sh := &ShardHeader{}
	block := &MsgBlock{

		ShardBlock: true,
		Header:     sh,
	}

	sh.timestamp = time.Now()
	sh.bCHeader = BeaconHeader{
		version:   BVersion(7),
		bits:      1,
		timestamp: time.Now().Add(1 * time.Hour),
	}
	sh.bits = 3
	sh.mergeMiningNumber = 4
	rand.Read(sh.merkleRoot[:])
	rand.Read(sh.prevBlock[:])
	sh.mergeMiningNumber = 5

	hashes := make([]byte, 400)
	coding := make([]byte, 300)
	rand.Read(hashes[:])
	rand.Read(coding[:])

	var bits uint32 = 222

	sh.bCHeader.SetMergedMiningTreeCodingProof(hashes, coding, bits)

	var b bytes.Buffer
	wr := bufio.NewWriter(&b)

	bCopy := block.Copy()

	fmt.Println("Clone 1", sh.bCHeader.treeEncoding)
	fmt.Println("Clone 2", bCopy.Header.BeaconHeader().treeEncoding)

	if err := bCopy.BtcEncode(wr, 0, BaseEncoding); err != nil {
		t.Error(err)
		return
	}
	wr.Flush()

	block2 := &MsgBlock{
		ShardBlock: true,
		Header:     &ShardHeader{},
	}
	reader := bufio.NewReader(&b)

	if err := block2.BtcDecode(reader, 0, BaseEncoding); err != nil {
		t.Error(err)
		return
	}

	sh2 := block2.Header.(*ShardHeader)

	if sh.timestamp.Unix() != sh2.timestamp.Unix() {
		t.Error("Timestamp not equal")
		return
	}

	if sh.bits != sh2.bits {
		t.Error("Bits not equal")
		return
	}

	if sh.mergeMiningNumber != sh2.mergeMiningNumber {
		t.Error("MergeMiningNumber not equal")
		return
	}

	if bytes.Compare(sh.merkleRoot[:], sh2.merkleRoot[:]) != 0 {
		t.Error("Merkle Root not equal")
		return
	}

	if bytes.Compare(sh.prevBlock[:], sh2.prevBlock[:]) != 0 {
		t.Error("prevBlock Root not equal")
		return
	}

	hashes2, coding2, bits2 := sh2.bCHeader.MergedMiningTreeCodingProof()

	fmt.Println(hashes2, coding2, bits2)
	if bytes.Compare(hashes, hashes2) != 0 {
		t.Error("Proof hashes not equal")
		return
	}

	if bytes.Compare(coding, coding2) != 0 {
		t.Error("Proof coding not equal")
		return
	}

	if bits != bits2 {
		t.Error("Proof bits not equal")
		return
	}
}
