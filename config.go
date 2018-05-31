package ifrit

import (
	"errors"

	"github.com/joonnna/ifrit/core"
)

var (
	errNoConf   = errors.New("Got nil config")
	errNoCaAddr = errors.New("Ca was set to true, but no address specified")
	errNoAddrs  = errors.New("Ca was set to false, but no entryaddrs set")
)

const (
	defGossipRate         = 10
	defMonitorRate        = 10
	defMaxFailPings       = 3
	defViewRemovalTimeout = 60
	defVisUpdateTimeout   = 5
	defMaxConc            = 100
)

// Config for ifrit client
// All values that are not set are set to their default values.
// Passing a nil config results in all defaults.
type Config struct {
	// How often ifrit should gossip with neighbours (in seconds), defaults to 10 seconds
	GossipRate uint32

	// How often ifrit should ping neighbours (in seconds), defaults to 10 seconds
	MonitorRate uint32

	// How many failed pings before ifrit assumes other nodes to be dead, defaults to 3
	MaxFailPings uint32

	// How long ifrit waits before removing crashed node from view (in seconds), defaults to 60 seconds
	ViewRemovalTimeout uint32

	// If ifrit should contact the visualizer upon gaining new neighbours
	Visualizer bool

	// Address where the visualizer can be contacted (ip:port)
	VisAddr string

	VisAppAddr string

	// How often ifrit should check if it has gained any new neighbours
	VisUpdateTimeout uint32

	// Max concurrent messages, used for rate limiting messaging service.
	// E.g. Don't want to spawn 5000 goroutines if SendToAll is called with 5000 live memebers.
	// When max is reached remaining sends are queued, defaults to 100.
	MaxConcurrentMsgs uint32

	// Decides whether to contact a certificate authority to get a signed certificate.
	// If set to false, the client will use a self signed certificate for TLS communication.
	// If set to true, CaAddr must be populated with the address(ip:port) of the ca.
	Ca     bool
	CaAddr string

	// If Ca is set to false this field needs to be populated with addresses of existing
	// ifrit clients.
	// Each address should be ip:port of other ifrit clients.
	EntryAddrs []string
}

func parseConfig(conf *Config) (*core.Config, error) {
	nodeConf := &core.Config{}

	if conf == nil {
		nodeConf.GossipRate = defGossipRate
		nodeConf.MonitorRate = defMonitorRate
		nodeConf.MaxFailPings = defMaxFailPings
		nodeConf.ViewRemovalTimeout = defViewRemovalTimeout
		nodeConf.MaxConc = defMaxConc
		return nodeConf, nil
	}

	if conf.Ca {
		if conf.CaAddr == "" {
			return nil, errNoCaAddr
		}
	}

	nodeConf.VisAppAddr = conf.VisAppAddr
	nodeConf.EntryAddrs = conf.EntryAddrs
	nodeConf.Ca = conf.Ca
	nodeConf.CaAddr = conf.CaAddr

	if conf.GossipRate == 0 {
		nodeConf.GossipRate = defGossipRate
	} else {
		nodeConf.GossipRate = conf.GossipRate
	}

	if conf.MonitorRate == 0 {
		nodeConf.MonitorRate = defMonitorRate
	} else {
		nodeConf.MonitorRate = conf.MonitorRate
	}

	if conf.MaxFailPings == 0 {
		nodeConf.MaxFailPings = defMaxFailPings
	} else {
		nodeConf.MaxFailPings = conf.MaxFailPings
	}

	if conf.ViewRemovalTimeout == 0 {
		nodeConf.ViewRemovalTimeout = defViewRemovalTimeout
	} else {
		nodeConf.ViewRemovalTimeout = conf.ViewRemovalTimeout
	}

	if conf.MaxConcurrentMsgs == 0 {
		nodeConf.MaxConc = defMaxConc
	} else {
		nodeConf.MaxConc = conf.MaxConcurrentMsgs
	}

	nodeConf.Visualizer = conf.Visualizer
	nodeConf.VisAddr = conf.VisAddr

	if conf.VisUpdateTimeout == 0 {
		nodeConf.VisUpdateTimeout = defVisUpdateTimeout
	} else {
		nodeConf.VisUpdateTimeout = conf.VisUpdateTimeout
	}

	return nodeConf, nil
}
