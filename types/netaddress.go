// Copyright (c) 2013-2015 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package types

import (
	"net"
	"time"
)

// MaxNetAddressPayload returns the max payload size for a bitcoin NetAddress
// based on the protocol version.
func MaxNetAddressPayload(pver uint32) uint32 {
	// Services 8 bytes + ip 16 bytes + port 2 bytes.
	plen := uint32(26)

	// NetAddressTimeVersion added a timestamp field.
	if pver >= NetAddressTimeVersion {
		// Timestamp 4 bytes.
		plen += 4
	}

	return plen
}

// NetAddress defines information about a server on the network including the time
// it was last seen, the services it supports, its IP address, and port.
type NetAddress struct {
	// Last time the address was seen.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.  This field is
	// not present in the bitcoin version message (MsgVersion) nor was it
	// added until protocol version >= NetAddressTimeVersion.
	Timestamp time.Time

	// Bitfield which identifies the services supported by the address.
	Services ServiceFlag

	// IP address of the server.
	IP net.IP

	// Port the server is using.  This is encoded in big endian on the wire
	// which differs from most everything else.
	Port uint16
}

// HasService returns whether the specified service is supported by the address.
func (na *NetAddress) HasService(service ServiceFlag) bool {
	return na.Services&service == service
}

// AddService adds service as a supported service by the server generating the
// message.
func (na *NetAddress) AddService(service ServiceFlag) {
	na.Services |= service
}

// NewNetAddressIPPort returns a new NetAddress using the provided IP, port, and
// supported services with defaults for the remaining fields.
func NewNetAddressIPPort(ip net.IP, port uint16, services ServiceFlag) *NetAddress {
	return NewNetAddressTimestamp(time.Now(), services, ip, port)
}

// NewNetAddressTimestamp returns a new NetAddress using the provided
// timestamp, IP, port, and supported services. The timestamp is rounded to
// single second precision.
func NewNetAddressTimestamp(
	timestamp time.Time, services ServiceFlag, ip net.IP, port uint16) *NetAddress {
	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	na := NetAddress{
		Timestamp: time.Unix(timestamp.Unix(), 0),
		Services:  services,
		IP:        ip,
		Port:      port,
	}
	return &na
}

// NewNetAddress returns a new NetAddress using the provided TCP address and
// supported services with defaults for the remaining fields.
func NewNetAddress(addr *net.TCPAddr, services ServiceFlag) *NetAddress {
	return NewNetAddressIPPort(addr.IP, uint16(addr.Port), services)
}
