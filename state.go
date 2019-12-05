package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/weaveworks/mesh"
	"log"
	"sync"
	"time"
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

// state is an implementation of a G-counter.
type state struct {
	mtx  sync.RWMutex
	set  map[mesh.PeerName]StateContent
	self mesh.PeerName
}

// state implements GossipData.
var _ mesh.GossipData = &state{}

// Construct an empty state object, ready to receive updates.
// This is suitable to use at program start.
// Other peers will populate us with data.
func newState(self mesh.PeerName) *state {
	return &state{
		set:  map[mesh.PeerName]StateContent{},
		self: self,
	}
}

func (st *state) getAllState() (result StateContent) {
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

func (st *state) del() (complete *state) {
	st.mtx.Lock()
	defer st.mtx.Unlock()
	if _, ok := st.set[st.self]; ok {
		st.set[st.self] = StateContent{
			NodeInfos: map[string]NodeInfo{
				st.self.String(): NodeInfo{},
			},
		}
	}
	return &state{
		set: st.set,
	}
}

func (st *state) setNodeInfo(cci *ClusterConfigInfo, bp *BrickPool) (complete *state) {
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
					IpAddress:     fmt.Sprintf("%s", *cci.ipAddress),
					ApiPort:       fmt.Sprintf("%s", *cci.featureApiHttpListen),
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
					IpAddress:     fmt.Sprintf("%s", *cci.ipAddress),
					ApiPort:       fmt.Sprintf("%s", *cci.featureApiHttpListen),
					LaunchAt:      time.Now(),
					LastUpdatedAt: time.Now(),
					//NodeName:      st.self.String(),
				},
			},
		}
	}
	return &state{
		set: st.set,
	}
}

func (st *state) copy() *state {
	st.mtx.RLock()
	defer st.mtx.RUnlock()
	return &state{
		set: st.set,
	}
}

// Encode serializes our complete state to a slice of byte-slices.
// In this simple example, we use a single gob-encoded
// buffer: see https://golang.org/pkg/encoding/gob/
func (st *state) Encode() [][]byte {
	st.mtx.RLock()
	defer st.mtx.RUnlock()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(st.set); err != nil {
		panic(err)
	}
	return [][]byte{buf.Bytes()}
}

func (st *state) Merge(other mesh.GossipData) (complete mesh.GossipData) {
	return st.mergeComplete(other.(*state).copy().set)
}

func (st *state) mergeReceived(set map[mesh.PeerName]StateContent) (received mesh.GossipData) {
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
	return &state{
		set: set, // all remaining elements were novel to us
	}
}

func (st *state) mergeDelta(set map[mesh.PeerName]StateContent) (delta mesh.GossipData) {
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
	return &state{
		set: set, // all remaining elements were novel to us
	}
}

func (st *state) mergeComplete(set map[mesh.PeerName]StateContent) (complete mesh.GossipData) {
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

	return &state{
		set: st.set, // n.b. can't .copy() due to lock contention
	}
}
