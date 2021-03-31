package rpc

// import (
// 	"context"

// 	"gitlab.com/jaxnet/core/shard.core/btcutil"
// 	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
// )

// type wsChainManager struct {
// 	chainProvider *cprovider.ChainProvider

// 	// queueNotification queues a notification for handling.
// 	queueNotification chan interface{}

// 	ctx context.Context
// }

// // NotifyBlockConnected passes a block newly-connected to the best BlockChain
// // to the notification manager for block and transaction notification
// // processing.
// func (m *wsChainManager) NotifyBlockConnected(chain *cprovider.ChainProvider, block *btcutil.Block) {
// 	m.queueNotification <- &notificationBlockConnected{
// 		Block: block,
// 		Chain: chain,
// 	}
// }

// // NotifyBlockDisconnected passes a block disconnected from the best BlockChain
// // to the notification manager for block notification processing.
// func (m *wsChainManager) NotifyBlockDisconnected(chain *cprovider.ChainProvider, block *btcutil.Block) {
// 	m.queueNotification <- &notificationBlockDisconnected{
// 		Block: block,
// 		Chain: chain,
// 	}
// }

// // NotifyMempoolTx passes a transaction accepted by mempool to the
// // notification manager for transaction notification processing.  If
// // isNew is true, the tx is is a new transaction, rather than one
// // added to the mempool during a reorg.
// func (m *wsChainManager) NotifyMempoolTx(tx *btcutil.Tx, isNew bool) {
// 	n := &notificationTxAcceptedByMempool{
// 		isNew: isNew,
// 		tx:    tx,
// 	}

// 	m.queueNotification <- n
// }
