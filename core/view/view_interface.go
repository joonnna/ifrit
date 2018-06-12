package view

type ViewService interface {
	Peer()
	FullView()
	LiveView()
	AddLive()
	RemoveLive()
	AddFull()
	Timeouts()
	GossipPartner()
	MonitorTarget()
}

/*
type ViewService interface {
	FullView()
	AddFull()

	LiveView()
	AddLive()
	RemoveLive()
}
*/

type Peer struct {
	IsLive()
	IsAccused()
	Equal()
	IncrementPing()
	Accusations()
	Accusation()

}



