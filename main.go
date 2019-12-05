package main

import (
	"flag"
	"fmt"
	"github.com/weaveworks/mesh"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

func processSignal(errs chan error) {
	go func(errs chan error) {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}(errs)
}

func main() {

	ipAddress, err := getExternalIP()
	if err != nil {
		fmt.Printf("Failed to get IPAddress")
		return
	}

	// Input
	clusterConfigInfo := ClusterConfigInfo{
		sizeOfInitBrick:      flag.Int("size_of_init_brick", 100000, "Size of Initial brick"),
		ipAddress:            flag.String("ipaddress", ipAddress, "Local IP"),
		featureApiHttpListen: flag.String("feature_api", ":8081", "HTTP listen address (API)"),
		nodeRole:             flag.String("node_role", "calc", "RoleName in cluster"),
		stateApiHttpListen:   flag.String("state_api", ":8001", "HTTP listen address (ClusterManage)"),
		meshListen:           flag.String("mesh", net.JoinHostPort("0.0.0.0", strconv.Itoa(mesh.Port)), "mesh listen address"),
		hwaddr:               flag.String("hwaddr", mustHardwareAddr(), "MAC address, i.e. mesh peer ID"),
		nickname:             flag.String("nickname", mustHostname(), "peer nickname"),
		password:             flag.String("password", "", "password (optional)"),
		channel:              flag.String("channel", "default", "gossip channel name"),
		peers:                ClusterPeers{},
	}
	flag.Var(clusterConfigInfo.peers, "peer", "initial peer (may be repeated)")
	flag.Parse()

	tracer.Start(tracer.WithServiceName("test-go"))
	defer tracer.Stop()

	errs := make(chan error)
	processSignal(errs)

	// 計算ノードとして動くモード
	if *clusterConfigInfo.nodeRole == "calc" {

		cpus := runtime.NumCPU()
		runtime.GOMAXPROCS(cpus)
		fmt.Printf("CPU=%d\n", cpus)

		fp := FeatureBrick{}
		fp.InitBrick(*clusterConfigInfo.sizeOfInitBrick, 0)
		insertRandomValuesIntoPool(&fp, *clusterConfigInfo.sizeOfInitBrick)

		bp := BrickPool{}
		bp.InitBrickPool()
		bp.RegisterIntoPool(&fp)

		startFeatureDbServer(&bp, &clusterConfigInfo, errs)
		peer := startClusteringFunc(clusterConfigInfo, errs)
		go func(peer peerController) {
			for true {
				peer.setNodeInfo(&clusterConfigInfo, &bp)
				time.Sleep(10 * time.Second)
			}
		}(peer)
		fmt.Println(<-errs)
		os.Exit(0)
	}

	// ReverseProxyとして動くモード
	if *clusterConfigInfo.nodeRole == "reverseProxy" {
		peer := startClusteringFunc(clusterConfigInfo, errs)
		startReverseProxy(peer, &clusterConfigInfo, errs)
		go func(peer peerController) {
			for true {
				fmt.Print(peer.getAllState())
				time.Sleep(10 * time.Second)
			}
		}(peer)
		fmt.Println(<-errs)
		os.Exit(0)
	}

}
