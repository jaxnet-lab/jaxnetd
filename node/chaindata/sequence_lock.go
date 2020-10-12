package chaindata

// SequenceLock represents the converted relative lock-time in seconds, and
// absolute block-height for a transaction input's relative lock-times.
// According to SequenceLock, after the referenced input has been confirmed
// within a block, a transaction spending that input can be included into a
// block either after 'seconds' (according to past median time), or once the
// 'BlockHeight' has been reached.
type SequenceLock struct {
	Seconds     int64
	BlockHeight int32
}
