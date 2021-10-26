// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package types

import (
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// TestBitcoinNetStringer tests the stringized output for bitcoin net types.
func TestBitcoinNetStringer(t *testing.T) {
	tests := []struct {
		in   wire.JaxNet
		want string
	}{
		{wire.MainNet, "MainNet"},
		{wire.TestNet, "TestNet"},
		{wire.SimNet, "SimNet"},
		{0xffffffff, "Unknown JaxNet (4294967295)"},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		result := test.in.String()
		if result != test.want {
			t.Errorf("String #%d\n got: %s want: %s", i, result,
				test.want)
			continue
		}
	}
}
