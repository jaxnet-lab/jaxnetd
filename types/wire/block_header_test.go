package wire

import (
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
