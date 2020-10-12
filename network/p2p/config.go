package p2p

import (
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
	"go.uber.org/zap"
)

type Config struct {
	Peers          []string `yaml:"peers" `
	Listeners      []string `yaml:"listeners"`
	AgentBlacklist []string `yaml:"agent_blacklist"`
	AgentWhitelist []string `yaml:"agent_whitelist"`
	DisableListen  bool     `yaml:"disable_listen" long:"nolisten" description:"Disable listening for incoming connections -- NOTE: Listening is automatically disabled if the --connect or --proxy options are used without also specifying listen interfaces via --listen"`

	ExternalIPs    []string      `yaml:"external_ips" long:"externalip" description:"Add an ip to the list of local addresses we claim to listen on to peers"`
	ConnectPeers   []string      `yaml:"connect_peers" long:"connect" description:"Connect only to the specified peers at startup"`
	BanDuration    time.Duration `yaml:"ban_duration" long:"banduration" description:"How long to ban misbehaving peers.  Valid time units are {s, m, h}.  Minimum 1 second"`
	BanThreshold   uint32        `yaml:"ban_threshold" long:"banthreshold" description:"Maximum allowed ban score before disconnecting and banning misbehaving peers."`
	DisableBanning bool          `yaml:"disable_banning" long:"nobanning" description:"Disable banning of misbehaving peers"`
	BlocksOnly     bool          `yaml:"blocks_only" long:"blocksonly" description:"Do not accept transactions from remote peers."`

	DisableTLS      bool          `yaml:"disable_tls" long:"notls" description:"Disable TLS for the RPC Server -- NOTE: This is only allowed if the RPC Server is bound to localhost"`
	OnionProxy      string        `yaml:"onion_proxy" long:"onion" description:"Connect to tor hidden services via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	OnionProxyPass  string        `yaml:"onion_proxy_pass" long:"onionpass" default-mask:"-" description:"Password for onion proxy Server"`
	OnionProxyUser  string        `yaml:"onion_proxy_user" long:"onionuser" description:"Username for onion proxy Server"`
	Proxy           string        `yaml:"proxy" long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	ProxyPass       string        `yaml:"proxy_pass" long:"proxypass" default-mask:"-" description:"Password for proxy Server"`
	ProxyUser       string        `yaml:"proxy_user" long:"proxyuser" description:"Username for proxy Server"`
	RejectNonStd    bool          `yaml:"reject_non_std" long:"rejectnonstd" description:"Reject non-standard transactions regardless of the default settings for the active network."`
	TrickleInterval time.Duration `yaml:"trickle_interval" long:"trickleinterval" description:"Minimum time between attempts to send new inventory to a connected Server"`
	DisableDNSSeed  bool          `yaml:"disable_dns_seed" long:"nodnsseed" description:"Disable DNS seeding for peers"`
	NoOnion         bool          `yaml:"no_onion" long:"noonion" description:"Disable connecting to tor hidden services"`
	Upnp            bool          `yaml:"upnp" long:"upnp" description:"Use UPnP to map our listening port outside of NAT"`

	Oniondial func(string, string, time.Duration) (net.Conn, error)
	Dial      func(string, string, time.Duration) (net.Conn, error)
	Lookup    func(string) ([]net.IP, error)
}

type ListenOpts struct {
	DefaultPort string
	Listeners   []string
}

func (o *ListenOpts) Update(listeners []string) error {
	port, err := GetFreePort()
	if err != nil {
		return err
	}
	o.DefaultPort = strconv.Itoa(port)
	o.Listeners = SetPortForListeners(listeners, port)
	return nil
}

func SetPortForListeners(listeners []string, port int) []string {
	for i, listener := range listeners {
		host, _, _ := net.SplitHostPort(listener)
		listeners[i] = net.JoinHostPort(host, strconv.Itoa(port))
	}

	return listeners
}

func GetFreePortList(count int) (ports []int, err error) {
	for i := 0; i < count; i++ {
		port, err := GetFreePort()
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	return
}

func GetFreePort() (port int, err error) {
	ln, err := net.Listen("tcp", "[::]:0")
	if err != nil {
		return -1, err
	}
	port = ln.Addr().(*net.TCPAddr).Port
	err = ln.Close()
	return
}

// initListeners initializes the configured net listeners and adds any bound
// addresses to the address manager. Returns the listeners and a NAT interface,
// which is non-nil if UPnP is in use.
func initListeners(cfg *Config, defaultPort string, amgr *addrmgr.AddrManager, listenAddrs []string, services wire.ServiceFlag, logger *zap.Logger) ([]net.Listener, NAT, error) {
	// Listen for TCP connections at the configured addresses
	netAddrs, err := ParseListeners(listenAddrs)
	if err != nil {
		return nil, nil, err
	}

	listeners := make([]net.Listener, 0, len(netAddrs))
	for _, addr := range netAddrs {
		listener, err := net.Listen(addr.Network(), addr.String())
		if err != nil {
			logger.Warn(fmt.Sprintf("Can't listen on %s: %v", addr, err))
			continue
		}
		listeners = append(listeners, listener)
	}

	var nat NAT
	if len(cfg.ExternalIPs) != 0 {
		defaultPort, err := strconv.ParseUint(defaultPort, 10, 16)
		if err != nil {
			logger.Error(fmt.Sprintf("Can not parse default port %s for active BlockChain: %v",
				defaultPort, err))
			return nil, nil, err
		}

		for _, sip := range cfg.ExternalIPs {
			eport := uint16(defaultPort)
			host, portstr, err := net.SplitHostPort(sip)
			if err != nil {
				// no port, use default.
				host = sip
			} else {
				port, err := strconv.ParseUint(portstr, 10, 16)
				if err != nil {
					logger.Warn(fmt.Sprintf("Can not parse port from %s for "+
						"externalip: %v", sip, err))
					continue
				}
				eport = uint16(port)
			}
			na, err := amgr.HostToNetAddress(host, eport, services)
			if err != nil {
				logger.Warn(fmt.Sprintf("Not adding %s as externalip: %v", sip, err))
				continue
			}

			err = amgr.AddLocalAddress(na, addrmgr.ManualPrio)
			if err != nil {
				logger.Warn(fmt.Sprintf("Skipping specified external IP: %v", err))
			}
		}
	} else {
		if cfg.Upnp {
			var err error
			nat, err = Discover()
			if err != nil {
				logger.Warn(fmt.Sprintf("Can't discover upnp: %v", err))
			}
			// nil nat here is fine, just means no upnp on network.
		}

		// Add bound addresses to address manager to be advertised to peers.
		for _, listener := range listeners {
			addr := listener.Addr().String()
			err := addLocalAddress(amgr, addr, services)
			if err != nil {
				logger.Warn(fmt.Sprintf("Skipping bound address %s: %v", addr, err))
			}
		}
	}

	return listeners, nat, nil
}

// addLocalAddress adds an address that this chainProvider is listening on to the
// address manager so that it may be relayed to peers.
func addLocalAddress(addrMgr *addrmgr.AddrManager, addr string, services wire.ServiceFlag) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return err
	}

	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		// If bound to unspecified address, advertise all local interfaces
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			ifaceIP, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}

			// If bound to 0.0.0.0, do not add IPv6 interfaces and if bound to
			// ::, do not add IPv4 interfaces.
			if (ip.To4() == nil) != (ifaceIP.To4() == nil) {
				continue
			}

			netAddr := wire.NewNetAddressIPPort(ifaceIP, uint16(port), services)
			addrMgr.AddLocalAddress(netAddr, addrmgr.BoundPrio)
		}
	} else {
		netAddr, err := addrMgr.HostToNetAddress(host, uint16(port), services)
		if err != nil {
			return err
		}

		addrMgr.AddLocalAddress(netAddr, addrmgr.BoundPrio)
	}

	return nil
}

// ParseListeners determines whether each listen address is IPv4 and IPv6 and
// returns a slice of appropriate net.Addrs to listen on with TCP. It also
// properly detects addresses which apply to "all interfaces" and adds the
// address as both IPv4 and IPv6.
func ParseListeners(addrs []string) ([]net.Addr, error) {
	netAddrs := make([]net.Addr, 0, len(addrs)*2)
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// Shouldn't happen due to already being normalized.
			return nil, err
		}

		// Empty host or host of * on plan9 is both IPv4 and IPv6.
		if host == "" || (host == "*" && runtime.GOOS == "plan9") {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp4", addr: addr})
			netAddrs = append(netAddrs, simpleAddr{net: "tcp6", addr: addr})
			continue
		}

		// Strip IPv6 zone id if present since net.ParseIP does not
		// handle it.
		zoneIndex := strings.LastIndex(host, "%")
		if zoneIndex > 0 {
			host = host[:zoneIndex]
		}

		// Parse the IP.
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("'%s' is not a valid IP address", host)
		}

		// To4 returns nil when the IP is not an IPv4 address, so use
		// this determine the address type.
		if ip.To4() == nil {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp6", addr: addr})
		} else {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp4", addr: addr})
		}
	}
	return netAddrs, nil
}
