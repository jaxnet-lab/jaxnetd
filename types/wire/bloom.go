package wire

// BloomUpdateType specifies how the filter is updated when a match is found
type BloomUpdateType uint8

const (
	// BloomUpdateNone indicates the filter is not adjusted when a match is
	// found.
	BloomUpdateNone BloomUpdateType = 0

	// BloomUpdateAll indicates if the filter matches any data element in a
	// public key script, the outpoint is serialized and inserted into the
	// filter.
	BloomUpdateAll BloomUpdateType = 1

	// BloomUpdateP2PubkeyOnly indicates if the filter matches a data
	// element in a public key script and the script is of the standard
	// pay-to-pubkey or multisig, the outpoint is serialized and inserted
	// into the filter.
	BloomUpdateP2PubkeyOnly BloomUpdateType = 2
)
