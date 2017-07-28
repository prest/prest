package gocql

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type nodeState int32

func (n nodeState) String() string {
	if n == NodeUp {
		return "UP"
	} else if n == NodeDown {
		return "DOWN"
	}
	return fmt.Sprintf("UNKNOWN_%d", n)
}

const (
	NodeUp nodeState = iota
	NodeDown
)

type cassVersion struct {
	Major, Minor, Patch int
}

func (c *cassVersion) Set(v string) error {
	if v == "" {
		return nil
	}

	return c.UnmarshalCQL(nil, []byte(v))
}

func (c *cassVersion) UnmarshalCQL(info TypeInfo, data []byte) error {
	return c.unmarshal(data)
}

func (c *cassVersion) unmarshal(data []byte) error {
	version := strings.TrimSuffix(string(data), "-SNAPSHOT")
	version = strings.TrimPrefix(version, "v")
	v := strings.Split(version, ".")

	if len(v) < 2 {
		return fmt.Errorf("invalid version string: %s", data)
	}

	var err error
	c.Major, err = strconv.Atoi(v[0])
	if err != nil {
		return fmt.Errorf("invalid major version %v: %v", v[0], err)
	}

	c.Minor, err = strconv.Atoi(v[1])
	if err != nil {
		return fmt.Errorf("invalid minor version %v: %v", v[1], err)
	}

	if len(v) > 2 {
		c.Patch, err = strconv.Atoi(v[2])
		if err != nil {
			return fmt.Errorf("invalid patch version %v: %v", v[2], err)
		}
	}

	return nil
}

func (c cassVersion) Before(major, minor, patch int) bool {
	if c.Major > major {
		return true
	} else if c.Minor > minor {
		return true
	} else if c.Patch > patch {
		return true
	}
	return false
}

func (c cassVersion) String() string {
	return fmt.Sprintf("v%d.%d.%d", c.Major, c.Minor, c.Patch)
}

func (c cassVersion) nodeUpDelay() time.Duration {
	if c.Major >= 2 && c.Minor >= 2 {
		// CASSANDRA-8236
		return 0
	}

	return 10 * time.Second
}

type HostInfo struct {
	// TODO(zariel): reduce locking maybe, not all values will change, but to ensure
	// that we are thread safe use a mutex to access all fields.
	mu               sync.RWMutex
	peer             net.IP
	broadcastAddress net.IP
	listenAddress    net.IP
	rpcAddress       net.IP
	preferredIP      net.IP
	connectAddress   net.IP
	port             int
	dataCenter       string
	rack             string
	hostId           string
	workload         string
	graph            bool
	dseVersion       string
	partitioner      string
	clusterName      string
	version          cassVersion
	state            nodeState
	tokens           []string
}

func (h *HostInfo) Equal(host *HostInfo) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	host.mu.RLock()
	defer host.mu.RUnlock()

	return h.ConnectAddress().Equal(host.ConnectAddress())
}

func (h *HostInfo) Peer() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.peer
}

func (h *HostInfo) setPeer(peer net.IP) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peer = peer
	return h
}

func (h *HostInfo) invalidConnectAddr() bool {
	addr := h.ConnectAddress()
	return addr == nil || addr.IsUnspecified()
}

// Returns the address that should be used to connect to the host.
// If you wish to override this, use an AddressTranslator or
// use a HostFilter to SetConnectAddress()
func (h *HostInfo) ConnectAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.connectAddress == nil {
		// Use 'rpc_address' if provided and it's not 0.0.0.0
		if h.rpcAddress != nil && !h.rpcAddress.IsUnspecified() {
			return h.rpcAddress
		} else if h.broadcastAddress != nil && !h.broadcastAddress.IsUnspecified() {
			return h.broadcastAddress
		} else if h.peer != nil {
			// Peer should always be set if this from 'system.peer'
			return h.peer
		}
	}
	return h.connectAddress
}

func (h *HostInfo) SetConnectAddress(address net.IP) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connectAddress = address
	return h
}

func (h *HostInfo) BroadcastAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.broadcastAddress
}

func (h *HostInfo) ListenAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.listenAddress
}

func (h *HostInfo) RPCAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rpcAddress
}

func (h *HostInfo) PreferredIP() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.preferredIP
}

func (h *HostInfo) DataCenter() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.dataCenter
}

func (h *HostInfo) setDataCenter(dataCenter string) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dataCenter = dataCenter
	return h
}

func (h *HostInfo) Rack() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rack
}

func (h *HostInfo) setRack(rack string) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rack = rack
	return h
}

func (h *HostInfo) HostID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.hostId
}

func (h *HostInfo) setHostID(hostID string) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hostId = hostID
	return h
}

func (h *HostInfo) WorkLoad() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.workload
}

func (h *HostInfo) Graph() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.graph
}

func (h *HostInfo) DSEVersion() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.dseVersion
}

func (h *HostInfo) Partitioner() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.partitioner
}

func (h *HostInfo) ClusterName() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clusterName
}

func (h *HostInfo) Version() cassVersion {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.version
}

func (h *HostInfo) setVersion(major, minor, patch int) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.version = cassVersion{major, minor, patch}
	return h
}

func (h *HostInfo) State() nodeState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

func (h *HostInfo) setState(state nodeState) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = state
	return h
}

func (h *HostInfo) Tokens() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.tokens
}

func (h *HostInfo) setTokens(tokens []string) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tokens = tokens
	return h
}

func (h *HostInfo) Port() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.port
}

func (h *HostInfo) setPort(port int) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.port = port
	return h
}

func (h *HostInfo) update(from *HostInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.tokens = from.tokens
	h.version = from.version
	h.hostId = from.hostId
	h.dataCenter = from.dataCenter
}

func (h *HostInfo) IsUp() bool {
	return h != nil && h.State() == NodeUp
}

func (h *HostInfo) String() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return fmt.Sprintf("[HostInfo connectAddress=%q peer=%q rpc_address=%q broadcast_address=%q "+
		"port=%d data_centre=%q rack=%q host_id=%q version=%q state=%s num_tokens=%d]",
		h.connectAddress, h.peer, h.rpcAddress, h.broadcastAddress,
		h.port, h.dataCenter, h.rack, h.hostId, h.version, h.state, len(h.tokens))
}

// Polls system.peers at a specific interval to find new hosts
type ringDescriber struct {
	session         *Session
	mu              sync.Mutex
	prevHosts       []*HostInfo
	localHost       *HostInfo
	prevPartitioner string
}

// Returns true if we are using system_schema.keyspaces instead of system.schema_keyspaces
func checkSystemSchema(control *controlConn) (bool, error) {
	iter := control.query("SELECT * FROM system_schema.keyspaces")
	if err := iter.err; err != nil {
		if errf, ok := err.(*errorFrame); ok {
			if errf.code == errSyntax {
				return false, nil
			}
		}

		return false, err
	}

	return true, nil
}

// Given a map that represents a row from either system.local or system.peers
// return as much information as we can in *HostInfo
func (r *ringDescriber) hostInfoFromMap(row map[string]interface{}) (*HostInfo, error) {
	const assertErrorMsg = "Assertion failed for %s"
	var ok bool

	// Default to our connected port if the cluster doesn't have port information
	host := HostInfo{
		port: r.session.cfg.Port,
	}

	for key, value := range row {
		switch key {
		case "data_center":
			host.dataCenter, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "data_center")
			}
		case "rack":
			host.rack, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "rack")
			}
		case "host_id":
			hostId, ok := value.(UUID)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "host_id")
			}
			host.hostId = hostId.String()
		case "release_version":
			version, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "release_version")
			}
			host.version.Set(version)
		case "peer":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "peer")
			}
			host.peer = net.ParseIP(ip)
		case "cluster_name":
			host.clusterName, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "cluster_name")
			}
		case "partitioner":
			host.partitioner, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "partitioner")
			}
		case "broadcast_address":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "broadcast_address")
			}
			host.broadcastAddress = net.ParseIP(ip)
		case "preferred_ip":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "preferred_ip")
			}
			host.preferredIP = net.ParseIP(ip)
		case "rpc_address":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "rpc_address")
			}
			host.rpcAddress = net.ParseIP(ip)
		case "listen_address":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "listen_address")
			}
			host.listenAddress = net.ParseIP(ip)
		case "workload":
			host.workload, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "workload")
			}
		case "graph":
			host.graph, ok = value.(bool)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "graph")
			}
		case "tokens":
			host.tokens, ok = value.([]string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "tokens")
			}
		case "dse_version":
			host.dseVersion, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "dse_version")
			}
		}
		// TODO(thrawn01): Add 'port'? once CASSANDRA-7544 is complete
		// Not sure what the port field will be called until the JIRA issue is complete
	}

	return &host, nil
}

// Ask the control node for it's local host information
func (r *ringDescriber) GetLocalHostInfo() (*HostInfo, error) {
	it := r.session.control.query("SELECT * FROM system.local WHERE key='local'")
	if it == nil {
		return nil, errors.New("Attempted to query 'system.local' on a closed control connection")
	}
	host, err := r.extractHostInfo(it)
	if err != nil {
		return nil, err
	}

	if host.invalidConnectAddr() {
		host.SetConnectAddress(r.session.control.GetHostInfo().ConnectAddress())
	}

	return host, nil
}

// Given an ip address and port, return a peer that matched the ip address
func (r *ringDescriber) GetPeerHostInfo(ip net.IP, port int) (*HostInfo, error) {
	it := r.session.control.query("SELECT * FROM system.peers WHERE peer=?", ip)
	if it == nil {
		return nil, errors.New("Attempted to query 'system.peers' on a closed control connection")
	}
	return r.extractHostInfo(it)
}

func (r *ringDescriber) extractHostInfo(it *Iter) (*HostInfo, error) {
	row := make(map[string]interface{})

	// expect only 1 row
	it.MapScan(row)
	if err := it.Close(); err != nil {
		return nil, err
	}

	// extract all available info about the host
	return r.hostInfoFromMap(row)
}

// Ask the control node for host info on all it's known peers
func (r *ringDescriber) GetClusterPeerInfo() ([]*HostInfo, error) {
	var hosts []*HostInfo

	// Ask the node for a list of it's peers
	it := r.session.control.query("SELECT * FROM system.peers")
	if it == nil {
		return nil, errors.New("Attempted to query 'system.peers' on a closed connection")
	}

	for {
		row := make(map[string]interface{})
		if !it.MapScan(row) {
			break
		}
		// extract all available info about the peer
		host, err := r.hostInfoFromMap(row)
		if err != nil {
			return nil, err
		}

		// If it's not a valid peer
		if !r.IsValidPeer(host) {
			Logger.Printf("Found invalid peer '%+v' "+
				"Likely due to a gossip or snitch issue, this host will be ignored", host)
			continue
		}
		hosts = append(hosts, host)
	}
	if it.err != nil {
		return nil, fmt.Errorf("while scanning 'system.peers' table: %s", it.err)
	}
	return hosts, nil
}

// Return true if the host is a valid peer
func (r *ringDescriber) IsValidPeer(host *HostInfo) bool {
	return !(len(host.RPCAddress()) == 0 ||
		host.hostId == "" ||
		host.dataCenter == "" ||
		host.rack == "" ||
		len(host.tokens) == 0)
}

// Return a list of hosts the cluster knows about
func (r *ringDescriber) GetHosts() ([]*HostInfo, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update the localHost info with data from the connected host
	localHost, err := r.GetLocalHostInfo()
	if err != nil {
		return r.prevHosts, r.prevPartitioner, err
	} else if localHost.invalidConnectAddr() {
		panic(fmt.Sprintf("unable to get localhost connect address: %v", localHost))
	}

	// Update our list of hosts by querying the cluster
	hosts, err := r.GetClusterPeerInfo()
	if err != nil {
		return r.prevHosts, r.prevPartitioner, err
	}

	hosts = append(hosts, localHost)

	// Filter the hosts if filter is provided
	filteredHosts := hosts
	if r.session.cfg.HostFilter != nil {
		filteredHosts = filteredHosts[:0]
		for _, host := range hosts {
			if r.session.cfg.HostFilter.Accept(host) {
				filteredHosts = append(filteredHosts, host)
			}
		}
	}

	r.prevHosts = filteredHosts
	r.prevPartitioner = localHost.partitioner
	r.localHost = localHost

	return filteredHosts, localHost.partitioner, nil
}

// Given an ip/port return HostInfo for the specified ip/port
func (r *ringDescriber) GetHostInfo(ip net.IP, port int) (*HostInfo, error) {
	// TODO(thrawn01): Is IgnorePeerAddr still useful now that we have DisableInitialHostLookup?
	// TODO(thrawn01): should we also check for DisableInitialHostLookup and return if true?

	// Ignore the port and connect address and use the address/port we already have
	if r.session.control == nil || r.session.cfg.IgnorePeerAddr {
		return &HostInfo{connectAddress: ip, port: port}, nil
	}

	// Attempt to get the host info for our control connection
	controlHost := r.session.control.GetHostInfo()
	if controlHost == nil {
		return nil, errors.New("invalid control connection")
	}

	var (
		host *HostInfo
		err  error
	)

	// If we are asking about the same node our control connection has a connection too
	if controlHost.ConnectAddress().Equal(ip) {
		host, err = r.GetLocalHostInfo()
	} else {
		host, err = r.GetPeerHostInfo(ip, port)
	}

	// No host was found matching this ip/port
	if err != nil {
		return nil, err
	}

	if controlHost.ConnectAddress().Equal(ip) {
		// Always respect the provided control node address and disregard the ip address
		// the cassandra node provides. We do this as we are already connected and have a
		// known valid ip address. This insulates gocql from client connection issues stemming
		// from node misconfiguration. For instance when a node is run from a container, by
		// default the node will report its ip address as 127.0.0.1 which is typically invalid.
		host.SetConnectAddress(ip)
	}

	if host.invalidConnectAddr() {
		return nil, fmt.Errorf("host ConnectAddress invalid: %v", host)
	}

	return host, nil
}

func (r *ringDescriber) refreshRing() error {
	// if we have 0 hosts this will return the previous list of hosts to
	// attempt to reconnect to the cluster otherwise we would never find
	// downed hosts again, could possibly have an optimisation to only
	// try to add new hosts if GetHosts didnt error and the hosts didnt change.
	hosts, partitioner, err := r.GetHosts()
	if err != nil {
		return err
	}

	// TODO: move this to session
	// TODO: handle removing hosts here
	for _, h := range hosts {
		if host, ok := r.session.ring.addHostIfMissing(h); !ok {
			r.session.pool.addHost(h)
			r.session.policy.AddHost(h)
		} else {
			host.update(h)
		}
	}

	r.session.metadata.setPartitioner(partitioner)
	r.session.policy.SetPartitioner(partitioner)
	return nil
}
