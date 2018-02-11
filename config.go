package ifrit

import "github.com/joonnna/ifrit/core"

const (
	defGossipRate         = 10
	defMonitorRate        = 10
	defMaxFailPings       = 3
	defViewRemovalTimeout = 60
	defVisUpdateTimeout   = 5
	defMaxConc            = 100
)

//Config for ifrit client
//All values that are not set are set to their default values (passing an empty struct results to all default values).
type Config struct {
	//How often ifrit should gossip with neighbours (in seconds), defaults to 10 seconds
	GossipRate uint32

	//How often ifrit should ping neighbours (in seconds), defaults to 10 seconds
	MonitorRate uint32

	//How many failed pings before ifrit assumes other nodes to be dead, defaults to 3
	MaxFailPings uint32

	//How long ifrit waits before removing crashed node from view (in seconds), defaults to 60 seconds
	ViewRemovalTimeout uint32

	//If ifrit should contact the visualizer upon gaining new neighbours
	Visualizer bool

	//Address where the visualizer can be contacted (ip:port)
	VisAddr string

	//How often ifrit should check if it has gained any new neighbours
	VisUpdateTimeout uint32

	//Max concurrent messages, used for rate limiting messaging service.
	//E.g. Don't want to spawn 5000 goroutines if SendToAll is called with 5000 live memebers.
	//When max is reached remaining sends are queued, defaults to 100.
	MaxConcurrentMsgs uint32
}

func parseConfig(conf *Config, entryAddr string) *core.Config {
	nodeConf := &core.Config{}

	if conf == nil {
		nodeConf.GossipRate = defGossipRate
		nodeConf.MonitorRate = defMonitorRate
		nodeConf.MaxFailPings = defMaxFailPings
		nodeConf.ViewRemovalTimeout = defViewRemovalTimeout
		nodeConf.VisUpdateTimeout = defVisUpdateTimeout
		nodeConf.MaxConc = defMaxConc
	}

	nodeConf.EntryAddr = entryAddr

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

	return nodeConf
}
