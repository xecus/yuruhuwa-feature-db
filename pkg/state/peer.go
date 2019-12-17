package state

import (
	"log"
	"bytes"
	"encoding/gob"

	"github.com/abeja-inc/feature-search-db/pkg/brick"
	"github.com/weaveworks/mesh"
)

// Peer encapsulates State and implements mesh.Gossiper.
// It should be passed to mesh.Router.NewGossip,
// and the resulting Gossip registered in turn,
// before calling mesh.Router.Start.
type Peer struct {
	st      *State
	send    mesh.Gossip
	actions chan<- func()
	quit    chan struct{}
	logger  *log.Logger
}

type PeerConfig struct {
	ipAddress string
	featureApiHttpListen string
}

func NewPeerConfig(
	ipAddress string,
	featureApiHttpListen string,
	)PeerConfig{
	return PeerConfig{
		ipAddress:            ipAddress,
		featureApiHttpListen: featureApiHttpListen,
	}
}

// peer implements mesh.Gossiper.
var _ mesh.Gossiper = &Peer{}

// Construct a peer with empty State.
// Be sure to Register a channel, later,
// so we can make outbound communication.
func NewPeer(self mesh.PeerName, logger *log.Logger) *Peer {
	actions := make(chan func())
	p := &Peer{
		st:      newState(self),
		send:    nil, // must .Register() later
		actions: actions,
		quit:    make(chan struct{}),
		logger:  logger,
	}
	go p.loop(actions)
	return p
}

func (p *Peer) loop(actions <-chan func()) {
	for {
		select {
		case f := <-actions:
			f()
		case <-p.quit:
			return
		}
	}
}

// Register the result of a mesh.Router.NewGossip.
func (p *Peer) Register(send mesh.Gossip) {
	p.actions <- func() { p.send = send }
}

// Return the current value of the counter.
func (p *Peer) GetAllState() StateContent {
	return p.st.getAllState()
}

func (p *Peer) Del() (result StateContent) {
	c := make(chan struct{})
	p.actions <- func() {
		defer close(c)
		st := p.st.del()
		if p.send != nil {
			p.send.GossipBroadcast(st)
		} else {
			p.logger.Printf("no sender configured; not broadcasting update right now")
		}
		result = st.getAllState()
	}
	<-c
	return result
}

func (p *Peer) SetNodeInfo(peerConf PeerConfig, bp *brick.BrickPool) (result StateContent) {
	c := make(chan struct{})
	p.actions <- func() {
		defer close(c)
		st := p.st.setNodeInfo(peerConf, bp)
		if p.send != nil {
			p.send.GossipBroadcast(st)
		} else {
			p.logger.Printf("no sender configured; not broadcasting update right now")
		}
		result = st.getAllState()
	}
	<-c
	return result
}

func (p *Peer) stop() {
	close(p.quit)
}

// Return a copy of our complete State.
func (p *Peer) Gossip() (complete mesh.GossipData) {
	complete = p.st.copy()
	p.logger.Printf("Gossip => complete %v", complete.(*State).set)
	return complete
}

// Merge the gossiped data represented by buf into our State.
// Return the State information that was modified.
func (p *Peer) OnGossip(buf []byte) (delta mesh.GossipData, err error) {
	var set map[mesh.PeerName]StateContent
	if err := gob.NewDecoder(bytes.NewReader(buf)).Decode(&set); err != nil {
		return nil, err
	}

	delta = p.st.mergeDelta(set)
	if delta == nil {
		p.logger.Printf("OnGossip %v => delta %v", set, delta)
	} else {
		p.logger.Printf("OnGossip %v => delta %v", set, delta.(*State).set)
	}
	return delta, nil
}

// Merge the gossiped data represented by buf into our State.
// Return the State information that was modified.
func (p *Peer) OnGossipBroadcast(src mesh.PeerName, buf []byte) (received mesh.GossipData, err error) {
	var set map[mesh.PeerName]StateContent
	if err := gob.NewDecoder(bytes.NewReader(buf)).Decode(&set); err != nil {
		return nil, err
	}

	received = p.st.mergeReceived(set)
	if received == nil {
		p.logger.Printf("OnGossipBroadcast %s %v => delta %v", src, set, received)
	} else {
		p.logger.Printf("OnGossipBroadcast %s %v => delta %v", src, set, received.(*State).set)
	}
	return received, nil
}

// Merge the gossiped data represented by buf into our State.
func (p *Peer) OnGossipUnicast(src mesh.PeerName, buf []byte) error {
	var set map[mesh.PeerName]StateContent
	if err := gob.NewDecoder(bytes.NewReader(buf)).Decode(&set); err != nil {
		return err
	}

	complete := p.st.mergeComplete(set)
	p.logger.Printf("OnGossipUnicast %s %v => complete %v", src, set, complete)
	return nil
}
