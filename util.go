package main

import (
	"errors"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

// 定期的に実行させるための関数
func duringFunc(d time.Duration, f func(), flag *bool) {
	go func() {
		for *flag {
			f()
			time.Sleep(d)
		}
	}()
}

// Brickを乱数で初期化する
func insertRandomValuesIntoPool(fp *FeatureBrick, capacity int) error {
	ta := time.Now().UnixNano()
	div := 1
	wg := sync.WaitGroup{}
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < div; i++ {
		var start, end int
		start = (capacity / div) * i
		if i == div-1 {
			end = capacity
		} else {
			end = (capacity / div) * (i + 1)
		}
		wg.Add(1)
		go func(start int, end int) {
			log.Printf("start=%d end=%d", start, end)
			for j := start; j < end; j++ {
				a := PosVector{}
				a.InitVector(true, 512)
				newDataPoint, _ := fp.AddNewDataPoint(&a)
				newDataPoint.PosVector.LoadPosition(&a)
			}
			wg.Done()
		}(start, end)
	}
	wg.Wait()
	tb := time.Now().UnixNano()
	log.Printf("Init random values within %d msec", (tb-ta)/1000000.0)
	fp.ShowDebug()
	return nil
}

// IPアドレスの自動取得
func getExternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}
