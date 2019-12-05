package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ---------------------- API for ReverseProxy -----------------------------

type BrickInfoWithNodeInfo struct {
	BrickInfo
	NodeName      string `json:"nodeName"`
	NodeIpAddress string `json:"nodeIpAddress"`
	NodeApiPort   int    `json:nodeApiPort`
}

type NodeStatResponse struct {
	Success      bool   `json:"success"`
	Address      string `json:"address"`
	ResponseTime int64  `json:"responseTime"`
	Contents     string `json:"contents"`
	StatusCode   int    `json:"statusCode"`
}

type ProxyStatResponse struct {
	Bricks             []BrickInfoWithNodeInfo     `json:"bricks"`
	Response           map[string]NodeStatResponse `json:"responses"`
	RequestProcessTime int64                       `json:"requestProcessTime"`
}

type NodeQueryResponse struct {
	Success      bool    `json:"success"`
	Address      string  `json:"address"`
	ResponseTime int64   `json:"responseTime"`
	DataID       string  `json:"dataID"`
	Distance     float64 `json:"distance"`
	StatusCode   int     `json:"statusCode"`
}

type ProxyQueryResult struct {
	DataID   string  `json:"dataID"`
	Distance float64 `json:"distance"`
	IsNew    bool    `json:"isNew"`
}

type ProxyQueryResponse struct {
	Bricks             []BrickInfoWithNodeInfo      `json:"bricks"`
	NodeResponse       map[string]NodeQueryResponse `json:"nodeResponses"`
	Result             ProxyQueryResult             `json:"result"`
	RequestProcessTime int64                        `json:"requestProcessTime"`
}

func handlerOfProxyStat(peer *peer) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		t_start := time.Now().UnixNano()
		// Allow only POST Method
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Invalid method"))
			return
		}
		// Create NodeLists
		status := peer.getAllState()
		bricks := []BrickInfoWithNodeInfo{}
		for nodeName, v := range status.NodeInfos {
			for _, v2 := range *v.Bricks {
				nodeAPIPorts := strings.Split(v.ApiPort, ":")
				nodeAPIPortInt, _ := strconv.Atoi(nodeAPIPorts[1])
				bricks = append(bricks, BrickInfoWithNodeInfo{
					v2,
					nodeName,
					v.IpAddress,
					nodeAPIPortInt,
				})
			}
		}

		// Access Each Node
		responses := map[string]NodeStatResponse{}
		for _, brick := range bricks {
			ta := time.Now().UnixNano()
			address := fmt.Sprintf("http://%s:%d/", brick.NodeIpAddress, brick.NodeApiPort)
			resp, err := http.Get(address)
			if err != nil {
				responses[brick.NodeName] = NodeStatResponse{
					Success: false,
				}
				return
			}
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				responses[brick.NodeName] = NodeStatResponse{
					Success: false,
				}
				return
			}
			tb := time.Now().UnixNano()
			responses[brick.NodeName] = NodeStatResponse{
				Address:      address,
				Success:      true,
				ResponseTime: tb - ta,
				Contents:     string(b),
			}
		}

		t_end := time.Now().UnixNano()

		jsonBytes, _ := json.Marshal(ProxyStatResponse{
			Bricks:             bricks,
			Response:           responses,
			RequestProcessTime: (t_end - t_start),
		})
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	}
}

func handlerOfProxyQuery(peer *peer) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		t_start := time.Now().UnixNano()

		// Allow only POST Method
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Invalid method"))
			return
		}
		defer r.Body.Close()

		// Check Content-Type
		contentType := r.Header.Get("Content-Type")
		if strings.ToLower(contentType) != "application/json" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Invalid Content-Type"))
			return
		}

		// Check GET Query
		v := r.URL.Query()

		if _, ok := v["featureGroupID"]; !ok {
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"FeatureGroupID must be specified"})
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(jsonBytes)
			return
		}
		featureGroupIDint, err := strconv.Atoi(v["featureGroupID"][0])
		if err != nil {
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"Invalid FeatureGroupID"})
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(jsonBytes)
			return
		}

		calcMode := string(CalcModeNaive)
		if _, ok := v["calcMode"]; ok {
			calcMode = v["calcMode"][0]
		}

		// Parse Payload that Client has sent
		query, err := ioutil.ReadAll(r.Body)
		if err != nil {
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"Failed to read payload."})
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(jsonBytes)
			return
		}

		var queryInputForm QueryInputForm
		err = json.Unmarshal(query, &queryInputForm)
		if err != nil {
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"Failed to parse json."})
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(jsonBytes)
			return
		}

		// Create NodeLists
		status := peer.getAllState()
		bricks := []BrickInfoWithNodeInfo{}

		var minBrick BrickInfoWithNodeInfo
		minUsageVal := float64(100.0)

		for nodeName, v := range status.NodeInfos {
			for _, v2 := range *v.Bricks {
				if v2.FeatureGroupID == featureGroupIDint {
					// Add BrickList
					nodeAPIPorts := strings.Split(v.ApiPort, ":")
					nodeAPIPortInt, _ := strconv.Atoi(nodeAPIPorts[1])
					tmp := BrickInfoWithNodeInfo{
						v2,
						nodeName,
						v.IpAddress,
						nodeAPIPortInt,
					}
					usage := float64(v2.NumOfAvailablePoints * 100.0 / v2.NumOfBrickTotalCap)
					if usage < minUsageVal {
						minBrick = tmp
						minUsageVal = usage
					}
					bricks = append(bricks, tmp)
				}
			}
		}

		querbyBytes, err := json.Marshal(queryInputForm)

		// Access Each Node
		processEachNode := func(ch chan map[string]NodeQueryResponse, brick BrickInfoWithNodeInfo, onlyRegister bool) {
			ta := time.Now().UnixNano()
			values := url.Values{}
			values.Add("featureGroupID", fmt.Sprintf("%d", featureGroupIDint))
			if onlyRegister {
				values.Add("onlyRegister", "true")
			} else {
				values.Add("onlyRegister", "false")
				values.Add("calcMode", calcMode)
			}
			address := fmt.Sprintf(
				"http://%s:%d/api/v1/searchQuery?%s",
				brick.NodeIpAddress,
				brick.NodeApiPort,
				values.Encode(),
			)
			resp, err := http.Post(
				address,
				"application/json",
				bytes.NewBuffer(querbyBytes),
			)
			if err != nil {
				ch <- map[string]NodeQueryResponse{
					brick.NodeName: NodeQueryResponse{
						Success: false,
					},
				}
				return
			}
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				ch <- map[string]NodeQueryResponse{
					brick.NodeName: NodeQueryResponse{
						Success: false,
					},
				}
				return
			}

			var tmp struct {
				DataID      string  `json:"dataID"`
				Distance    float64 `json:"distance"`
				ElapsedTime int64   `json:"elapsedTime"`
			}
			json.Unmarshal(b, &tmp)

			tb := time.Now().UnixNano()
			ch <- map[string]NodeQueryResponse{
				brick.NodeName: NodeQueryResponse{
					Address:      address,
					Success:      true,
					ResponseTime: tb - ta,
					DataID:       tmp.DataID,
					Distance:     tmp.Distance,
					StatusCode:   resp.StatusCode,
				},
			}
		}

		ch := make(chan map[string]NodeQueryResponse)
		for _, brick := range bricks {
			go processEachNode(ch, brick, false)
		}

		// Merge
		responses := map[string]NodeQueryResponse{}
		recvCnt := 0
		var minDataID string
		var minDistance float64
		for _ = range bricks {
			for k, v := range <-ch {
				if recvCnt == 0 {
					minDataID = v.DataID
					minDistance = v.Distance
				} else {
					if minDistance > v.Distance {
						minDataID = v.DataID
						minDistance = v.Distance
					}
				}
				responses[k] = v
				recvCnt += 1
			}
		}

		isNew := false
		// TODO: Implement load distance parameter from outside
		if minDistance > 100.0 {
			isNew = true
			go processEachNode(ch, minBrick, true)
			for _, v := range <-ch {
				minDataID = v.DataID
				minDistance = v.Distance
			}
		}

		t_end := time.Now().UnixNano()

		jsonBytes, _ := json.Marshal(ProxyQueryResponse{
			Bricks:       bricks,
			NodeResponse: responses,
			Result: ProxyQueryResult{
				DataID:   minDataID,
				Distance: minDistance,
				IsNew:    isNew,
			},
			RequestProcessTime: (t_end - t_start),
		})
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	}
}

func startReverseProxy(peer *peer, c *ClusterConfigInfo, errs chan error) {

	logger := log.New(os.Stderr, "(Reverse API) > ", log.LstdFlags)

	logRequest := func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
			handler.ServeHTTP(w, r)
		})
	}

	go func(errs chan error) {
		logger.Printf("HTTP server starting (%s)\n", *c.featureApiHttpListen)
		r := httptrace.NewRouter(
			httptrace.WithServiceName("ProyxyAPI"),
		)
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{\"Status\": \"OK From Reverse Proxy\"}"))
		})
		r.HandleFunc("/stat", handlerOfProxyStat(peer))
		r.HandleFunc("/api/v1/searchQuery", handlerOfProxyQuery(peer))
		errs <- http.ListenAndServe(*c.featureApiHttpListen, logRequest(r))
	}(errs)

}
