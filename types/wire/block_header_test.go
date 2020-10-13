package wire

import (
	"bytes"
	"crypto/rand"
	"fmt"
	rand2 "math/rand"
	"testing"
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
		bh.SetMergeMiningTrie(hashes, coding, bitsSize)

		hashes2, coding2, bitsSize2 := bh.MergedMiningTreeCodingProof()
		if  bytes.Compare(hashes, hashes2) != 0{
			t.Error("Hashes not equal at ", i)
		}

		if  bytes.Compare(coding, coding2) != 0{
			t.Error("Coding not equal at ", i)
		}
		if bitsSize != bitsSize2 {
			t.Error("Bits not equal at ", i)
		}
		fmt.Printf("%d. Test ok. Hash %d, Coding %d, %d\n", i, hashesSize, codingSize, bitsSize)
		i++
	}

}
