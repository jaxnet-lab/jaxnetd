// Copyright (c) 2013-2018 The btcsuite developers
// Copyright (c) 2016-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package peer

import (
	"container/list"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/go-socks/socks"
	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core/corelog"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
	"gitlab.com/jaxnet/core/shard.core/node/chain"
	"gitlab.com/jaxnet/core/shard.core/node/encoder"
	"gitlab.com/jaxnet/core/shard.core/types"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
	"go.uber.org/zap"
)

const (
	// MaxProtocolVersion is the max protocol version the peer supports.
	MaxProtocolVersion = wire.FeeFilterVersion

	// DefaultTrickleInterval is the min time between attempts to send an
	// inv message to a peer.
	DefaultTrickleInterval = 10 * time.Second

	// MinAcceptableProtocolVersion is the lowest protocol version that a
	// connected peer may support.
	MinAcceptableProtocolVersion = wire.MultipleAddressVersion

	// outputBufferSize is the number of elements the output channels use.
	outputBufferSize = 50

	// invTrickleSize is the maximum amount of inventory to send in a single
	// message when trickling inventory to remote peers.
	maxInvTrickleSize = 1000

	// maxKnownInventory is the maximum number of items to keep in the known
	// inventory cache.
	maxKnownInventory = 1000

	// pingInterval is the interval of time to wait in between sending ping
	// messages.
	pingInterval = 2 * time.Minute

	// negotiateTimeout is the duration of inactivity before we timeout a
	// peer that hasn't completed the initial version negotiation.
	negotiateTimeout = 30 * time.Second

	// idleTimeout is the duration of inactivity before we time out a peer.
	idleTimeout = 5 * time.Minute

	// stallTickInterval is the interval of time between each check for
	// stalled peers.
	stallTickInterval = 15 * time.Second

	// stallResponseTimeout is the base maximum amount of time messages that
	// expect a response will wait before disconnecting the peer for
	// stalling.  The deadlines are adjusted for callback running times and
	// only checked on each stall tick interval.
	stallResponseTimeout = 30 * time.Second
)

var (
	// nodeCount is the total number of peer connections made since startup
	// and is used to assign an id to a peer.
	nodeCount int32

	// zeroHash is the zero value hash (all zeros).  It is defined as a
	// convenience.
	zeroHash chainhash.Hash

	// sentNonces houses the unique nonces that are generated when pushing
	// version messages that are used to detect self connections.
	sentNonces = newMruNonceMap(50)

	// allowSelfConns is only used to allow the tests to bypass the self
	// connection detecting and disconnect logic since they intentionally
	// do so for testing purposes.
	allowSelfConns bool
)

// MessageListeners defines callback function pointers to invoke with message
// listeners for a peer. Any listener which is not set to a concrete callback
// during peer initialization is ignored. Execution of multiple message
// listeners occurs serially, so one callback blocks the execution of the next.
//
// NOTE: Unless otherwise documented, these listeners must NOT directly call any
// blocking calls (such as WaitForShutdown) on the peer instance since the input
// handler goroutine blocks until the callback has completed.  Doing so will
// result in a deadlock.
type MessageListeners struct {
	// OnGetAddr is invoked when a peer receives a getaddr bitcoin message.
	OnGetAddr func(p *Peer, msg *wire.MsgGetAddr)

	// OnAddr is invoked when a peer receives an addr bitcoin message.
	OnAddr func(p *Peer, msg *wire.MsgAddr)

	// OnPing is invoked when a peer receives a ping bitcoin message.
	OnPing func(p *Peer, msg *wire.MsgPing)

	// OnPong is invoked when a peer receives a pong bitcoin message.
	OnPong func(p *Peer, msg *wire.MsgPong)

	// OnAlert is invoked when a peer receives an alert bitcoin message.
	OnAlert func(p *Peer, msg *wire.MsgAlert)

	// OnMemPool is invoked when a peer receives a mempool bitcoin message.
	OnMemPool func(p *Peer, msg *wire.MsgMemPool)

	// OnTx is invoked when a peer receives a tx bitcoin message.
	OnTx func(p *Peer, msg *wire.MsgTx)

	// OnBlock is invoked when a peer receives a block bitcoin message.
	OnBlock func(p *Peer, msg *wire.MsgBlock, buf []byte)

	// OnCFilter is invoked when a peer receives a cfilter bitcoin message.
	OnCFilter func(p *Peer, msg *wire.MsgCFilter)

	// OnCFHeaders is invoked when a peer receives a cfheaders bitcoin
	// message.
	OnCFHeaders func(p *Peer, msg *wire.MsgCFHeaders)

	// OnCFCheckpt is invoked when a peer receives a cfcheckpt bitcoin
	// message.
	OnCFCheckpt func(p *Peer, msg *wire.MsgCFCheckpt)

	// OnInv is invoked when a peer receives an inv bitcoin message.
	OnInv func(p *Peer, msg *wire.MsgInv)

	// OnHeaders is invoked when a peer receives a headers bitcoin message.
	OnHeaders func(p *Peer, msg *wire.MsgHeaders)

	// OnNotFound is invoked when a peer receives a notfound bitcoin
	// message.
	OnNotFound func(p *Peer, msg *wire.MsgNotFound)

	// OnGetData is invoked when a peer receives a getdata bitcoin message.
	OnGetData func(p *Peer, msg *wire.MsgGetData)

	// OnGetBlocks is invoked when a peer receives a getblocks bitcoin
	// message.
	OnGetBlocks func(p *Peer, msg *wire.MsgGetBlocks)

	// OnGetHeaders is invoked when a peer receives a getheaders bitcoin
	// message.
	OnGetHeaders func(p *Peer, msg *wire.MsgGetHeaders)

	// OnGetCFilters is invoked when a peer receives a getcfilters bitcoin
	// message.
	OnGetCFilters func(p *Peer, msg *wire.MsgGetCFilters)

	// OnGetCFHeaders is invoked when a peer receives a getcfheaders
	// bitcoin message.
	OnGetCFHeaders func(p *Peer, msg *wire.MsgGetCFHeaders)

	// OnGetCFCheckpt is invoked when a peer receives a getcfcheckpt
	// bitcoin message.
	OnGetCFCheckpt func(p *Peer, msg *wire.MsgGetCFCheckpt)

	// OnFeeFilter is invoked when a peer receives a feefilter bitcoin message.
	OnFeeFilter func(p *Peer, msg *wire.MsgFeeFilter)

	// OnFilterAdd is invoked when a peer receives a filteradd bitcoin message.
	OnFilterAdd func(p *Peer, msg *wire.MsgFilterAdd)

	// OnFilterClear is invoked when a peer receives a filterclear bitcoin
	// message.
	OnFilterClear func(p *Peer, msg *wire.MsgFilterClear)

	// OnFilterLoad is invoked when a peer receives a filterload bitcoin
	// message.
	OnFilterLoad func(p *Peer, msg *wire.MsgFilterLoad)

	// OnMerkleBlock  is invoked when a peer receives a merkleblock bitcoin
	// message.
	OnMerkleBlock func(p *Peer, msg *wire.MsgMerkleBlock)

	// OnVersion is invoked when a peer receives a version bitcoin message.
	// The caller may return a reject message in which case the message will
	// be sent to the peer and the peer will be disconnected.
	OnVersion func(p *Peer, msg *wire.MsgVersion) *wire.MsgReject

	// OnVerAck is invoked when a peer receives a verack bitcoin message.
	OnVerAck func(p *Peer, msg *wire.MsgVerAck)

	// OnReject is invoked when a peer receives a reject bitcoin message.
	OnReject func(p *Peer, msg *wire.MsgReject)

	// OnSendHeaders is invoked when a peer receives a sendheaders bitcoin
	// message.
	OnSendHeaders func(p *Peer, msg *wire.MsgSendHeaders)

	// OnRead is invoked when a peer receives a bitcoin message.  It
	// consists of the number of bytes read, the message, and whether or not
	// an error in the read occurred.  Typically, callers will opt to use
	// the callbacks for the specific message types, however this can be
	// useful for circumstances such as keeping track of server-wide byte
	// counts or working with custom message types for which the peer does
	// not directly provide a callback.
	OnRead func(p *Peer, bytesRead int, msg wire.Message, err error)

	// OnWrite is invoked when we write a bitcoin message to a peer.  It
	// consists of the number of bytes written, the message, and whether or
	// not an error in the write occurred.  This can be useful for
	// circumstances such as keeping track of server-wide byte counts.
	OnWrite func(p *Peer, bytesWritten int, msg wire.Message, err error)
}

// Config is the struct to hold configuration options useful to Peer.
type Config struct {
	// NewestBlock specifies a callback which provides the newest block
	// details to the peer as needed.  This can be nil in which case the
	// peer will report a block height of 0, however it is good practice for
	// peers to specify this so their currently best known is accurately
	// reported.
	NewestBlock HashFunc

	// HostToNetAddress returns the netaddress for the given host. This can be
	// nil in  which case the host will be parsed as an IP address.
	HostToNetAddress HostToNetAddrFunc

	// Proxy indicates a proxy is being used for connections.  The only
	// effect this has is to prevent leaking the tor proxy address, so it
	// only needs to specified if using a tor proxy.
	Proxy string

	// UserAgentName specifies the user agent name to advertise.  It is
	// highly recommended to specify this value.
	UserAgentName string

	// UserAgentVersion specifies the user agent version to advertise.  It
	// is highly recommended to specify this value and that it follows the
	// form "major.minor.revision" e.g. "2.6.41".
	UserAgentVersion string

	// UserAgentComments specify the user agent comments to advertise.  These
	// values must not contain the illegal characters specified in BIP 14:
	// '/', ':', '(', ')'.
	UserAgentComments []string

	// ChainParams identifies which chain parameters the peer is associated
	// with.  It is highly recommended to specify this field, however it can
	// be omitted in which case the test network will be used.
	ChainParams *chaincfg.Params

	// Services specifies which services to advertise as supported by the
	// local peer.  This field can be omitted in which case it will be 0
	// and therefore advertise no supported services.
	Services wire.ServiceFlag

	// ProtocolVersion specifies the maximum protocol version to use and
	// advertise.  This field can be omitted in which case
	// peer.MaxProtocolVersion will be used.
	ProtocolVersion uint32

	// DisableRelayTx specifies if the remote peer should be informed to
	// not send inv messages for transactions.
	DisableRelayTx bool

	// Listeners houses callback functions to be invoked on receiving peer
	// messages.
	Listeners MessageListeners

	// TrickleInterval is the duration of the ticker which trickles down the
	// inventory to a peer.
	TrickleInterval time.Duration

	// ChainsPortsProvider is a function to get the port of some p2p shard.
	ChainsPortsProvider func(shardID uint32) (int, bool)
	TriggerRedirect     func(peerAddress, newAddress *wire.NetAddress)
}

// minUint32 is a helper function to return the minimum of two uint32s.
// This avoids a math import and the need to cast to floats.
func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// newNetAddress attempts to extract the IP address and port from the passed
// net.Addr interface and create a bitcoin NetAddress structure using that
// information.
func newNetAddress(addr net.Addr, services wire.ServiceFlag) (*wire.NetAddress, error) {
	// addr will be a net.TCPAddr when not using a proxy.
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		ip := tcpAddr.IP
		port := uint16(tcpAddr.Port)
		na := wire.NewNetAddressIPPort(ip, port, services)
		return na, nil
	}

	// addr will be a socks.ProxiedAddr when using a proxy.
	if proxiedAddr, ok := addr.(*socks.ProxiedAddr); ok {
		ip := net.ParseIP(proxiedAddr.Host)
		if ip == nil {
			ip = net.ParseIP("0.0.0.0")
		}
		port := uint16(proxiedAddr.Port)
		na := wire.NewNetAddressIPPort(ip, port, services)
		return na, nil
	}

	// For the most part, addr should be one of the two above cases, but
	// to be safe, fall back to trying to parse the information from the
	// address string as a last resort.
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}
	na := wire.NewNetAddressIPPort(ip, uint16(port), services)
	return na, nil
}

// outMsg is used to house a message to be sent along with a channel to signal
// when the message has been sent (or won't be sent due to things such as
// shutdown)
type outMsg struct {
	msg      wire.Message
	doneChan chan<- struct{}
	encoding encoder.MessageEncoding
}

// stallControlCmd represents the command of a stall control message.
type stallControlCmd uint8

// Constants for the command of a stall control message.
const (
	// sccSendMessage indicates a message is being sent to the remote peer.
	sccSendMessage stallControlCmd = iota

	// sccReceiveMessage indicates a message has been received from the
	// remote peer.
	sccReceiveMessage

	// sccHandlerStart indicates a callback handler is about to be invoked.
	sccHandlerStart

	// sccHandlerStart indicates a callback handler has completed.
	sccHandlerDone
)

// stallControlMsg is used to signal the stall handler about specific events
// so it can properly detect and handle stalled remote peers.
type stallControlMsg struct {
	command stallControlCmd
	message wire.Message
}

// StatsSnap is a snapshot of peer stats at a point in time.
type StatsSnap struct {
	ID             int32
	Addr           string
	Services       wire.ServiceFlag
	LastSend       time.Time
	LastRecv       time.Time
	BytesSent      uint64
	BytesRecv      uint64
	ConnTime       time.Time
	TimeOffset     int64
	Version        uint32
	UserAgent      string
	Inbound        bool
	StartingHeight int32
	LastBlock      int32
	LastPingNonce  uint64
	LastPingTime   time.Time
	LastPingMicros int64
}

// HashFunc is a function which returns a block hash, height and error
// It is used as a callback to get newest block details.
type HashFunc func() (hash *chainhash.Hash, height int32, err error)

// AddrFunc is a func which takes an address and returns a related address.
type AddrFunc func(remoteAddr *wire.NetAddress) *wire.NetAddress

// HostToNetAddrFunc is a func which takes a host, port, services and returns
// the netaddress.
type HostToNetAddrFunc func(host string, port uint16,
	services wire.ServiceFlag) (*wire.NetAddress, error)

// NOTE: The overall data flow of a peer is split into 3 goroutines.  Inbound
// messages are read via the inHandler goroutine and generally dispatched to
// their own handler.  For inbound data-related messages such as blocks,
// transactions, and inventory, the data is handled by the corresponding
// message handlers.  The data flow for outbound messages is split into 2
// goroutines, queueHandler and outHandler.  The first, queueHandler, is used
// as a way for external entities to queue messages, by way of the QueueMessage
// function, quickly regardless of whether the peer is currently sending or not.
// It acts as the traffic cop between the external world and the actual
// goroutine which writes to the network socket.

// Peer provides a basic concurrent safe bitcoin peer for handling bitcoin
// communications via the peer-to-peer protocol.  It provides full duplex
// reading and writing, automatic handling of the initial handshake process,
// querying of usage statistics and other information about the remote peer such
// as its address, user agent, and protocol version, output message queuing,
// inventory trickling, and the ability to dynamically register and unregister
// callbacks for handling bitcoin protocol messages.
//
// Outbound messages are typically queued via QueueMessage or QueueInventory.
// QueueMessage is intended for all messages, including responses to data such
// as blocks and transactions.  QueueInventory, on the other hand, is only
// intended for relaying inventory as it employs a trickling mechanism to batch
// the inventory together.  However, some helper functions for pushing messages
// of specific types that typically require common special handling are
// provided as a convenience.
type Peer struct {
	// The following variables must only be used atomically.
	bytesReceived uint64
	bytesSent     uint64
	lastRecv      int64
	lastSend      int64
	connected     int32
	disconnect    int32

	chain chain.IChainCtx

	conn net.Conn

	// These fields are set at creation time and never modified, so they are
	// safe to read from concurrently without a mutex.
	addr    string
	cfg     Config
	inbound bool

	flagsMtx             sync.Mutex // protects the peer flags below
	na                   *wire.NetAddress
	id                   int32
	userAgent            string
	services             wire.ServiceFlag
	versionKnown         bool
	advertisedProtoVer   uint32 // protocol version advertised by remote
	protocolVersion      uint32 // negotiated protocol version
	sendHeadersPreferred bool   // peer sent a sendheaders message
	verAckReceived       bool
	witnessEnabled       bool

	wireEncoding encoder.MessageEncoding

	knownInventory     *mruInventoryMap
	prevGetBlocksMtx   sync.Mutex
	prevGetBlocksBegin *chainhash.Hash
	prevGetBlocksStop  *chainhash.Hash
	prevGetHdrsMtx     sync.Mutex
	prevGetHdrsBegin   *chainhash.Hash
	prevGetHdrsStop    *chainhash.Hash

	// These fields keep track of statistics for the peer and are protected
	// by the statsMtx mutex.
	statsMtx           sync.RWMutex
	timeOffset         int64
	timeConnected      time.Time
	startingHeight     int32
	lastBlock          int32
	lastAnnouncedBlock *chainhash.Hash
	lastPingNonce      uint64    // Set to nonce if we have a pending ping.
	lastPingTime       time.Time // Time we sent last ping.
	lastPingMicros     int64     // Time for last ping to return.

	stallControl  chan stallControlMsg
	outputQueue   chan outMsg
	sendQueue     chan outMsg
	sendDoneQueue chan struct{}
	outputInvChan chan *types.InvVect
	inQuit        chan struct{}
	queueQuit     chan struct{}
	outQuit       chan struct{}
	quit          chan struct{}
	log           *zap.Logger

	redirectRequested bool
	newAddress        *wire.NetAddress
}

// String returns the peer's address and directionality as a human-readable
// string.
//
// This function is safe for concurrent access.
func (peer *Peer) String() string {
	return fmt.Sprintf("%s (%s)", peer.addr, directionString(peer.inbound))
}

// UpdateLastBlockHeight updates the last known block for the peer.
//
// This function is safe for concurrent access.
func (peer *Peer) UpdateLastBlockHeight(newHeight int32) {
	peer.statsMtx.Lock()
	log.Tracef("Updating last block height of peer %v from %v to %v",
		peer.addr, peer.lastBlock, newHeight)
	peer.lastBlock = newHeight
	peer.statsMtx.Unlock()
}

// UpdateLastAnnouncedBlock updates meta-data about the last block hash this
// peer is known to have announced.
//
// This function is safe for concurrent access.
func (peer *Peer) UpdateLastAnnouncedBlock(blkHash *chainhash.Hash) {
	log.Tracef("Updating last blk for peer %v, %v", peer.addr, blkHash)

	peer.statsMtx.Lock()
	peer.lastAnnouncedBlock = blkHash
	peer.statsMtx.Unlock()
}

// AddKnownInventory adds the passed inventory to the cache of known inventory
// for the peer.
//
// This function is safe for concurrent access.
func (peer *Peer) AddKnownInventory(invVect *types.InvVect) {
	peer.knownInventory.Add(invVect)
}

// StatsSnapshot returns a snapshot of the current peer flags and statistics.
//
// This function is safe for concurrent access.
func (peer *Peer) StatsSnapshot() *StatsSnap {
	peer.statsMtx.RLock()

	peer.flagsMtx.Lock()
	id := peer.id
	addr := peer.addr
	userAgent := peer.userAgent
	services := peer.services
	protocolVersion := peer.advertisedProtoVer
	peer.flagsMtx.Unlock()

	// Get a copy of all relevant flags and stats.
	statsSnap := &StatsSnap{
		ID:             id,
		Addr:           addr,
		UserAgent:      userAgent,
		Services:       services,
		LastSend:       peer.LastSend(),
		LastRecv:       peer.LastRecv(),
		BytesSent:      peer.BytesSent(),
		BytesRecv:      peer.BytesReceived(),
		ConnTime:       peer.timeConnected,
		TimeOffset:     peer.timeOffset,
		Version:        protocolVersion,
		Inbound:        peer.inbound,
		StartingHeight: peer.startingHeight,
		LastBlock:      peer.lastBlock,
		LastPingNonce:  peer.lastPingNonce,
		LastPingMicros: peer.lastPingMicros,
		LastPingTime:   peer.lastPingTime,
	}

	peer.statsMtx.RUnlock()
	return statsSnap
}

// ID returns the peer id.
//
// This function is safe for concurrent access.
func (peer *Peer) ID() int32 {
	peer.flagsMtx.Lock()
	id := peer.id
	peer.flagsMtx.Unlock()

	return id
}

func (peer *Peer) RedirectRequested() bool {
	return peer.redirectRequested
}

func (peer *Peer) NewAddress() *wire.NetAddress {
	peer.flagsMtx.Lock()
	na := *peer.newAddress
	peer.flagsMtx.Unlock()

	return &na
}

// NA returns the peer network address.
//
// This function is safe for concurrent access.
func (peer *Peer) NA() *wire.NetAddress {
	peer.flagsMtx.Lock()
	na := peer.na
	peer.flagsMtx.Unlock()

	return na
}

// Addr returns the peer address.
//
// This function is safe for concurrent access.
func (peer *Peer) Addr() string {
	// The address doesn't change after initialization, therefore it is not
	// protected by a mutex.
	return peer.addr
}

// Inbound returns whether the peer is inbound.
//
// This function is safe for concurrent access.
func (peer *Peer) Inbound() bool {
	return peer.inbound
}

// Services returns the services flag of the remote peer.
//
// This function is safe for concurrent access.
func (peer *Peer) Services() wire.ServiceFlag {
	peer.flagsMtx.Lock()
	services := peer.services
	peer.flagsMtx.Unlock()

	return services
}

// UserAgent returns the user agent of the remote peer.
//
// This function is safe for concurrent access.
func (peer *Peer) UserAgent() string {
	peer.flagsMtx.Lock()
	userAgent := peer.userAgent
	peer.flagsMtx.Unlock()

	return userAgent
}

// LastAnnouncedBlock returns the last announced block of the remote peer.
//
// This function is safe for concurrent access.
func (peer *Peer) LastAnnouncedBlock() *chainhash.Hash {
	peer.statsMtx.RLock()
	lastAnnouncedBlock := peer.lastAnnouncedBlock
	peer.statsMtx.RUnlock()

	return lastAnnouncedBlock
}

// LastPingNonce returns the last ping nonce of the remote peer.
//
// This function is safe for concurrent access.
func (peer *Peer) LastPingNonce() uint64 {
	peer.statsMtx.RLock()
	lastPingNonce := peer.lastPingNonce
	peer.statsMtx.RUnlock()

	return lastPingNonce
}

// LastPingTime returns the last ping time of the remote peer.
//
// This function is safe for concurrent access.
func (peer *Peer) LastPingTime() time.Time {
	peer.statsMtx.RLock()
	lastPingTime := peer.lastPingTime
	peer.statsMtx.RUnlock()

	return lastPingTime
}

// LastPingMicros returns the last ping micros of the remote peer.
//
// This function is safe for concurrent access.
func (peer *Peer) LastPingMicros() int64 {
	peer.statsMtx.RLock()
	lastPingMicros := peer.lastPingMicros
	peer.statsMtx.RUnlock()

	return lastPingMicros
}

// VersionKnown returns the whether or not the version of a peer is known
// locally.
//
// This function is safe for concurrent access.
func (peer *Peer) VersionKnown() bool {
	peer.flagsMtx.Lock()
	versionKnown := peer.versionKnown
	peer.flagsMtx.Unlock()

	return versionKnown
}

// VerAckReceived returns whether or not a verack message was received by the
// peer.
//
// This function is safe for concurrent access.
func (peer *Peer) VerAckReceived() bool {
	peer.flagsMtx.Lock()
	verAckReceived := peer.verAckReceived
	peer.flagsMtx.Unlock()

	return verAckReceived
}

// ProtocolVersion returns the negotiated peer protocol version.
//
// This function is safe for concurrent access.
func (peer *Peer) ProtocolVersion() uint32 {
	peer.flagsMtx.Lock()
	protocolVersion := peer.protocolVersion
	peer.flagsMtx.Unlock()

	return protocolVersion
}

// LastBlock returns the last block of the peer.
//
// This function is safe for concurrent access.
func (peer *Peer) LastBlock() int32 {
	peer.statsMtx.RLock()
	lastBlock := peer.lastBlock
	peer.statsMtx.RUnlock()

	return lastBlock
}

// LastSend returns the last send time of the peer.
//
// This function is safe for concurrent access.
func (peer *Peer) LastSend() time.Time {
	return time.Unix(atomic.LoadInt64(&peer.lastSend), 0)
}

// LastRecv returns the last recv time of the peer.
//
// This function is safe for concurrent access.
func (peer *Peer) LastRecv() time.Time {
	return time.Unix(atomic.LoadInt64(&peer.lastRecv), 0)
}

// LocalAddr returns the local address of the connection.
//
// This function is safe fo concurrent access.
func (peer *Peer) LocalAddr() net.Addr {
	var localAddr net.Addr
	if atomic.LoadInt32(&peer.connected) != 0 {
		localAddr = peer.conn.LocalAddr()
	}
	return localAddr
}

// BytesSent returns the total number of bytes sent by the peer.
//
// This function is safe for concurrent access.
func (peer *Peer) BytesSent() uint64 {
	return atomic.LoadUint64(&peer.bytesSent)
}

// BytesReceived returns the total number of bytes received by the peer.
//
// This function is safe for concurrent access.
func (peer *Peer) BytesReceived() uint64 {
	return atomic.LoadUint64(&peer.bytesReceived)
}

// TimeConnected returns the time at which the peer connected.
//
// This function is safe for concurrent access.
func (peer *Peer) TimeConnected() time.Time {
	peer.statsMtx.RLock()
	timeConnected := peer.timeConnected
	peer.statsMtx.RUnlock()

	return timeConnected
}

// TimeOffset returns the number of seconds the local time was offset from the
// time the peer reported during the initial negotiation phase.  Negative values
// indicate the remote peer's time is before the local time.
//
// This function is safe for concurrent access.
func (peer *Peer) TimeOffset() int64 {
	peer.statsMtx.RLock()
	timeOffset := peer.timeOffset
	peer.statsMtx.RUnlock()

	return timeOffset
}

// StartingHeight returns the last known height the peer reported during the
// initial negotiation phase.
//
// This function is safe for concurrent access.
func (peer *Peer) StartingHeight() int32 {
	peer.statsMtx.RLock()
	startingHeight := peer.startingHeight
	peer.statsMtx.RUnlock()

	return startingHeight
}

// WantsHeaders returns if the peer wants header messages instead of
// inventory vectors for blocks.
//
// This function is safe for concurrent access.
func (peer *Peer) WantsHeaders() bool {
	peer.flagsMtx.Lock()
	sendHeadersPreferred := peer.sendHeadersPreferred
	peer.flagsMtx.Unlock()

	return sendHeadersPreferred
}

// IsWitnessEnabled returns true if the peer has signalled that it supports
// segregated witness.
//
// This function is safe for concurrent access.
func (peer *Peer) IsWitnessEnabled() bool {
	peer.flagsMtx.Lock()
	witnessEnabled := peer.witnessEnabled
	peer.flagsMtx.Unlock()

	return witnessEnabled
}

// PushAddrMsg sends an addr message to the connected peer using the provided
// addresses.  This function is useful over manually sending the message via
// QueueMessage since it automatically limits the addresses to the maximum
// number allowed by the message and randomizes the chosen addresses when there
// are too many.  It returns the addresses that were actually sent and no
// message will be sent if there are no entries in the provided addresses slice.
//
// This function is safe for concurrent access.
func (peer *Peer) PushAddrMsg(addresses []*wire.NetAddress) ([]*wire.NetAddress, error) {
	addressCount := len(addresses)

	// Nothing to send.
	if addressCount == 0 {
		return nil, nil
	}

	msg := wire.NewMsgAddr()
	msg.AddrList = make([]*wire.NetAddress, addressCount)
	copy(msg.AddrList, addresses)

	// Randomize the addresses sent if there are more than the maximum allowed.
	if addressCount > wire.MaxAddrPerMsg {
		// Shuffle the address list.
		for i := 0; i < wire.MaxAddrPerMsg; i++ {
			j := i + rand.Intn(addressCount-i)
			msg.AddrList[i], msg.AddrList[j] = msg.AddrList[j], msg.AddrList[i]
		}

		// Truncate it to the maximum size.
		msg.AddrList = msg.AddrList[:wire.MaxAddrPerMsg]
	}

	peer.QueueMessage(msg, nil)
	return msg.AddrList, nil
}

// PushGetBlocksMsg sends a getblocks message for the provided block locator
// and stop hash.  It will ignore back-to-back duplicate requests.
//
// This function is safe for concurrent access.
func (peer *Peer) PushGetBlocksMsg(locator blockchain.BlockLocator, stopHash *chainhash.Hash) error {
	// Extract the begin hash from the block locator, if one was specified,
	// to use for filtering duplicate getblocks requests.
	var beginHash *chainhash.Hash
	if len(locator) > 0 {
		beginHash = locator[0]
	}

	// Filter duplicate getblocks requests.
	peer.prevGetBlocksMtx.Lock()
	isDuplicate := peer.prevGetBlocksStop != nil && peer.prevGetBlocksBegin != nil &&
		beginHash != nil && stopHash.IsEqual(peer.prevGetBlocksStop) &&
		beginHash.IsEqual(peer.prevGetBlocksBegin)
	peer.prevGetBlocksMtx.Unlock()

	if isDuplicate {
		log.Tracef("Filtering duplicate [getblocks] with begin "+
			"hash %v, stop hash %v", beginHash, stopHash)
		return nil
	}

	// Construct the getblocks request and queue it to be sent.
	msg := wire.NewMsgGetBlocks(stopHash)
	for _, hash := range locator {
		err := msg.AddBlockLocatorHash(hash)
		if err != nil {
			return err
		}
	}
	peer.QueueMessage(msg, nil)

	// Update the previous getblocks request information for filtering
	// duplicates.
	peer.prevGetBlocksMtx.Lock()
	peer.prevGetBlocksBegin = beginHash
	peer.prevGetBlocksStop = stopHash
	peer.prevGetBlocksMtx.Unlock()
	return nil
}

// PushGetHeadersMsg sends a getblocks message for the provided block locator
// and stop hash.  It will ignore back-to-back duplicate requests.
//
// This function is safe for concurrent access.
func (peer *Peer) PushGetHeadersMsg(locator blockchain.BlockLocator, stopHash *chainhash.Hash) error {
	// Extract the begin hash from the block locator, if one was specified,
	// to use for filtering duplicate getheaders requests.
	var beginHash *chainhash.Hash
	if len(locator) > 0 {
		beginHash = locator[0]
	}

	// Filter duplicate getheaders requests.
	peer.prevGetHdrsMtx.Lock()
	isDuplicate := peer.prevGetHdrsStop != nil && peer.prevGetHdrsBegin != nil &&
		beginHash != nil && stopHash.IsEqual(peer.prevGetHdrsStop) &&
		beginHash.IsEqual(peer.prevGetHdrsBegin)
	peer.prevGetHdrsMtx.Unlock()

	if isDuplicate {
		log.Tracef("Filtering duplicate [getheaders] with begin hash %v",
			beginHash)
		return nil
	}

	// Construct the getheaders request and queue it to be sent.
	msg := wire.NewMsgGetHeaders()
	msg.HashStop = *stopHash
	for _, hash := range locator {
		err := msg.AddBlockLocatorHash(hash)
		if err != nil {
			return err
		}
	}
	peer.QueueMessage(msg, nil)

	// Update the previous getheaders request information for filtering
	// duplicates.
	peer.prevGetHdrsMtx.Lock()
	peer.prevGetHdrsBegin = beginHash
	peer.prevGetHdrsStop = stopHash
	peer.prevGetHdrsMtx.Unlock()
	return nil
}

// PushRejectMsg sends a reject message for the provided command, reject code,
// reject reason, and hash.  The hash will only be used when the command is a tx
// or block and should be nil in other cases.  The wait parameter will cause the
// function to block until the reject message has actually been sent.
//
// This function is safe for concurrent access.
func (peer *Peer) PushRejectMsg(command string, code wire.RejectCode, reason string, hash *chainhash.Hash, wait bool) {
	// Don't bother sending the reject message if the protocol version
	// is too low.
	if peer.VersionKnown() && peer.ProtocolVersion() < wire.RejectVersion {
		return
	}

	msg := wire.NewMsgReject(command, code, reason)
	if command == wire.CmdTx || command == wire.CmdBlock {
		if hash == nil {
			log.Warnf("Sending a reject message for command "+
				"type %v which should have specified a hash "+
				"but does not", command)
			hash = &zeroHash
		}
		msg.Hash = *hash
	}

	// Send the message without waiting if the caller has not requested it.
	if !wait {
		peer.QueueMessage(msg, nil)
		return
	}

	// Send the message and block until it has been sent before returning.
	doneChan := make(chan struct{}, 1)
	peer.QueueMessage(msg, doneChan)
	<-doneChan
}

// handlePingMsg is invoked when a peer receives a ping bitcoin message.  For
// recent clients (protocol version > BIP0031Version), it replies with a pong
// message.  For older clients, it does nothing and anything other than failure
// is considered a successful ping.
func (peer *Peer) handlePingMsg(msg *wire.MsgPing) {
	// Only reply with pong if the message is from a new enough client.
	if peer.ProtocolVersion() > wire.BIP0031Version {
		// Include nonce from ping so pong can be identified.
		peer.QueueMessage(wire.NewMsgPong(msg.Nonce), nil)
	}
}

// handlePongMsg is invoked when a peer receives a pong bitcoin message.  It
// updates the ping statistics as required for recent clients (protocol
// version > BIP0031Version).  There is no effect for older clients or when a
// ping was not previously sent.
func (peer *Peer) handlePongMsg(msg *wire.MsgPong) {
	// Arguably we could use a buffered channel here sending data
	// in a fifo manner whenever we send a ping, or a list keeping track of
	// the times of each ping. For now we just make a best effort and
	// only record stats if it was for the last ping sent. Any preceding
	// and overlapping pings will be ignored. It is unlikely to occur
	// without large usage of the ping rpc call since we ping infrequently
	// enough that if they overlap we would have timed out the peer.
	if peer.ProtocolVersion() > wire.BIP0031Version {
		peer.statsMtx.Lock()
		if peer.lastPingNonce != 0 && msg.Nonce == peer.lastPingNonce {
			peer.lastPingMicros = time.Since(peer.lastPingTime).Nanoseconds()
			peer.lastPingMicros /= 1000 // convert to usec.
			peer.lastPingNonce = 0
		}
		peer.statsMtx.Unlock()
	}
}

// readMessage reads the next bitcoin message from the peer with logging.
func (peer *Peer) readMessage(encoding encoder.MessageEncoding) (wire.Message, []byte, error) {
	n, msg, buf, err := wire.ReadMessageWithEncodingN(peer.chain, peer.conn,
		peer.ProtocolVersion(), peer.cfg.ChainParams.Net, encoding)
	atomic.AddUint64(&peer.bytesReceived, uint64(n))
	if peer.cfg.Listeners.OnRead != nil {
		peer.cfg.Listeners.OnRead(peer, n, msg, err)
	}
	if err != nil {
		return nil, nil, err
	}

	// Use closures to log expensive operations so they are only run when
	// the logging level requires it.
	// log.Debugf("%v", newLogClosure(func() string {
	// 	// Debug summary of message.
	// 	summary := messageSummary(msg)
	// 	if len(summary) > 0 {
	// 		summary = " (" + summary + ")"
	// 	}
	// 	return fmt.Sprintf("Received %v%s from %s",
	// 		msg.Command(), summary, peer)
	// }))
	// log.Tracef("%v", newLogClosure(func() string {
	// 	return spew.Sdump(msg)
	// }))
	// log.Tracef("%v", newLogClosure(func() string {
	// 	return spew.Sdump(buf)
	// }))

	return msg, buf, nil
}

// writeMessage sends a bitcoin message to the peer with logging.
func (peer *Peer) writeMessage(msg wire.Message, enc encoder.MessageEncoding) error {
	// Don't do anything if we're disconnecting.
	if atomic.LoadInt32(&peer.disconnect) != 0 {
		return nil
	}

	// Use closures to log expensive operations so they are only run when
	// the logging level requires it.
	// log.Debugf("%v", newLogClosure(func() string {
	// 	// Debug summary of message.
	// 	summary := messageSummary(msg)
	// 	if len(summary) > 0 {
	// 		summary = " (" + summary + ")"
	// 	}
	// 	return fmt.Sprintf("Sending %v%s to %s", msg.Command(),
	// 		summary, peer)
	// }))
	// log.Tracef("%v", newLogClosure(func() string {
	// 	return spew.Sdump(msg)
	// }))
	// log.Tracef("%v", newLogClosure(func() string {
	// 	var buf bytes.Buffer
	// 	_, err := wire.WriteMessageWithEncodingN(&buf, msg, peer.ProtocolVersion(),
	// 		peer.cfg.ChainParams.Net, enc)
	// 	if err != nil {
	// 		return err.Error()
	// 	}
	// 	return spew.Sdump(buf.Bytes())
	// }))

	// Write the message to the peer.
	n, err := wire.WriteMessageWithEncodingN(peer.conn, msg,
		peer.ProtocolVersion(), peer.cfg.ChainParams.Net, enc)
	atomic.AddUint64(&peer.bytesSent, uint64(n))
	if peer.cfg.Listeners.OnWrite != nil {
		peer.cfg.Listeners.OnWrite(peer, n, msg, err)
	}
	return err
}

// isAllowedReadError returns whether or not the passed error is allowed without
// disconnecting the peer.  In particular, regression tests need to be allowed
// to send malformed messages without the peer being disconnected.
func (peer *Peer) isAllowedReadError(err error) bool {
	// Only allow read errors in regression test mode.
	if peer.cfg.ChainParams.Net != types.TestNet {
		return false
	}

	// Don't allow the error if it's not specifically a malformed message error.
	if _, ok := err.(*wire.MessageError); !ok {
		return false
	}

	// Don't allow the error if it's not coming from localhost or the
	// hostname can't be determined for some reason.
	host, _, err := net.SplitHostPort(peer.addr)
	if err != nil {
		return false
	}

	if host != "127.0.0.1" && host != "localhost" {
		return false
	}

	// Allowed if all checks passed.
	return true
}

// shouldHandleReadError returns whether or not the passed error, which is
// expected to have come from reading from the remote peer in the inHandler,
// should be logged and responded to with a reject message.
func (peer *Peer) shouldHandleReadError(err error) bool {
	// No logging or reject message when the peer is being forcibly
	// disconnected.
	if atomic.LoadInt32(&peer.disconnect) != 0 {
		return false
	}

	// No logging or reject message when the remote peer has been
	// disconnected.
	if err == io.EOF {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
		return false
	}

	return true
}

// maybeAddDeadline potentially adds a deadline for the appropriate expected
// response for the passed wire protocol command to the pending responses map.
func (peer *Peer) maybeAddDeadline(pendingResponses map[string]time.Time, msgCmd string) {
	// Setup a deadline for each message being sent that expects a response.
	//
	// NOTE: Pings are intentionally ignored here since they are typically
	// sent asynchronously and as a result of a long backlock of messages,
	// such as is typical in the case of initial block download, the
	// response won't be received in time.
	deadline := time.Now().Add(stallResponseTimeout)
	switch msgCmd {
	case wire.CmdVersion:
		// Expects a verack message.
		pendingResponses[wire.CmdVerAck] = deadline

	case wire.CmdMemPool:
		// Expects an inv message.
		pendingResponses[wire.CmdInv] = deadline

	case wire.CmdGetBlocks:
		// Expects an inv message.
		pendingResponses[wire.CmdInv] = deadline

	case wire.CmdGetData:
		// Expects a block, merkleblock, tx, or notfound message.
		pendingResponses[wire.CmdBlock] = deadline
		pendingResponses[wire.CmdMerkleBlock] = deadline
		pendingResponses[wire.CmdTx] = deadline
		pendingResponses[wire.CmdNotFound] = deadline

	case wire.CmdGetHeaders:
		// Expects a headers message.  Use a longer deadline since it
		// can take a while for the remote peer to load all of the
		// headers.
		deadline = time.Now().Add(stallResponseTimeout * 3)
		pendingResponses[wire.CmdHeaders] = deadline
	}
}

// stallHandler handles stall detection for the peer.  This entails keeping
// track of expected responses and assigning them deadlines while accounting for
// the time spent in callbacks.  It must be run as a goroutine.
func (peer *Peer) stallHandler() {
	// These variables are used to adjust the deadline times forward by the
	// time it takes callbacks to execute.  This is done because new
	// messages aren't read until the previous one is finished processing
	// (which includes callbacks), so the deadline for receiving a response
	// for a given message must account for the processing time as well.
	var handlerActive bool
	var handlersStartTime time.Time
	var deadlineOffset time.Duration

	// pendingResponses tracks the expected response deadline times.
	pendingResponses := make(map[string]time.Time)

	// stallTicker is used to periodically check pending responses that have
	// exceeded the expected deadline and disconnect the peer due to
	// stalling.
	stallTicker := time.NewTicker(stallTickInterval)
	defer stallTicker.Stop()

	// ioStopped is used to detect when both the input and output handler
	// goroutines are done.
	var ioStopped bool
out:
	for {
		select {
		case msg := <-peer.stallControl:
			switch msg.command {
			case sccSendMessage:
				// Add a deadline for the expected response
				// message if needed.
				peer.maybeAddDeadline(pendingResponses,
					msg.message.Command())

			case sccReceiveMessage:
				// Remove received messages from the expected
				// response map.  Since certain commands expect
				// one of a group of responses, remove
				// everything in the expected group accordingly.
				switch msgCmd := msg.message.Command(); msgCmd {
				case wire.CmdBlock:
					fallthrough
				case wire.CmdMerkleBlock:
					fallthrough
				case wire.CmdTx:
					fallthrough
				case wire.CmdNotFound:
					delete(pendingResponses, wire.CmdBlock)
					delete(pendingResponses, wire.CmdMerkleBlock)
					delete(pendingResponses, wire.CmdTx)
					delete(pendingResponses, wire.CmdNotFound)

				default:
					delete(pendingResponses, msgCmd)
				}

			case sccHandlerStart:
				// Warn on unbalanced callback signalling.
				if handlerActive {
					log.Warn("Received handler start " +
						"control command while a " +
						"handler is already active")
					continue
				}

				handlerActive = true
				handlersStartTime = time.Now()

			case sccHandlerDone:
				// Warn on unbalanced callback signalling.
				if !handlerActive {
					log.Warn("Received handler done " +
						"control command when a " +
						"handler is not already active")
					continue
				}

				// Extend active deadlines by the time it took
				// to execute the callback.
				duration := time.Since(handlersStartTime)
				deadlineOffset += duration
				handlerActive = false

			default:
				log.Warnf("Unsupported message command %v",
					msg.command)
			}

		case <-stallTicker.C:
			// Calculate the offset to apply to the deadline based
			// on how long the handlers have taken to execute since
			// the last tick.
			now := time.Now()
			offset := deadlineOffset
			if handlerActive {
				offset += now.Sub(handlersStartTime)
			}

			// Disconnect the peer if any of the pending responses
			// don't arrive by their adjusted deadline.
			for command, deadline := range pendingResponses {
				if now.Before(deadline.Add(offset)) {
					continue
				}

				log.Debugf("Peer %s appears to be stalled or "+
					"misbehaving, %s timeout -- "+
					"disconnecting", peer, command)
				peer.Disconnect()
				break
			}

			// Reset the deadline offset for the next tick.
			deadlineOffset = 0

		case <-peer.inQuit:
			// The stall handler can exit once both the input and
			// output handler goroutines are done.
			if ioStopped {
				break out
			}
			ioStopped = true

		case <-peer.outQuit:
			// The stall handler can exit once both the input and
			// output handler goroutines are done.
			if ioStopped {
				break out
			}
			ioStopped = true
		}
	}

	// Drain any wait channels before going away so there is nothing left
	// waiting on this goroutine.
cleanup:
	for {
		select {
		case <-peer.stallControl:
		default:
			break cleanup
		}
	}
	log.Tracef("Peer stall handler done for %s", peer)
}

// inHandler handles all incoming messages for the peer.  It must be run as a
// goroutine.
func (peer *Peer) inHandler() {
	// The timer is stopped when a new message is received and reset after it
	// is processed.
	idleTimer := time.AfterFunc(idleTimeout, func() {
		log.Warnf("Peer %s no answer for %s -- disconnecting", peer, idleTimeout)
		peer.log.Debug(fmt.Sprintf("Peer %s no answer for %s -- disconnecting", peer, idleTimeout))

		peer.Disconnect()
	})

	peer.log.Debug("start read loop")

out:
	for atomic.LoadInt32(&peer.disconnect) == 0 {
		// Read a message and stop the idle timer as soon as the read
		// is done.  The timer is reset below for the next iteration if
		// needed.
		rmsg, buf, err := peer.readMessage(peer.wireEncoding)
		idleTimer.Stop()
		if err != nil {
			// In order to allow regression tests with malformed messages, don't
			// disconnect the peer when we're in regression test mode and the
			// error is one of the allowed errors.
			if peer.isAllowedReadError(err) {
				log.Errorf("Allowed test error from %s: %v", peer, err)
				idleTimer.Reset(idleTimeout)
				continue
			}

			// Only log the error and send reject message if the
			// local peer is not forcibly disconnecting and the
			// remote peer has not disconnected.
			if peer.shouldHandleReadError(err) {
				errMsg := fmt.Sprintf("Can't read message from %s: %v", peer, err)
				if err != io.ErrUnexpectedEOF {
					log.Errorf(errMsg)
				}

				// Push a reject message for the malformed message and wait for
				// the message to be sent before disconnecting.
				//
				// NOTE: Ideally this would include the command in the header if
				// at least that much of the message was valid, but that is not
				// currently exposed by wire, so just used malformed for the
				// command.
				peer.PushRejectMsg("malformed", wire.RejectMalformed, errMsg, nil,
					true)
			}
			break out
		}

		peer.log.Debug("new income message from peer",
			zap.String("msg_type", fmt.Sprintf("%T", rmsg)))

		atomic.StoreInt64(&peer.lastRecv, time.Now().Unix())
		peer.stallControl <- stallControlMsg{sccReceiveMessage, rmsg}

		// Handle each supported message type.
		peer.stallControl <- stallControlMsg{sccHandlerStart, rmsg}
		switch msg := rmsg.(type) {
		case *wire.MsgVersion:
			// Limit to one version message per peer.
			peer.PushRejectMsg(msg.Command(), wire.RejectDuplicate,
				"duplicate version message", nil, true)
			break out

		case *wire.MsgVerAck:
			// Limit to one verack message per peer.
			peer.PushRejectMsg(
				msg.Command(), wire.RejectDuplicate,
				"duplicate verack message", nil, true,
			)
			break out

		case *wire.MsgGetAddr:
			if peer.cfg.Listeners.OnGetAddr != nil {
				peer.cfg.Listeners.OnGetAddr(peer, msg)
			}

		case *wire.MsgAddr:
			if peer.cfg.Listeners.OnAddr != nil {
				peer.cfg.Listeners.OnAddr(peer, msg)
			}

		case *wire.MsgPing:
			peer.handlePingMsg(msg)
			if peer.cfg.Listeners.OnPing != nil {
				peer.cfg.Listeners.OnPing(peer, msg)
			}

		case *wire.MsgPong:
			peer.handlePongMsg(msg)
			if peer.cfg.Listeners.OnPong != nil {
				peer.cfg.Listeners.OnPong(peer, msg)
			}

		case *wire.MsgAlert:
			if peer.cfg.Listeners.OnAlert != nil {
				peer.cfg.Listeners.OnAlert(peer, msg)
			}

		case *wire.MsgMemPool:
			if peer.cfg.Listeners.OnMemPool != nil {
				peer.cfg.Listeners.OnMemPool(peer, msg)
			}

		case *wire.MsgTx:
			if peer.cfg.Listeners.OnTx != nil {
				peer.cfg.Listeners.OnTx(peer, msg)
			}

		case *wire.MsgBlock:
			if peer.cfg.Listeners.OnBlock != nil {
				peer.cfg.Listeners.OnBlock(peer, msg, buf)
			}

		case *wire.MsgInv:
			if peer.cfg.Listeners.OnInv != nil {
				peer.cfg.Listeners.OnInv(peer, msg)
			}

		case *wire.MsgHeaders:
			if peer.cfg.Listeners.OnHeaders != nil {
				peer.cfg.Listeners.OnHeaders(peer, msg)
			}

		case *wire.MsgNotFound:
			if peer.cfg.Listeners.OnNotFound != nil {
				peer.cfg.Listeners.OnNotFound(peer, msg)
			}

		case *wire.MsgGetData:
			if peer.cfg.Listeners.OnGetData != nil {
				peer.cfg.Listeners.OnGetData(peer, msg)
			}

		case *wire.MsgGetBlocks:
			if peer.cfg.Listeners.OnGetBlocks != nil {
				peer.cfg.Listeners.OnGetBlocks(peer, msg)
			}

		case *wire.MsgGetHeaders:
			if peer.cfg.Listeners.OnGetHeaders != nil {
				peer.cfg.Listeners.OnGetHeaders(peer, msg)
			}

		case *wire.MsgGetCFilters:
			if peer.cfg.Listeners.OnGetCFilters != nil {
				peer.cfg.Listeners.OnGetCFilters(peer, msg)
			}

		case *wire.MsgGetCFHeaders:
			if peer.cfg.Listeners.OnGetCFHeaders != nil {
				peer.cfg.Listeners.OnGetCFHeaders(peer, msg)
			}

		case *wire.MsgGetCFCheckpt:
			if peer.cfg.Listeners.OnGetCFCheckpt != nil {
				peer.cfg.Listeners.OnGetCFCheckpt(peer, msg)
			}

		case *wire.MsgCFilter:
			if peer.cfg.Listeners.OnCFilter != nil {
				peer.cfg.Listeners.OnCFilter(peer, msg)
			}

		case *wire.MsgCFHeaders:
			if peer.cfg.Listeners.OnCFHeaders != nil {
				peer.cfg.Listeners.OnCFHeaders(peer, msg)
			}

		case *wire.MsgFeeFilter:
			if peer.cfg.Listeners.OnFeeFilter != nil {
				peer.cfg.Listeners.OnFeeFilter(peer, msg)
			}

		case *wire.MsgFilterAdd:
			if peer.cfg.Listeners.OnFilterAdd != nil {
				peer.cfg.Listeners.OnFilterAdd(peer, msg)
			}

		case *wire.MsgFilterClear:
			if peer.cfg.Listeners.OnFilterClear != nil {
				peer.cfg.Listeners.OnFilterClear(peer, msg)
			}

		case *wire.MsgFilterLoad:
			if peer.cfg.Listeners.OnFilterLoad != nil {
				peer.cfg.Listeners.OnFilterLoad(peer, msg)
			}

		case *wire.MsgMerkleBlock:
			if peer.cfg.Listeners.OnMerkleBlock != nil {
				peer.cfg.Listeners.OnMerkleBlock(peer, msg)
			}

		case *wire.MsgReject:
			if peer.cfg.Listeners.OnReject != nil {
				peer.cfg.Listeners.OnReject(peer, msg)
			}

		case *wire.MsgSendHeaders:
			peer.flagsMtx.Lock()
			peer.sendHeadersPreferred = true
			peer.flagsMtx.Unlock()

			if peer.cfg.Listeners.OnSendHeaders != nil {
				peer.cfg.Listeners.OnSendHeaders(peer, msg)
			}

		default:
			log.Debugf("Received unhandled message of type %v "+
				"from %v", rmsg.Command(), peer)
		}
		peer.stallControl <- stallControlMsg{sccHandlerDone, rmsg}

		// A message was received so reset the idle timer.
		idleTimer.Reset(idleTimeout)
	}

	// Ensure the idle timer is stopped to avoid leaking the resource.
	idleTimer.Stop()

	// Ensure connection is closed.
	peer.Disconnect()

	close(peer.inQuit)
	log.Tracef("Peer input handler done for %s", peer)
}

// queueHandler handles the queuing of outgoing data for the peer. This runs as
// a muxer for various sources of input so we can ensure that server and peer
// handlers will not block on us sending a message.  That data is then passed on
// to outHandler to be actually written.
func (peer *Peer) queueHandler() {
	pendingMsgs := list.New()
	invSendQueue := list.New()
	trickleTicker := time.NewTicker(peer.cfg.TrickleInterval)
	defer trickleTicker.Stop()

	// We keep the waiting flag so that we know if we have a message queued
	// to the outHandler or not.  We could use the presence of a head of
	// the list for this but then we have rather racy concerns about whether
	// it has gotten it at cleanup time - and thus who sends on the
	// message's done channel.  To avoid such confusion we keep a different
	// flag and pendingMsgs only contains messages that we have not yet
	// passed to outHandler.
	waiting := false

	// To avoid duplication below.
	queuePacket := func(msg outMsg, list *list.List, waiting bool) bool {
		if !waiting {
			peer.sendQueue <- msg
		} else {
			list.PushBack(msg)
		}
		// we are always waiting now.
		return true
	}
out:
	for {
		select {
		case msg := <-peer.outputQueue:
			waiting = queuePacket(msg, pendingMsgs, waiting)

		// This channel is notified when a message has been sent across
		// the network socket.
		case <-peer.sendDoneQueue:
			// No longer waiting if there are no more messages
			// in the pending messages queue.
			next := pendingMsgs.Front()
			if next == nil {
				waiting = false
				continue
			}

			// Notify the outHandler about the next item to
			// asynchronously send.
			val := pendingMsgs.Remove(next)
			peer.sendQueue <- val.(outMsg)

		case iv := <-peer.outputInvChan:
			// No handshake?  They'll find out soon enough.
			if peer.VersionKnown() {
				// If this is a new block, then we'll blast it
				// out immediately, sipping the inv trickle
				// queue.
				if iv.Type == types.InvTypeBlock ||
					iv.Type == types.InvTypeWitnessBlock {

					invMsg := wire.NewMsgInvSizeHint(1)
					invMsg.AddInvVect(iv)
					waiting = queuePacket(outMsg{msg: invMsg},
						pendingMsgs, waiting)
				} else {
					invSendQueue.PushBack(iv)
				}
			}

		case <-trickleTicker.C:
			// Don't send anything if we're disconnecting or there
			// is no queued inventory.
			// version is known if send queue has any entries.
			if atomic.LoadInt32(&peer.disconnect) != 0 ||
				invSendQueue.Len() == 0 {
				continue
			}

			// Create and send as many inv messages as needed to
			// drain the inventory send queue.
			invMsg := wire.NewMsgInvSizeHint(uint(invSendQueue.Len()))
			for e := invSendQueue.Front(); e != nil; e = invSendQueue.Front() {
				iv := invSendQueue.Remove(e).(*types.InvVect)

				// Don't send inventory that became known after
				// the initial check.
				if peer.knownInventory.Exists(iv) {
					continue
				}

				invMsg.AddInvVect(iv)
				if len(invMsg.InvList) >= maxInvTrickleSize {
					waiting = queuePacket(
						outMsg{msg: invMsg},
						pendingMsgs, waiting)
					invMsg = wire.NewMsgInvSizeHint(uint(invSendQueue.Len()))
				}

				// Add the inventory that is being relayed to
				// the known inventory for the peer.
				peer.AddKnownInventory(iv)
			}
			if len(invMsg.InvList) > 0 {
				waiting = queuePacket(outMsg{msg: invMsg},
					pendingMsgs, waiting)
			}

		case <-peer.quit:
			break out
		}
	}

	// Drain any wait channels before we go away so we don't leave something
	// waiting for us.
	for e := pendingMsgs.Front(); e != nil; e = pendingMsgs.Front() {
		val := pendingMsgs.Remove(e)
		msg := val.(outMsg)
		if msg.doneChan != nil {
			msg.doneChan <- struct{}{}
		}
	}
cleanup:
	for {
		select {
		case msg := <-peer.outputQueue:
			if msg.doneChan != nil {
				msg.doneChan <- struct{}{}
			}
		case <-peer.outputInvChan:
			// Just drain channel
		// sendDoneQueue is buffered so doesn't need draining.
		default:
			break cleanup
		}
	}
	close(peer.queueQuit)
	log.Tracef("Peer queue handler done for %s", peer)
}

// shouldLogWriteError returns whether or not the passed error, which is
// expected to have come from writing to the remote peer in the outHandler,
// should be logged.
func (peer *Peer) shouldLogWriteError(err error) bool {
	// No logging when the peer is being forcibly disconnected.
	if atomic.LoadInt32(&peer.disconnect) != 0 {
		return false
	}

	// No logging when the remote peer has been disconnected.
	if err == io.EOF {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
		return false
	}

	return true
}

// outHandler handles all outgoing messages for the peer.  It must be run as a
// goroutine.  It uses a buffered channel to serialize output messages while
// allowing the sender to continue running asynchronously.
func (peer *Peer) outHandler() {
out:
	for {
		select {
		case msg := <-peer.sendQueue:
			switch m := msg.msg.(type) {
			case *wire.MsgPing:
				// Only expects a pong message in later protocol
				// versions.  Also set up statistics.
				if peer.ProtocolVersion() > wire.BIP0031Version {
					peer.statsMtx.Lock()
					peer.lastPingNonce = m.Nonce
					peer.lastPingTime = time.Now()
					peer.statsMtx.Unlock()
				}
			}

			peer.stallControl <- stallControlMsg{sccSendMessage, msg.msg}

			err := peer.writeMessage(msg.msg, msg.encoding)
			if err != nil {
				if peer.shouldLogWriteError(err) {
					log.Errorf("Failed to send message to "+
						"%s: %v", peer, err)
				}
				peer.Disconnect()
				if msg.doneChan != nil {
					msg.doneChan <- struct{}{}
				}
				continue
			}

			// At this point, the message was successfully sent, so
			// update the last send time, signal the sender of the
			// message that it has been sent (if requested), and
			// signal the send queue to the deliver the next queued
			// message.
			atomic.StoreInt64(&peer.lastSend, time.Now().Unix())
			if msg.doneChan != nil {
				msg.doneChan <- struct{}{}
			}
			peer.sendDoneQueue <- struct{}{}

		case <-peer.quit:
			break out
		}
	}

	<-peer.queueQuit

	// Drain any wait channels before we go away so we don't leave something
	// waiting for us. We have waited on queueQuit and thus we can be sure
	// that we will not miss anything sent on sendQueue.
cleanup:
	for {
		select {
		case msg := <-peer.sendQueue:
			if msg.doneChan != nil {
				msg.doneChan <- struct{}{}
			}
			// no need to send on sendDoneQueue since queueHandler
			// has been waited on and already exited.
		default:
			break cleanup
		}
	}
	close(peer.outQuit)
	log.Tracef("Peer output handler done for %s", peer)
}

// pingHandler periodically pings the peer.  It must be run as a goroutine.
func (peer *Peer) pingHandler() {
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

out:
	for {
		select {
		case <-pingTicker.C:
			nonce, err := encoder.RandomUint64()
			if err != nil {
				log.Errorf("Not sending ping to %s: %v", peer, err)
				continue
			}
			peer.QueueMessage(wire.NewMsgPing(nonce), nil)

		case <-peer.quit:
			break out
		}
	}
}

// QueueMessage adds the passed bitcoin message to the peer send queue.
//
// This function is safe for concurrent access.
func (peer *Peer) QueueMessage(msg wire.Message, doneChan chan<- struct{}) {
	peer.QueueMessageWithEncoding(msg, doneChan, wire.BaseEncoding)
}

// QueueMessageWithEncoding adds the passed bitcoin message to the peer send
// queue. This function is identical to QueueMessage, however it allows the
// caller to specify the wire encoding type that should be used when
// encoding/decoding blocks and transactions.
//
// This function is safe for concurrent access.
func (peer *Peer) QueueMessageWithEncoding(msg wire.Message, doneChan chan<- struct{},
	encoding encoder.MessageEncoding) {

	// Avoid risk of deadlock if goroutine already exited.  The goroutine
	// we will be sending to hangs around until it knows for a fact that
	// it is marked as disconnected and *then* it drains the channels.
	if !peer.Connected() {
		if doneChan != nil {
			go func() {
				doneChan <- struct{}{}
			}()
		}
		return
	}
	peer.outputQueue <- outMsg{msg: msg, encoding: encoding, doneChan: doneChan}
}

// QueueInventory adds the passed inventory to the inventory send queue which
// might not be sent right away, rather it is trickled to the peer in batches.
// Inventory that the peer is already known to have is ignored.
//
// This function is safe for concurrent access.
func (peer *Peer) QueueInventory(invVect *types.InvVect) {
	// Don't add the inventory to the send queue if the peer is already
	// known to have it.
	if peer.knownInventory.Exists(invVect) {
		return
	}

	// Avoid risk of deadlock if goroutine already exited.  The goroutine
	// we will be sending to hangs around until it knows for a fact that
	// it is marked as disconnected and *then* it drains the channels.
	if !peer.Connected() {
		return
	}

	peer.outputInvChan <- invVect
}

// Connected returns whether or not the peer is currently connected.
//
// This function is safe for concurrent access.
func (peer *Peer) Connected() bool {
	return atomic.LoadInt32(&peer.connected) != 0 &&
		atomic.LoadInt32(&peer.disconnect) == 0
}

// Disconnect disconnects the peer by closing the connection.  Calling this
// function when the peer is already disconnected or in the process of
// disconnecting will have no effect.
func (peer *Peer) Disconnect() {
	if atomic.AddInt32(&peer.disconnect, 1) != 1 {
		return
	}

	log.Tracef("Disconnecting %s", peer)
	if atomic.LoadInt32(&peer.connected) != 0 {
		peer.conn.Close()
	}
	close(peer.quit)
}

// readRemoteVersionMsg waits for the next message to arrive from the remote
// peer.  If the next message is not a version message or the version is not
// acceptable then return an error.
func (peer *Peer) readRemoteVersionMsg() error {
	// Read their version message.
	remoteMsg, _, err := peer.readMessage(wire.LatestEncoding)
	if err != nil {
		return nil, err
	}

	var msg *wire.MsgVersion
	switch m := remoteMsg.(type) {
	case *wire.MsgVersion:
		msg = m

	case *wire.MsgPortRedirect:
		peer.redirectRequested = true
		peer.newAddress = wire.NewNetAddressIPPort(m.IP, uint16(m.Port), peer.services)

		return errors.New("redirect required")

	default:
		// Notify and disconnect clients if the first message is not a version
		// message.
		reason := "a version message must precede all others"
		rejectMsg := wire.NewMsgReject(msg.Command(), wire.RejectMalformed,
			reason)

		if err = peer.writeMessage(rejectMsg, wire.LatestEncoding); err != nil {
			return err
		}
		return errors.New(reason)
	}

	// Detect self connections.
	if !allowSelfConns && sentNonces.Exists(msg.Nonce) {
		return nil, errors.New("disconnecting peer connected to self")
	}

	if !inbound && peer.chain.ShardID() != msg.Shard {
		return nil, errors.New("shardID of remote peer is not match")
	}

	if peer.chain.ShardID() != msg.Shard {
		port, ok := peer.cfg.ChainsPortsProvider(msg.Shard)
		if !ok {
			reason := "shard with requested ID not found or not supported"
			rejectMsg := wire.NewMsgReject(msg.Command(), wire.RejectInvalid,
				reason)

			if err = peer.writeMessage(rejectMsg, wire.LatestEncoding); err != nil {
				return err
			}
			return errors.New(reason)
		}

		addr := peer.conn.LocalAddr()
		host, _, _ := net.SplitHostPort(addr.String())
		ip := net.ParseIP(host)

		respMsg := wire.NewMsgPortRedirect(msg.Shard, uint32(port), ip)
		if err = peer.writeMessage(respMsg, wire.LatestEncoding); err != nil {
			return err
		}

		peer.redirectRequested = true
		reason := "remote peer shardID not match; redirect instruction send"
		return errors.New(reason)
	}

	// Negotiate the protocol version and set the services to what the remote
	// peer advertised.
	peer.flagsMtx.Lock()
	peer.advertisedProtoVer = uint32(msg.ProtocolVersion)
	peer.protocolVersion = minUint32(peer.protocolVersion, peer.advertisedProtoVer)
	peer.versionKnown = true
	peer.services = msg.Services
	peer.flagsMtx.Unlock()
	log.Debugf("Negotiated protocol version %d for peer %s",
		peer.protocolVersion, peer)

	// Updating a bunch of stats including block based stats, and the
	// peer's time offset.
	peer.statsMtx.Lock()
	peer.lastBlock = msg.LastBlock
	peer.startingHeight = msg.LastBlock
	peer.timeOffset = msg.Timestamp.Unix() - time.Now().Unix()
	peer.statsMtx.Unlock()

	// Set the peer's ID, user agent, and potentially the flag which
	// specifies the witness support is enabled.
	peer.flagsMtx.Lock()
	peer.id = atomic.AddInt32(&nodeCount, 1)
	peer.userAgent = msg.UserAgent

	// Determine if the peer would like to receive witness data with
	// transactions, or not.
	if peer.services&wire.SFNodeWitness == wire.SFNodeWitness {
		peer.witnessEnabled = true
	}
	peer.flagsMtx.Unlock()

	// Once the version message has been exchanged, we're able to determine
	// if this peer knows how to encode witness data over the wire
	// protocol. If so, then we'll switch to a decoding mode which is
	// prepared for the new transaction format introduced as part of
	// BIP0144.
	if peer.services&wire.SFNodeWitness == wire.SFNodeWitness {
		peer.wireEncoding = wire.WitnessEncoding
	}

	// Invoke the callback if specified.
	if peer.cfg.Listeners.OnVersion != nil {
		rejectMsg := peer.cfg.Listeners.OnVersion(peer, msg)
		if rejectMsg != nil {
			if err = peer.writeMessage(rejectMsg, wire.LatestEncoding); err != nil {
				return err
			}
			return errors.New(rejectMsg.Reason)
		}
	}

	// Notify and disconnect clients that have a protocol version that is
	// too old.
	//
	// NOTE: If minAcceptableProtocolVersion is raised to be higher than
	// wire.RejectVersion, this should send a reject packet before
	// disconnecting.
	if uint32(msg.ProtocolVersion) < MinAcceptableProtocolVersion {
		// Send a reject message indicating the protocol version is
		// obsolete and wait for the message to be sent before
		// disconnecting.
		reason := fmt.Sprintf("protocol version must be %d or greater",
			MinAcceptableProtocolVersion)
		rejectMsg := wire.NewMsgReject(msg.Command(), wire.RejectObsolete,
			reason)
		if err = peer.writeMessage(rejectMsg, wire.LatestEncoding); err != nil {
			return err
		}
		return errors.New(reason)
	}

	return nil, nil
}

// readRemoteVerAckMsg waits for the next message to arrive from the remote
// peer. If this message is not a verack message, then an error is returned.
// This method is to be used as part of the version negotiation upon a new
// connection.
func (peer *Peer) readRemoteVerAckMsg() error {
	// Read the next message from the wire.
	remoteMsg, _, err := peer.readMessage(wire.LatestEncoding)
	if err != nil {
		return err
	}

	// It should be a verack message, otherwise send a reject message to the
	// peer explaining why.
	msg, ok := remoteMsg.(*wire.MsgVerAck)
	if !ok {
		reason := "a verack message must follow version"
		rejectMsg := wire.NewMsgReject(
			msg.Command(), wire.RejectMalformed, reason,
		)
		_ = peer.writeMessage(rejectMsg, wire.LatestEncoding)
		return errors.New(reason)
	}

	peer.flagsMtx.Lock()
	peer.verAckReceived = true
	peer.flagsMtx.Unlock()

	if peer.cfg.Listeners.OnVerAck != nil {
		peer.cfg.Listeners.OnVerAck(peer, msg)
	}

	return nil
}

// localVersionMsg creates a version message that can be used to send to the
// remote peer.
func (peer *Peer) localVersionMsg() (*wire.MsgVersion, error) {
	var blockNum int32
	if peer.cfg.NewestBlock != nil {
		var err error
		_, blockNum, err = peer.cfg.NewestBlock()
		if err != nil {
			return nil, err
		}
	}

	theirNA := peer.na

	// If we are behind a proxy and the connection comes from the proxy then
	// we return an unroutable address as their address. This is to prevent
	// leaking the tor proxy address.
	if peer.cfg.Proxy != "" {
		proxyaddress, _, err := net.SplitHostPort(peer.cfg.Proxy)
		// invalid proxy means poorly configured, be on the safe side.
		if err != nil || peer.na.IP.String() == proxyaddress {
			theirNA = wire.NewNetAddressIPPort(net.IP([]byte{0, 0, 0, 0}), 0,
				theirNA.Services)
		}
	}

	// Create a wire.NetAddress with only the services set to use as the
	// "addrme" in the version message.
	//
	// Older nodes previously added the IP and port information to the
	// address manager which proved to be unreliable as an inbound
	// connection from a peer didn't necessarily mean the peer itself
	// accepted inbound connections.
	//
	// Also, the timestamp is unused in the version message.
	ourNA := &wire.NetAddress{
		Services: peer.cfg.Services,
	}

	// Generate a unique nonce for this peer so self connections can be
	// detected.  This is accomplished by adding it to a size-limited map of
	// recently seen nonces.
	nonce := uint64(rand.Int63())
	sentNonces.Add(nonce)

	// Version message.
	msg := wire.NewMsgVersion(peer.chain, ourNA, theirNA, nonce, blockNum)
	msg.AddUserAgent(peer.cfg.UserAgentName, peer.cfg.UserAgentVersion,
		peer.cfg.UserAgentComments...)

	// Advertise local services.
	msg.Services = peer.cfg.Services

	// Advertise our max supported protocol version.
	msg.ProtocolVersion = int32(peer.cfg.ProtocolVersion)

	// Advertise if inv messages for transactions are desired.
	msg.DisableRelayTx = peer.cfg.DisableRelayTx

	return msg, nil
}

// writeLocalVersionMsg writes our version message to the remote peer.
func (peer *Peer) writeLocalVersionMsg() error {
	localVerMsg, err := peer.localVersionMsg()
	if err != nil {
		return err
	}

	return peer.writeMessage(localVerMsg, wire.LatestEncoding)
}

// negotiateInboundProtocol performs the negotiation protocol for an inbound
// peer. The events should occur in the following order, otherwise an error is
// returned:
//
//   1. Remote peer sends their version.
//   2.1 We send our version [if shardID eq].
//   2.2 We send address of proper shard peer address [if shardID not eq].
//   3. We send our verack.
//   4. Remote peer sends their verack.
func (peer *Peer) negotiateInboundProtocol() error {
	if err := peer.readRemoteVersionMsg(); err != nil {
		if peer.redirectRequested && peer.newAddress != nil && peer.cfg.TriggerRedirect != nil {
			peer.cfg.TriggerRedirect(peer.na, peer.newAddress)
		}
		return err
	}

	if err := peer.writeLocalVersionMsg(); err != nil {
		return err
	}

	err := peer.writeMessage(wire.NewMsgVerAck(), wire.LatestEncoding)
	if err != nil {
		return err
	}

	return peer.readRemoteVerAckMsg()
}

// negotiateOutboundProtocol performs the negotiation protocol for an outbound
// peer. The events should occur in the following order, otherwise an error is
// returned:
//
//   1. We send our version.
//   2.1 Remote peer sends their version [if shardID eq].
//   2.2 Remote peer sends peer address [if shardID not eq].
//   3. Remote peer sends their verack.
//   4. We send our verack.
func (peer *Peer) negotiateOutboundProtocol() error {
	if err := peer.writeLocalVersionMsg(); err != nil {
		return err
	}

	if err := peer.readRemoteVersionMsg(); err != nil {
		if peer.redirectRequested && peer.newAddress != nil && peer.cfg.TriggerRedirect != nil {
			peer.cfg.TriggerRedirect(peer.na, peer.newAddress)
		}
		return err
	}

	if err := peer.readRemoteVerAckMsg(); err != nil {
		return err
	}

	return peer.writeMessage(wire.NewMsgVerAck(), wire.LatestEncoding)
}

// start begins processing input and output messages.
func (peer *Peer) start() error {
	log.Tracef("Starting peer %s", peer)

	negotiateErr := make(chan error, 1)
	go func() {
		if peer.inbound {
			negotiateErr <- peer.negotiateInboundProtocol()
		} else {
			negotiateErr <- peer.negotiateOutboundProtocol()
		}
	}()

	// Negotiate the protocol within the specified negotiateTimeout.
	select {
	case err := <-negotiateErr:
		if err != nil {
			if !peer.redirectRequested {
				peer.log.Error("negotiateErr", zap.String("remote_addr", peer.addr), zap.Error(err))
			}

			peer.Disconnect()
			return err
		}

	case <-time.After(negotiateTimeout):
		peer.Disconnect()
		return errors.New("protocol negotiation timeout")
	}

	peer.log.Debug("Connected to peer", zap.String("remote_addr", peer.Addr()))

	// The protocol has been negotiated successfully so start processing input
	// and output messages.
	go peer.stallHandler()
	go peer.inHandler()
	go peer.queueHandler()
	go peer.outHandler()
	go peer.pingHandler()

	return nil
}

// AssociateConnection associates the given conn to the peer. Calling this
// function when the peer is already connected will have no effect.
func (peer *Peer) AssociateConnection(conn net.Conn) {
	// Already connected?
	if !atomic.CompareAndSwapInt32(&peer.connected, 0, 1) {
		return
	}

	peer.conn = conn
	peer.timeConnected = time.Now()

	if peer.inbound {
		peer.addr = peer.conn.RemoteAddr().String()

		// Set up a NetAddress for the peer to be used with AddrManager.  We
		// only do this inbound because outbound set this up at connection time
		// and no point recomputing.
		na, err := newNetAddress(peer.conn.RemoteAddr(), peer.services)
		if err != nil {
			peer.log.Error("cannot create remote net address", zap.Error(err))
			peer.Disconnect()
			return
		}
		peer.na = na
	}

	go func() {
		peer.log.Debug("start peer", zap.String("remote_addr", peer.addr))
		if err := peer.start(); err != nil {
			log.Debugf("Cannot start peer %v: %v", peer, err)
			peer.log.Debug("start peer: disconnect", zap.String("remote_addr", peer.addr), zap.Error(err))
			peer.Disconnect()
		}
	}()
}

// WaitForDisconnect waits until the peer has completely disconnected and all
// resources are cleaned up.  This will happen if either the local or remote
// side has been disconnected or the peer is forcibly disconnected via
// Disconnect.
func (peer *Peer) WaitForDisconnect() {
	<-peer.quit
}

// newPeerBase returns a new base bitcoin peer based on the inbound flag.  This
// is used by the NewInboundPeer and NewOutboundPeer functions to perform base
// setup needed by both types of peers.
func newPeerBase(origCfg *Config, inbound bool, chainCtx chain.IChainCtx) *Peer {
	// Default to the max supported protocol version if not specified by the
	// caller.
	cfg := *origCfg // Copy to avoid mutating caller.
	if cfg.ProtocolVersion == 0 {
		cfg.ProtocolVersion = MaxProtocolVersion
	}

	// Set the chainCtx parameters to testnet if the caller did not specify any.
	if cfg.ChainParams == nil {
		cfg.ChainParams = &chaincfg.TestNet3Params
	}

	// Set the trickle interval if a non-positive value is specified.
	if cfg.TrickleInterval <= 0 {
		cfg.TrickleInterval = DefaultTrickleInterval
	}
	logger := log.(*corelog.LogAdapter)

	p := Peer{
		inbound:         inbound,
		chain:           chainCtx,
		wireEncoding:    wire.BaseEncoding,
		knownInventory:  newMruInventoryMap(maxKnownInventory),
		stallControl:    make(chan stallControlMsg, 1), // nonblocking sync
		outputQueue:     make(chan outMsg, outputBufferSize),
		sendQueue:       make(chan outMsg, 1),   // nonblocking sync
		sendDoneQueue:   make(chan struct{}, 1), // nonblocking sync
		outputInvChan:   make(chan *types.InvVect, outputBufferSize),
		inQuit:          make(chan struct{}),
		queueQuit:       make(chan struct{}),
		outQuit:         make(chan struct{}),
		quit:            make(chan struct{}),
		cfg:             cfg, // Copy so caller can't mutate.
		services:        cfg.Services,
		protocolVersion: cfg.ProtocolVersion,
		log: logger.Logger.With(
			zap.Bool("inbound", inbound),
			zap.Uint32("shardID", chainCtx.ShardID()),
		),
	}
	return &p
}

// NewInboundPeer returns a new inbound bitcoin peer. Use Start to begin
// processing incoming and outgoing messages.
func NewInboundPeer(cfg *Config, chainCtx chain.IChainCtx) *Peer {
	return newPeerBase(cfg, true, chainCtx)
}

// NewOutboundPeer returns a new outbound bitcoin peer.
func NewOutboundPeer(cfg *Config, addr string, chainCtx chain.IChainCtx) (*Peer, error) {
	p := newPeerBase(cfg, false, chainCtx)
	p.addr = addr

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}

	if cfg.HostToNetAddress != nil {
		na, err := cfg.HostToNetAddress(host, uint16(port), 0)
		if err != nil {
			return nil, err
		}
		p.na = na
	} else {
		p.na = wire.NewNetAddressIPPort(net.ParseIP(host), uint16(port), 0)
	}

	p.log = p.log.With(
		zap.String("remote_ip", p.NA().IP.String()),
		zap.Uint16("remote_port", p.NA().Port),
	)
	return p, nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
