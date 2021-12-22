// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package config

import (
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

// ActiveNetParams is a pointer to the parameters specific to the
// currently active bitcoin network.
var ActiveNetParams = &chaincfg.MainNetParams
