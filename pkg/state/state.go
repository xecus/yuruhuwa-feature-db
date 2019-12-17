package state

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/abeja-inc/feature-search-db/pkg/brick"

	"github.com/weaveworks/mesh"
)

type BrickInfo struct {
	UniqueID             string `json:"uniqueID"`
	BrickID              string `json:"brickID"`
	FeatureGroupID       int    `json:"groupID"`
	NumOfBrickTotalCap   int    `json:"numOfBrickTotalCap"`
	NumOfAvailablePoints int    `json:"numOfAvailablePoints"`
}

type NodeInfo struct {
	Bricks        *[]BrickInfo `json:"bricks"`
	Count         int          `json:"count"`
	IpAddress     string       `json:"ipAddress"`
	ApiPort       string       `json:"api_port"`
	LaunchAt      time.Time    `json:"launch_at"`
	LastUpdatedAt time.Time    `json:"last_updated_at"`
}

func (ni *NodeInfo) GetLastUpdatedAt() int64 {
	return ni.LastUpdatedAt.Unix()
}

type StateContent struct {
	NodeInfos map[string]NodeInfo
}

// State is an implementation of a G-counter.
type State struct {
	mtx  sync.RWMutex
	set  map[mesh.PeerName]StateContent
	self mesh.PeerName
}

// State implements GossipData.
var _ mesh.GossipData = &State{}

// Construct an empty State object, ready to receive updates.
// This is suitable to use at program start.
// Other peers will populate us with data.
func newState(self mesh.PeerName) *State {
	return &State{
		set:  map[mesh.PeerName]StateContent{},
		self: self,
	}
}

func (st *State) getAllState() (result StateContent) {
	st.mtx.RLock()
	defer st.mtx.RUnlock()
	// NodeInfos
	result.NodeInfos = map[string]NodeInfo{}
	//log.Println(st.set)
	for _, v := range st.set {
		for nodeInfoKey, nodeInfoVal := range v.NodeInfos {
			resultNodeInfo := result.NodeInfos[nodeInfoKey]
			// Deleted
			if nodeInfoVal == (NodeInfo{}) {
				continue
			}
			if nodeInfoVal.GetLastUpdatedAt() > resultNodeInfo.GetLastUpdatedAt() {
				result.NodeInfos[nodeInfoKey] = nodeInfoVal
			}
		}
	}
	return result
}

func (st *State) del() (complete *State) {
	st.mtx.Lock()
	defer st.mtx.Unlock()
	if _, ok := st.set[st.self]; ok {
		st.set[st.self] = StateContent{
			NodeInfos: map[string]NodeInfo{
				st.self.String(): NodeInfo{},
			},
		}
	}
	return &State{
		set: st.set,
	}
}

func (st *State) setNodeInfo(peerConf PeerConfig, bp *brick.BrickPool) (complete *State) {
	st.mtx.Lock()
	defer st.mtx.Unlock()

	bricks, _ := bp.GetAllBricks()
	brickInfos := make([]BrickInfo, 0, len(bricks))
	for _, b := range bricks {
		brickInfos = append(brickInfos, BrickInfo{
			UniqueID:             b.GetUniqueIDstr(),
			BrickID:              b.GetBrickIDstr(),
			FeatureGroupID:       b.GetFeatureGroupIDint(),
			NumOfBrickTotalCap:   b.NumOfBrickTotalCap,
			NumOfAvailablePoints: b.NumOfAvailablePoints,
			//NodeName:             st.self.String(),
		})
	}

	if _, ok := st.set[st.self]; ok {
		// NodeInfos
		c := st.set[st.self].NodeInfos[st.self.String()]
		st.set[st.self] = StateContent{
			NodeInfos: map[string]NodeInfo{
				st.self.String(): NodeInfo{
					Bricks:        &brickInfos,
					Count:         c.Count + 1,
					IpAddress:     fmt.Sprintf("%s", peerConf.ipAddress),
					ApiPort:       fmt.Sprintf("%s", peerConf.featureApiHttpListen),
					LaunchAt:      c.LaunchAt,
					LastUpdatedAt: time.Now(),
					//NodeName:      st.self.String(),
				},
			},
		}
	} else {
		// NodeInfos
		st.set[st.self] = StateContent{
			NodeInfos: map[string]NodeInfo{
				st.self.String(): NodeInfo{
					Bricks:        &brickInfos,
					Count:         0,
					IpAddress:     fmt.Sprintf("%s", peerConf.ipAddress),
					ApiPort:       fmt.Sprintf("%s", peerConf.featureApiHttpListen),
					LaunchAt:      time.Now(),
					LastUpdatedAt: time.Now(),
					//NodeName:      st.self.String(),
				},
			},
		}
	}
	return &State{
		set: st.set,
	}
}

func (st *State) copy() *State {
	st.mtx.RLock()
	defer st.mtx.RUnlock()
	return &State{
		set: st.set,
	}
}

// Encode serializes our complete State to a slice of byte-slices.
// In this simple example, we use a single gob-encoded
// buffer: see https://golang.org/pkg/encoding/gob/
func (st *State) Encode() [][]byte {
	st.mtx.RLock()
	defer st.mtx.RUnlock()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(st.set); err != nil {
		panic(err)
	}
	return [][]byte{buf.Bytes()}
}

func (st *State) Merge(other mesh.GossipData) (complete mesh.GossipData) {
	return st.mergeComplete(other.(*State).copy().set)
}

func (st *State) mergeReceived(set map[mesh.PeerName]StateContent) (received mesh.GossipData) {
	st.mtx.Lock()
	defer st.mtx.Unlock()
	log.Println("mergeReceived")

	for peer, v := range set {
		if _, ok := st.set[peer]; !ok {
			st.set[peer] = StateContent{
				NodeInfos: map[string]NodeInfo{},
			}
		}
		obj := st.set[peer]
		for nodeInfoKey, nodeInfoVal := range v.NodeInfos {
			objNodeInfo := obj.NodeInfos[nodeInfoKey]
			if nodeInfoVal == (NodeInfo{}) {
				delete(st.set[peer].NodeInfos, nodeInfoKey)
				continue
			}
			if nodeInfoVal.GetLastUpdatedAt() <= objNodeInfo.GetLastUpdatedAt() {
				delete(set[peer].NodeInfos, nodeInfoKey)
				continue
			}
			st.set[peer].NodeInfos[nodeInfoKey] = nodeInfoVal
		}
	}
	return &State{
		set: set, // all remaining elements were novel to us
	}
}

func (st *State) mergeDelta(set map[mesh.PeerName]StateContent) (delta mesh.GossipData) {
	st.mtx.Lock()
	defer st.mtx.Unlock()
	log.Println("mergeDelta")

	for peer, v := range set {
		if _, ok := st.set[peer]; !ok {
			st.set[peer] = StateContent{
				NodeInfos: map[string]NodeInfo{},
			}
		}
		obj := st.set[peer]
		for nodeInfoKey, nodeInfoVal := range v.NodeInfos {
			objNodeInfo := obj.NodeInfos[nodeInfoKey]
			if nodeInfoVal == (NodeInfo{}) {
				delete(st.set[peer].NodeInfos, nodeInfoKey)
				continue
			}
			if nodeInfoVal.GetLastUpdatedAt() <= objNodeInfo.GetLastUpdatedAt() {
				delete(set[peer].NodeInfos, nodeInfoKey)
				continue
			}
			st.set[peer].NodeInfos[nodeInfoKey] = nodeInfoVal
		}
	}

	if len(set) <= 0 {
		return nil // per OnGossip requirements
	}
	return &State{
		set: set, // all remaining elements were novel to us
	}
}

func (st *State) mergeComplete(set map[mesh.PeerName]StateContent) (complete mesh.GossipData) {
	st.mtx.Lock()
	defer st.mtx.Unlock()
	log.Println("mergeComplete")

	for peer, v := range set {
		log.Println(peer)
		obj := st.set[peer]
		// Sync NodeInfos
		for nodeInfoKey, nodeInfoVal := range v.NodeInfos {
			objNodeInfo := obj.NodeInfos[nodeInfoKey]
			if nodeInfoVal == (NodeInfo{}) {
				delete(st.set[peer].NodeInfos, nodeInfoKey)
				continue
			}
			if nodeInfoVal.GetLastUpdatedAt() > objNodeInfo.GetLastUpdatedAt() {
				st.set[peer].NodeInfos[nodeInfoKey] = nodeInfoVal
			}
		}
	}

	return &State{
		set: st.set, // n.b. can't .copy() due to lock contention
	}
}
