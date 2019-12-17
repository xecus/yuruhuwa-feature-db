package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/abeja-inc/feature-search-db/pkg/brick"
	"github.com/abeja-inc/feature-search-db/pkg/state"

	"github.com/weaveworks/mesh"
)

type ClusterPeers map[string]struct{}

type ClusterConfigInfo struct {
	SizeOfInitBrick      *int
	IpAddress            *string
	FeatureApiHttpListen *string
	NodeRole             *string
	stateApiHttpListen   *string
	meshListen           *string
	hwaddr               *string
	nickname             *string
	password             *string
	channel              *string
	Peers                ClusterPeers
}

func NewConfig(
	sizeOfInitBrick *int,
	ipAddress *string,
	featureApiHttpListen *string,
	nodeRole *string,
	stateApiHttpListen *string,
	meshListen *string,
	hwaddr *string,
	nickname *string,
	password *string,
	channel *string,
	peers ClusterPeers,
) ClusterConfigInfo {
	return ClusterConfigInfo{
		SizeOfInitBrick:      sizeOfInitBrick,
		IpAddress:            ipAddress,
		FeatureApiHttpListen: featureApiHttpListen,
		NodeRole:             nodeRole,
		stateApiHttpListen:   stateApiHttpListen,
		meshListen:           meshListen,
		hwaddr:               hwaddr,
		nickname:             nickname,
		password:             password,
		channel:              channel,
		Peers:                peers,
	}
}

func (cci ClusterConfigInfo) StateConfig() state.PeerConfig {
	return state.NewPeerConfig(
		*cci.IpAddress,
		*cci.FeatureApiHttpListen,
	)
}

func (cci *ClusterConfigInfo) Show() {
	fmt.Printf("nodeRole=%s\n", *cci.NodeRole)
	fmt.Printf("FeatureApiHttpListen=%s\n", *cci.FeatureApiHttpListen)
	fmt.Printf("stateApiHttpListen=%s\n", *cci.stateApiHttpListen)
	fmt.Printf("meshListen=%s\n", *cci.meshListen)
	fmt.Printf("hwaddr=%s\n", *cci.hwaddr)
	fmt.Printf("nickname=%s\n", *cci.nickname)
	fmt.Printf("password=%s\n", *cci.password)
	fmt.Printf("channel=%s\n", *cci.channel)
	for k, _ := range cci.Peers {
		fmt.Printf("peers[%s]\n", k)
	}
}

func StartClusteringFunc(c ClusterConfigInfo, errs chan error) *state.Peer {

	logger := log.New(os.Stderr, "(Clustering) "+*c.nickname+"> ", log.LstdFlags)

	host, portStr, err := net.SplitHostPort(*c.meshListen)
	if err != nil {
		logger.Fatalf("mesh address: %s: %v", *c.meshListen, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		logger.Fatalf("mesh address: %s: %v", *c.meshListen, err)
	}

	name, err := mesh.PeerNameFromString(*c.hwaddr)
	if err != nil {
		logger.Fatalf("%s: %v", *c.hwaddr, err)
	}

	router, err := mesh.NewRouter(mesh.Config{
		Host:               host,
		Port:               port,
		ProtocolMinVersion: mesh.ProtocolMinVersion,
		Password:           []byte(*c.password),
		ConnLimit:          64,
		PeerDiscovery:      true,
		TrustedSubnets:     []*net.IPNet{},
	}, name, *c.nickname, mesh.NullOverlay{}, log.New(ioutil.Discard, "", 0))

	if err != nil {
		logger.Fatalf("Could not create router: %v", err)
	}

	peer := state.NewPeer(name, logger)
	gossip, err := router.NewGossip(*c.channel, peer)
	if err != nil {
		logger.Fatalf("Could not create gossip: %v", err)
	}
	peer.Register(gossip)

	func() {
		logger.Printf("mesh router starting (%s)", *c.meshListen)
		router.Start()
	}()
	defer func() {
		logger.Printf("mesh router stopping")
		router.Stop()
	}()
	router.ConnectionMaker.InitiateConnections(c.Peers.slice(), true)

	go func(errs chan error) {
		logger.Printf("HTTP server starting (%s)", *c.stateApiHttpListen)
		http.HandleFunc("/", handlerOfClusterApiController(peer))
		errs <- http.ListenAndServe(*c.stateApiHttpListen, nil)
	}(errs)
	return peer
}

type PeerController interface {
	GetAllState() state.StateContent
	SetNodeInfo(peerConfig state.PeerConfig, bp *brick.BrickPool) state.StateContent
	Del() state.StateContent
}

func handlerOfClusterApiController(pc PeerController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			bytes, _ := json.Marshal(pc.GetAllState())
			s := string(bytes)
			fmt.Fprintf(w, s)
		case http.MethodDelete:
			bytes, _ := json.Marshal(pc.Del())
			s := string(bytes)
			fmt.Fprintf(w, s)
		case http.MethodPost:
			defer r.Body.Close()
			//b, err := ioutil.ReadAll(r.Body)
			//if err != nil {
			//}
			//err = json.NewDecoder(bytes.NewReader(b)).Decode(&queryInputForm)
			//bytes, _ := json.Marshal(pc.setNodeInfo())
			//s := string(bytes)
			//fmt.Fprintf(w, s)
			fmt.Fprintf(w, "Not Implemented")
		}
	}
}

func (ss ClusterPeers) Set(value string) error {
	ss[value] = struct{}{}
	return nil
}

func (ss ClusterPeers) String() string {
	return strings.Join(ss.slice(), ",")
}

func (ss ClusterPeers) slice() []string {
	slice := make([]string, 0, len(ss))
	for k := range ss {
		slice = append(slice, k)
	}
	sort.Strings(slice)
	return slice
}

func MustHardwareAddr() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	for _, iface := range ifaces {
		if s := iface.HardwareAddr.String(); s != "" {
			return s
		}
	}
	panic("no valid network interfaces")
}

func MustHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return hostname
}
