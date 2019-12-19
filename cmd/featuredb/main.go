package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/abeja-inc/feature-search-db/pkg/api/proxy"
	"github.com/abeja-inc/feature-search-db/pkg/api/query"
	"github.com/abeja-inc/feature-search-db/pkg/brick"
	"github.com/abeja-inc/feature-search-db/pkg/cluster"
	"github.com/abeja-inc/feature-search-db/pkg/util"

	"github.com/weaveworks/mesh"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func processSignal(errs chan error) {
	go func(errs chan error) {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}(errs)
}

func main() {
	ipAddress, err := util.GetExternalIP()
	if err != nil {
		fmt.Printf("Failed to get IPAddress")
		return
	}

	// Input
	clusterConfigInfo := cluster.NewConfig(
		flag.Int("size_of_init_brick", 100000, "Size of Initial brick"),
		flag.String("ipaddress", ipAddress, "Local IP"),
		flag.String("feature_api", ":8081", "HTTP listen address (API)"),
		flag.String("node_role", "calc", "RoleName in cluster"),
		flag.String("state_api", ":8001", "HTTP listen address (ClusterManage)"),
		flag.String("mesh", net.JoinHostPort("0.0.0.0", strconv.Itoa(mesh.Port)), "mesh listen address"),
		flag.String("hwaddr", cluster.MustHardwareAddr(), "MAC address, i.e. mesh peer ID"),
		flag.String("nickname", cluster.MustHostname(), "peer nickname"),
		flag.String("password", "", "password (optional)"),
		flag.String("channel", "default", "gossip channel name"),
		cluster.ClusterPeers{},
	)
	flag.Var(clusterConfigInfo.Peers, "peer", "initial peer (may be repeated)")
	flag.Parse()

	errs := make(chan error)
	processSignal(errs)

	// 計算ノードとして動くモード
	if *clusterConfigInfo.NodeRole == "calc" {
		tracer.Start(
			tracer.WithServiceName("feature-db-calcNode"),
			tracer.WithAnalytics(true),
		)
		defer tracer.Stop()
		cpus := runtime.NumCPU()
		runtime.GOMAXPROCS(cpus)
		fmt.Printf("CPU=%d\n", cpus)

		fp := brick.NewBrick(*clusterConfigInfo.SizeOfInitBrick,
			0,
			//brick.NewLinerFindStrategy(), // TODO: to be changeable
			brick.NewLinerDividingFindStrategy(2), // TODO: to be changeable
		)
		brick.InsertRandomValuesIntoPool(&fp, *clusterConfigInfo.SizeOfInitBrick)

		bp := brick.BrickPool{}
		bp.InitBrickPool()
		bp.RegisterIntoPool(&fp)

		query.StartFeatureDbServer(&bp, &clusterConfigInfo, errs)
		peer := cluster.StartClusteringFunc(clusterConfigInfo, errs)
		stateConf := clusterConfigInfo.StateConfig()
		go func(peer cluster.PeerController) {
			for true {
				peer.SetNodeInfo(stateConf, &bp)
				time.Sleep(10 * time.Second)
			}
		}(peer)
		fmt.Println(<-errs)
		os.Exit(0)
	}

	// ReverseProxyとして動くモード
	if *clusterConfigInfo.NodeRole == "reverseProxy" {
		tracer.Start(
			tracer.WithServiceName("feature-db-proxy"),
			tracer.WithAnalytics(true),
		)
		peer := cluster.StartClusteringFunc(clusterConfigInfo, errs)
		proxy.StartReverseProxy(peer, *clusterConfigInfo.FeatureApiHttpListen, errs)
		go func(peer cluster.PeerController) {
			for true {
				fmt.Print(peer.GetAllState())
				time.Sleep(10 * time.Second)
			}
		}(peer)
		fmt.Println(<-errs)
		os.Exit(0)
	}

}
