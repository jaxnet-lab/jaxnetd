// nolint: forcetypeassert
package rpc

import (
	"fmt"
	"math"
	"time"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// wsCommandHandler describes a callback function used to handle a specific
// command.
type wsCommandHandler func(*cprovider.ChainProvider, *wsClient, interface{}) (interface{}, error)

type wsHandler struct {
	core     *MultiChainRPC
	handlers map[string]wsCommandHandler
}

// nolint: golint, revive
func WebSocketHandlers(core *MultiChainRPC) *wsHandler {
	res := &wsHandler{core: core}

	res.handlers = map[string]wsCommandHandler{
		"loadtxfilter":              res.handleLoadTxFilter,
		"help":                      res.handleWebsocketHelp,
		"notifyblocks":              res.handleNotifyBlocks,
		"notifynewtransactions":     res.handleNotifyNewTransactions,
		"notifyreceived":            res.handleNotifyReceived,
		"notifyspent":               res.handleNotifySpent,
		"session":                   res.handleSession,
		"stopnotifyblocks":          res.handleStopNotifyBlocks,
		"stopnotifynewtransactions": res.handleStopNotifyNewTransactions,
		"stopnotifyspent":           res.handleStopNotifySpent,
		"stopnotifyreceived":        res.handleStopNotifyReceived,
		"rescan":                    res.handleRescan,
		"rescanblocks":              res.handleRescanBlocks,
	}
	return res
}

func (h *wsHandler) handleWebsocketHelp(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	// TODO: repair help responses
	// cmd, ok := icmd.(*jaxjson.HelpCmd)
	// if !ok {
	//	return nil, jaxjson.ErrRPCInternal
	// }
	//
	// // Provide a usage overview of all commands when no specific command
	// // was specified.
	// var command string
	// if cmd.Command != nil {
	//	command = *cmd.Command
	// }
	// if command == "" {
	//	usage, err := h.server.helpCache.rpcUsage(true)
	//	if err != nil {
	//		//context := "Failed to generate RPC usage"
	//		return nil, err
	//	}
	//	return usage, nil
	// }
	//
	// // Check that the command asked for is supported and implemented.
	// // Search the list of websocket handlers as well as the main list of
	// // handlers since help should only be provided for those cases.
	// valid := true
	// if _, ok := h.handlers[command]; !ok {
	//	if _, ok := h.handlers[command]; !ok {
	//		valid = false
	//	}
	// }
	// if !valid {
	//	return nil, &jaxjson.RPCError{
	//		Code:    jaxjson.ErrRPCInvalidParameter,
	//		Message: "Unknown command: " + command,
	//	}
	// }
	//
	// // Get the help for the command.
	// help, err := h.server.helpCache.rpcMethodHelp(command)
	// if err != nil {
	//	return nil, err
	// }
	// return help, nil
	return nil, nil
}

func (h *wsHandler) handleLoadTxFilter(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd := icmd.(*jaxjson.LoadTxFilterCmd)

	outPoints := make([]wire.OutPoint, len(cmd.OutPoints))
	for i := range cmd.OutPoints {
		hash, err := chainhash.NewHashFromStr(cmd.OutPoints[i].Hash)
		if err != nil {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCInvalidParameter,
				Message: err.Error(),
			}
		}
		outPoints[i] = wire.OutPoint{
			Hash:  *hash,
			Index: cmd.OutPoints[i].Index,
		}
	}

	params := chain.ChainParams

	wsc.Lock()
	if cmd.Reload || wsc.filterData == nil {
		wsc.filterData = newWSClientFilter(cmd.Addresses, outPoints, params)
		wsc.Unlock()
	} else {
		wsc.Unlock()

		wsc.filterData.mu.Lock()
		for _, a := range cmd.Addresses {
			wsc.filterData.addAddressStr(a, params)
		}
		for i := range outPoints {
			wsc.filterData.addUnspentOutPoint(&outPoints[i])
		}
		wsc.filterData.mu.Unlock()
	}

	return nil, nil
}

func (h *wsHandler) handleSession(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	return &jaxjson.SessionResult{SessionID: wsc.sessionID}, nil
}

func (h *wsHandler) handleNotifyBlocks(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	wsc.manager.RegisterBlockUpdates(wsc, chain.ChainCtx.ShardID())
	return nil, nil
}

func (h *wsHandler) handleStopNotifyBlocks(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	wsc.manager.UnregisterBlockUpdates(wsc, chain.ChainCtx.ShardID())
	return nil, nil
}

func (h *wsHandler) handleNotifyNewTransactions(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*jaxjson.NotifyNewTransactionsCmd)
	if !ok {
		return nil, jaxjson.ErrRPCInternal
	}

	wsc.verboseTxUpdates = cmd.Verbose != nil && *cmd.Verbose
	wsc.manager.RegisterNewMempoolTxsUpdates(wsc)
	return nil, nil
}

func (h *wsHandler) handleStopNotifyNewTransactions(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	wsc.manager.UnregisterNewMempoolTxsUpdates(wsc)
	return nil, nil
}

// nolint: staticcheck
func (h *wsHandler) handleNotifyReceived(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*jaxjson.NotifyReceivedCmd)
	if !ok {
		return nil, jaxjson.ErrRPCInternal
	}

	// Decode addresses to validate input, but the strings slice is used
	// directly if these are all ok.
	err := checkAddressValidity(cmd.Addresses, chain.ChainParams)
	if err != nil {
		return nil, err
	}

	wsc.manager.RegisterTxOutAddressRequests(chain, wsc, cmd.Addresses)
	return nil, nil
}

// nolint: staticcheck
func (h *wsHandler) handleStopNotifyReceived(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*jaxjson.StopNotifyReceivedCmd)
	if !ok {
		return nil, jaxjson.ErrRPCInternal
	}

	// Decode addresses to validate input, but the strings slice is used
	// directly if these are all ok.
	err := checkAddressValidity(cmd.Addresses, chain.ChainParams)
	if err != nil {
		return nil, err
	}

	for _, addr := range cmd.Addresses {
		wsc.manager.UnregisterTxOutAddressRequest(chain, wsc, addr)
	}

	return nil, nil
}

// nolint: staticcheck
func (h *wsHandler) handleNotifySpent(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*jaxjson.NotifySpentCmd)
	if !ok {
		return nil, jaxjson.ErrRPCInternal
	}

	outpoints, err := deserializeOutpoints(cmd.OutPoints)
	if err != nil {
		return nil, err
	}

	wsc.manager.RegisterSpentRequests(chain, wsc, outpoints)
	return nil, nil
}

// nolint: staticcheck
func (h *wsHandler) handleStopNotifySpent(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*jaxjson.StopNotifySpentCmd)
	if !ok {
		return nil, jaxjson.ErrRPCInternal
	}

	outpoints, err := deserializeOutpoints(cmd.OutPoints)
	if err != nil {
		return nil, err
	}

	for _, outpoint := range outpoints {
		wsc.manager.UnregisterSpentRequest(chain, wsc, outpoint)
	}

	return nil, nil
}

// handleRescan implements the rescan command extension for websocket
// connections.
//
// NOTE: This does not smartly handle reorgs, and fixing requires database
// changes (for safe, concurrent access to full block ranges, and support
// for other chains than the best chain).  It will, however, detect whether
// a reorg removed a block that was previously processed, and result in the
// handler erroring.  Clients must handle this by finding a block still in
// the chain (perhaps from a rescanprogress notification) to resume their
// rescan.
func (h *wsHandler) handleRescan(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*jaxjson.RescanCmd)
	if !ok {
		return nil, jaxjson.ErrRPCInternal
	}

	outpoints := make([]*wire.OutPoint, 0, len(cmd.OutPoints))
	for i := range cmd.OutPoints {
		cmdOutpoint := &cmd.OutPoints[i]
		blockHash, err := chainhash.NewHashFromStr(cmdOutpoint.Hash)
		if err != nil {
			return nil, rpcDecodeHexError(cmdOutpoint.Hash)
		}
		outpoint := wire.NewOutPoint(blockHash, cmdOutpoint.Index)
		outpoints = append(outpoints, outpoint)
	}

	numAddrs := len(cmd.Addresses)
	if numAddrs == 1 {
		log.Debug().Msg("Beginning rescan for 1 address")
	} else {
		log.Debug().Msgf("Beginning rescan for %d addresses", numAddrs)
	}

	// Build lookup maps.
	lookups := rescanKeys{
		addrs:   map[string]struct{}{},
		unspent: map[wire.OutPoint]struct{}{},
	}
	for _, addrStr := range cmd.Addresses {
		lookups.addrs[addrStr] = struct{}{}
	}
	for _, outpoint := range outpoints {
		lookups.unspent[*outpoint] = struct{}{}
	}

	// chain := wsc.server.cfg.Chain

	minBlockHash, err := chainhash.NewHashFromStr(cmd.BeginBlock)
	if err != nil {
		return nil, rpcDecodeHexError(cmd.BeginBlock)
	}
	minBlock, err := chain.BlockChain().BlockHeightByHash(minBlockHash)
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCBlockNotFound,
			Message: "Error getting block: " + err.Error(),
		}
	}

	maxBlock := int32(math.MaxInt32)
	if cmd.EndBlock != nil {
		maxBlockHash, err := chainhash.NewHashFromStr(*cmd.EndBlock)
		if err != nil {
			return nil, rpcDecodeHexError(*cmd.EndBlock)
		}
		maxBlock, err = chain.BlockChain().BlockHeightByHash(maxBlockHash)
		if err != nil {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCBlockNotFound,
				Message: "Error getting block: " + err.Error(),
			}
		}
	}

	var (
		lastBlock     *jaxutil.Block
		lastBlockHash *chainhash.Hash
	)
	if len(lookups.addrs) != 0 || len(lookups.unspent) != 0 {
		// With all the arguments parsed, we'll execute our chunked rescan
		// which will notify the clients of any address deposits or output
		// spends.
		lastBlock, lastBlockHash, err = scanBlockChunks(
			wsc, cmd, &lookups, minBlock, maxBlock, chain,
		)
		if err != nil {
			return nil, err
		}

		// If the last block is nil, then this means that the client
		// disconnected mid-rescan. As a result, we don't need to send
		// anything back to them.
		if lastBlock == nil {
			return nil, nil
		}
	} else {
		log.Debug().Msgf("Skipping rescan as client has no addrs/utxos")

		// If we didn't actually do a rescan, then we'll give the
		// client our best known block within the final rescan finished
		// notification.
		chainTip := chain.BestSnapshot()
		lastBlockHash = &chainTip.Hash
		lastBlock, err = chain.BlockChain().BlockByHash(lastBlockHash)
		if err != nil {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCBlockNotFound,
				Message: "Error getting block: " + err.Error(),
			}
		}
	}

	// Notify websocket client of the finished rescan.  Due to how btcd
	// asynchronously queues notifications to not block calling code,
	// there is no guarantee that any of the notifications created during
	// rescan (such as rescanprogress, recvtx and redeemingtx) will be
	// received before the rescan RPC returns.  Therefore, another method
	// is needed to safely inform clients that all rescan notifications have
	// been sent.
	n := jaxjson.NewRescanFinishedNtfn(
		lastBlockHash.String(), lastBlock.Height(),
		lastBlock.MsgBlock().Header.Timestamp().Unix(),
	)
	if mn, err := jaxjson.MarshalCmd(nil, chain.ChainParams.ChainID, n); err != nil {
		log.Error().Msgf("Failed to marshal rescan finished notification: %v", err)
	} else {
		// The rescan is finished, so we don't care whether the client
		// has disconnected at this point, so discard error.
		_ = wsc.QueueNotification(mn)
	}

	log.Debug().Msg("Finished rescan")
	return nil, nil
}

// handleRescanBlocks implements the rescanblocks command extension for
// websocket connections.
//
// NOTE: This extension is ported from github.com/decred/dcrd
func (h *wsHandler) handleRescanBlocks(chain *cprovider.ChainProvider, wsc *wsClient, icmd interface{}) (interface{}, error) {
	cmd, ok := icmd.(*jaxjson.RescanBlocksCmd)
	if !ok {
		return nil, jaxjson.ErrRPCInternal
	}

	// Load client's transaction filter.  Must exist in order to continue.
	wsc.Lock()
	filter := wsc.filterData
	wsc.Unlock()
	if filter == nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCMisc,
			Message: "Transaction filter must be loaded before rescanning",
		}
	}

	blockHashes := make([]*chainhash.Hash, len(cmd.BlockHashes))

	for i := range cmd.BlockHashes {
		hash, err := chainhash.NewHashFromStr(cmd.BlockHashes[i])
		if err != nil {
			return nil, err
		}
		blockHashes[i] = hash
	}

	discoveredData := make([]jaxjson.RescannedBlock, 0, len(blockHashes))

	// Iterate over each block in the request and rescan.  When a block
	// contains relevant transactions, add it to the response.
	bc := chain.BlockChain()
	params := chain.ChainParams
	var lastBlockHash *chainhash.Hash
	for i := range blockHashes {
		block, err := bc.BlockByHash(blockHashes[i])
		if err != nil {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCBlockNotFound,
				Message: "Failed to fetch block: " + err.Error(),
			}
		}
		prevHash := block.MsgBlock().Header.PrevBlockHash()
		if lastBlockHash != nil && !prevHash.IsEqual(lastBlockHash) {
			return nil, &jaxjson.RPCError{
				Code: jaxjson.ErrRPCInvalidParameter,
				Message: fmt.Sprintf("Block %v is not a child of %v",
					blockHashes[i], lastBlockHash),
			}
		}
		lastBlockHash = blockHashes[i]

		transactions := rescanBlockFilter(filter, block, params)
		if len(transactions) != 0 {
			discoveredData = append(discoveredData, jaxjson.RescannedBlock{
				Hash:         cmd.BlockHashes[i],
				Transactions: transactions,
			})
		}
	}

	return &discoveredData, nil
}

// rescanBlockFilter rescans a block for any relevant transactions for the
// passed lookup keys. Any discovered transactions are returned hex encoded as
// a string slice.
//
// NOTE: This extension is ported from github.com/decred/dcrd
func rescanBlockFilter(filter *wsClientFilter, block *jaxutil.Block, params *chaincfg.Params) []string {
	var transactions []string

	filter.mu.Lock()
	for _, tx := range block.Transactions() {
		msgTx := tx.MsgTx()

		// Keep track of whether the transaction has already been added
		// to the result.  It shouldn't be added twice.
		added := false

		// Scan inputs if not a coinbase transaction.
		if !chaindata.IsCoinBaseTx(msgTx) {
			for _, input := range msgTx.TxIn {
				if !filter.existsUnspentOutPoint(&input.PreviousOutPoint) {
					continue
				}
				if !added {
					transactions = append(transactions, txHexString(msgTx))
					added = true
				}
			}
		}

		// Scan outputs.
		for i, output := range msgTx.TxOut {
			_, addrs, _, err := txscript.ExtractPkScriptAddrs(output.PkScript, params)
			if err != nil {
				continue
			}
			for _, a := range addrs {
				if !filter.existsAddress(a) {
					continue
				}

				op := wire.OutPoint{
					Hash:  *tx.Hash(),
					Index: uint32(i),
				}
				filter.addUnspentOutPoint(&op)

				if !added {
					transactions = append(transactions, txHexString(msgTx))
					added = true
				}
			}
		}
	}
	filter.mu.Unlock()

	return transactions
}

// checkAddressValidity checks the validity of each address in the passed
// string slice. It does this by attempting to decode each address using the
// current active network parameters. If any single address fails to decode
// properly, the function returns an error. Otherwise, nil is returned.
func checkAddressValidity(addrs []string, params *chaincfg.Params) error {
	for _, addr := range addrs {
		_, err := jaxutil.DecodeAddress(addr, params)
		if err != nil {
			return &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCInvalidAddressOrKey,
				Message: fmt.Sprintf("Invalid address or key: %v", addr),
			}
		}
	}
	return nil
}

// deserializeOutpoints deserializes each serialized outpoint.
func deserializeOutpoints(serializedOuts []jaxjson.OutPoint) ([]*wire.OutPoint, error) {
	outpoints := make([]*wire.OutPoint, 0, len(serializedOuts))
	for i := range serializedOuts {
		blockHash, err := chainhash.NewHashFromStr(serializedOuts[i].Hash)
		if err != nil {
			return nil, rpcDecodeHexError(serializedOuts[i].Hash)
		}
		index := serializedOuts[i].Index
		outpoints = append(outpoints, wire.NewOutPoint(blockHash, index))
	}

	return outpoints, nil
}

// scanBlockChunks executes a rescan in chunked stages. We do this to limit the
// amount of memory that we'll allocate to a given rescan. Every so often,
// we'll send back a rescan progress notification to the websockets client. The
// final block and block hash that we've scanned will be returned.
func scanBlockChunks(wsc *wsClient, cmd *jaxjson.RescanCmd, lookups *rescanKeys, minBlock,
	maxBlock int32, chain *cprovider.ChainProvider) (
	*jaxutil.Block, *chainhash.Hash, error,
) {
	// lastBlock and lastBlockHash track the previously-rescanned block.
	// They equal nil when no previous blocks have been rescanned.
	var (
		lastBlock     *jaxutil.Block
		lastBlockHash *chainhash.Hash
	)

	// A ticker is created to wait at least 10 seconds before notifying the
	// websocket client of the current progress completed by the rescan.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Instead of fetching all block shas at once, fetch in smaller chunks
	// to ensure large rescans consume a limited amount of memory.
fetchRange:
	for minBlock < maxBlock {
		// Limit the max number of hashes to fetch at once to the
		// maximum number of items allowed in a single inventory.
		// This value could be higher since it's not creating inventory
		// messages, but this mirrors the limiting logic used in the
		// peer-to-peer protocol.
		maxLoopBlock := maxBlock
		if maxLoopBlock-minBlock > wire.MaxInvPerMsg {
			maxLoopBlock = minBlock + wire.MaxInvPerMsg
		}
		hashList, err := chain.BlockChain().HeightRange(minBlock, maxLoopBlock)
		if err != nil {
			log.Error().Msgf("Error looking up block range: %v", err)
			return nil, nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCDatabase,
				Message: "Database error: " + err.Error(),
			}
		}
		if len(hashList) == 0 {
			// The rescan is finished if no blocks hashes for this
			// range were successfully fetched and a stop block
			// was provided.
			if maxBlock != math.MaxInt32 {
				break
			}

			// If the rescan is through the current block, set up
			// the client to continue to receive notifications
			// regarding all rescanned addresses and the current set
			// of unspent outputs.
			//
			// This is done safely by temporarily grabbing exclusive
			// access of the block manager.  If no more blocks have
			// been attached between this pause and the fetch above,
			// then it is safe to register the websocket client for
			// continuous notifications if necessary.  Otherwise,
			// continue the fetch loop again to rescan the new
			// blocks (or error due to an irrecoverable reorganize).
			pauseGuard := chain.SyncManager.Pause()
			best := chain.BestSnapshot()
			curHash := &best.Hash
			again := true
			if lastBlockHash == nil || *lastBlockHash == *curHash {
				again = false
				n := wsc.manager
				n.RegisterSpentRequests(chain, wsc, lookups.unspentSlice())
				n.RegisterTxOutAddressRequests(chain, wsc, cmd.Addresses)
			}
			close(pauseGuard)
			if err != nil {
				log.Error().Msgf("Error fetching best block "+
					"hash: %v", err)
				return nil, nil, &jaxjson.RPCError{
					Code:    jaxjson.ErrRPCDatabase,
					Message: "Database error: " + err.Error(),
				}
			}
			if again {
				continue
			}
			break
		}

	loopHashList:
		for i := range hashList {
			blk, err := chain.BlockChain().BlockByHash(&hashList[i])
			if err != nil {
				// Only handle reorgs if a block could not be
				// found for the hash.
				if dbErr, ok := err.(database.Error); !ok ||
					dbErr.ErrorCode != database.ErrBlockNotFound {

					log.Error().Msgf("Error looking up block: %v", err)
					return nil, nil, &jaxjson.RPCError{
						Code: jaxjson.ErrRPCDatabase,
						Message: "Database error: " +
							err.Error(),
					}
				}

				// If an absolute max block was specified, don't
				// attempt to handle the reorg.
				if maxBlock != math.MaxInt32 {
					log.Error().Msgf("Stopping rescan for reorged block %v", cmd.EndBlock)
					return nil, nil, &ErrRescanReorg
				}

				// If the lookup for the previously valid block
				// hash failed, there may have been a reorg.
				// Fetch a new range of block hashes and verify
				// that the previously processed block (if there
				// was any) still exists in the database.  If it
				// doesn't, we error.
				//
				// A goto is used to branch executation back to
				// before the range was evaluated, as it must be
				// reevaluated for the new hashList.
				minBlock += int32(i)
				hashList, err = recoverFromReorg(
					chain, minBlock, maxBlock, lastBlockHash,
				)
				if err != nil {
					return nil, nil, err
				}
				if len(hashList) == 0 {
					break fetchRange
				}
				goto loopHashList
			}
			if i == 0 && lastBlockHash != nil {
				// Ensure the new hashList is on the same fork
				// as the last block from the old hashList.
				jsonErr := descendantBlock(lastBlockHash, blk)
				if jsonErr != nil {
					return nil, nil, jsonErr
				}
			}

			// A select statement is used to stop rescans if the
			// client requesting the rescan has disconnected.
			select {
			case <-wsc.quit:
				log.Debug().Msgf("Stopped rescan at height %v "+
					"for disconnected client", blk.Height())
				return nil, nil, nil
			default:
				rescanBlock(chain, wsc, lookups, blk)
				lastBlock = blk
				lastBlockHash = blk.Hash()
			}

			// Periodically notify the client of the progress
			// completed.  Continue with next block if no progress
			// notification is needed yet.
			select {
			case <-ticker.C: // fallthrough
			default:
				continue
			}

			n := jaxjson.NewRescanProgressNtfn(
				hashList[i].String(), blk.Height(),
				blk.MsgBlock().Header.Timestamp().Unix(),
			)
			mn, err := jaxjson.MarshalCmd(nil, chain.ChainParams.ChainID, n)
			if err != nil {
				log.Error().Msgf("Failed to marshal rescan "+
					"progress notification: %v", err)
				continue
			}

			if err = wsc.QueueNotification(mn); err == ErrClientQuit {
				// Finished if the client disconnected.
				log.Debug().Msgf("Stopped rescan at height %v "+
					"for disconnected client", blk.Height())
				return nil, nil, nil
			}
		}

		minBlock += int32(len(hashList))
	}

	return lastBlock, lastBlockHash, nil
}

type rescanKeys struct {
	addrs   map[string]struct{}
	unspent map[wire.OutPoint]struct{}
}

// unspentSlice returns a slice of currently-unspent outpoints for the rescan
// lookup keys.  This is primarily intended to be used to register outpoints
// for continuous notifications after a rescan has completed.
func (r *rescanKeys) unspentSlice() []*wire.OutPoint {
	ops := make([]*wire.OutPoint, 0, len(r.unspent))
	for op := range r.unspent {
		opCopy := op
		ops = append(ops, &opCopy)
	}
	return ops
}

// recoverFromReorg attempts to recover from a detected reorganize during a
// rescan.  It fetches a new range of block shas from the database and
// verifies that the new range of blocks is on the same fork as a previous
// range of blocks.  If this condition does not hold true, the JSON-RPC error
// for an unrecoverable reorganize is returned.
func recoverFromReorg(chain *cprovider.ChainProvider, minBlock, maxBlock int32,
	lastBlock *chainhash.Hash,
) ([]chainhash.Hash, error) {
	hashList, err := chain.BlockChain().HeightRange(minBlock, maxBlock)
	if err != nil {
		log.Error().Msgf("Error looking up block range: %v", err)
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCDatabase,
			Message: "Database error: " + err.Error(),
		}
	}
	if lastBlock == nil || len(hashList) == 0 {
		return hashList, nil
	}

	blk, err := chain.BlockChain().BlockByHash(&hashList[0])
	if err != nil {
		log.Error().Msgf("Error looking up possibly reorged block: %v",
			err)
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCDatabase,
			Message: "Database error: " + err.Error(),
		}
	}

	jsonErr := descendantBlock(lastBlock, blk)
	if jsonErr != nil {
		return nil, jsonErr
	}
	return hashList, nil
}

// descendantBlock returns the appropriate JSON-RPC error if a current block
// fetched during a reorganize is not a direct child of the parent block hash.
func descendantBlock(prevHash *chainhash.Hash, curBlock *jaxutil.Block) error {
	curHash := curBlock.MsgBlock().Header.PrevBlockHash()
	if !prevHash.IsEqual(&curHash) {
		log.Error().Msgf("Stopping rescan for reorged block %v "+
			"(replaced by block %v)", prevHash, curHash)
		return &ErrRescanReorg
	}
	return nil
}

// rescanBlock rescans all transactions in a single block.  This is a helper
// function for handleRescan.
func rescanBlock(chain *cprovider.ChainProvider, wsc *wsClient, lookups *rescanKeys, blk *jaxutil.Block) {
	for _, tx := range blk.Transactions() {
		// Hexadecimal representation of this tx.  Only created if
		// needed, and reused for later notifications if already made.
		var txHex string

		// All inputs and outputs must be iterated through to correctly
		// modify the unspent map, however, just a single notification
		// for any matching transaction inputs or outputs should be
		// created and sent.
		spentNotified := false
		recvNotified := false

		// notifySpend is a closure we'll use when we first detect that
		// a transactions spends an outpoint/script in our filter list.
		notifySpend := func() error {
			if txHex == "" {
				txHex = txHexString(tx.MsgTx())
			}
			marshalledJSON, err := newRedeemingTxNotification(chain, txHex, tx.Index(), blk)
			if err != nil {
				return fmt.Errorf("unable to marshal jaxjson.RedeeminTxNtfn: %v", err)
			}

			return wsc.QueueNotification(marshalledJSON)
		}

		// We'll start by iterating over the transaction's inputs to
		// determine if it spends an outpoint/script in our filter list.
		for _, txin := range tx.MsgTx().TxIn {
			// If it spends an outpoint, we'll dispatch a spend
			// notification for the transaction.
			if _, ok := lookups.unspent[txin.PreviousOutPoint]; ok {
				delete(lookups.unspent, txin.PreviousOutPoint)

				if spentNotified {
					continue
				}

				err := notifySpend()

				// Stop the rescan early if the websocket client
				// disconnected.
				if err == ErrClientQuit {
					return
				}
				if err != nil {
					log.Error().Msgf("Unable to notify redeeming transaction %v: %v", tx.Hash(), err)
					continue
				}

				spentNotified = true
			}

			// We'll also recompute the pkScript the input is
			// attempting to spend to determine whether it is
			// relevant to us.
			pkScript, err := txscript.ComputePkScript(
				txin.SignatureScript, txin.Witness,
			)
			if err != nil {
				continue
			}
			addr, err := pkScript.Address(chain.ChainParams)
			if err != nil {
				continue
			}

			// If it is, we'll also dispatch a spend notification
			// for this transaction if we haven't already.
			if _, ok := lookups.addrs[addr.String()]; ok {
				if spentNotified {
					continue
				}

				err := notifySpend()

				// Stop the rescan early if the websocket client
				// disconnected.
				if err == ErrClientQuit {
					return
				}
				if err != nil {
					log.Error().Msgf("Unable to notify "+
						"redeeming transaction %v: %v",
						tx.Hash(), err)
					continue
				}

				spentNotified = true
			}
		}

		for txOutIdx, txout := range tx.MsgTx().TxOut {
			_, addrs, _, _ := txscript.ExtractPkScriptAddrs(txout.PkScript, chain.ChainParams)

			for _, addr := range addrs {
				if _, ok := lookups.addrs[addr.String()]; !ok {
					continue
				}

				outpoint := wire.OutPoint{
					Hash:  *tx.Hash(),
					Index: uint32(txOutIdx),
				}
				lookups.unspent[outpoint] = struct{}{}

				if recvNotified {
					continue
				}

				if txHex == "" {
					txHex = txHexString(tx.MsgTx())
				}
				ntfn := jaxjson.NewRecvTxNtfn(txHex, blockDetails(blk, tx.Index()))

				marshalledJSON, err := jaxjson.MarshalCmd(nil, chain.ChainParams.ChainID, ntfn)
				if err != nil {
					log.Error().Msgf("Failed to marshal recvtx notification: %v", err)
					return
				}

				err = wsc.QueueNotification(marshalledJSON)
				// Stop the rescan early if the websocket client
				// disconnected.
				if err == ErrClientQuit {
					return
				}
				recvNotified = true
			}
		}
	}
}
