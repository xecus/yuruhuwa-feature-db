package query

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"github.com/abeja-inc/feature-search-db/pkg/api"
	"github.com/abeja-inc/feature-search-db/pkg/api/proxy"
	"github.com/abeja-inc/feature-search-db/pkg/brick"
	"github.com/abeja-inc/feature-search-db/pkg/cluster"
	"github.com/abeja-inc/feature-search-db/pkg/data"
	"github.com/abeja-inc/feature-search-db/pkg/state"

	"github.com/gorilla/mux"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

func StartFeatureDbServer(bp *brick.BrickPool, c *cluster.ClusterConfigInfo, errs chan error) {
	logger := log.New(os.Stderr, "(Feature API) > ", log.LstdFlags)
	logRequest := func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
			handler.ServeHTTP(w, r)
		})
	}
	// Calcノードが提供するAPI群のエンドポイント定義
	go func(errs chan error) {
		logger.Printf("HTTP server starting (%s)\n", *c.FeatureApiHttpListen)
		r := httptrace.NewRouter(
			httptrace.WithServiceName("QueryAPI"),
		)
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{\"Status\": \"OK From FeatureDb\"}"))
		})
		r.HandleFunc("/api/v1/bricks", handlerOfBricks(bp))
		r.HandleFunc("/api/v1/bricks/{uniqueID}", handlerOfDetailOfBrick(bp))
		r.HandleFunc("/api/v1/bricks/{uniqueID}/datapoints", handlerOfDataPoints(bp))
		r.HandleFunc("/api/v1/bricks/{uniqueID}/datapoints/{dataID}", handlerOfDownloadingDataPoint(bp))
		// ノード間のBrick共有用 (※差分転送実装がまだ)
		r.HandleFunc("/api/v1/bricks/{uniqueID}/download", handlerOfDownloadingBrick(bp))
		// 特徴量検索用エンドポイント
		r.HandleFunc("/api/v1/searchQuery", handlerOfQueryAPI(bp))
		errs <- http.ListenAndServe(*c.FeatureApiHttpListen, logRequest(r))
	}(errs)
}

func handlerOfDownloadingBrick(bp *brick.BrickPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Invalid method"))
			return
		}

		vars := mux.Vars(r)
		IDstr := vars["uniqueID"]

		fb, _ := bp.GetBrickByUniqueIDstr(IDstr)
		if fb == nil {
			resp := struct {
				Msg string `json:"msg"`
			}{"Not Found target brick."}
			jsonBytes, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusNotFound)
			w.Write(jsonBytes)
			return
		}

		ta := time.Now().UnixNano()
		encodedBrick := fb.Encode()
		tb := time.Now().UnixNano()
		fmt.Printf("Encode time = %d nsec \n", tb-ta)

		w.Header().Add("Content-Length", strconv.Itoa(len(encodedBrick)))
		w.Header().Add("Content-Type", "application/force-download")
		w.WriteHeader(http.StatusOK)
		w.Write(encodedBrick)
	}
}

func handlerOfDownloadingDataPoint(bp *brick.BrickPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Invalid method"))
			return
		}

		vars := mux.Vars(r)
		brickUniqueIDstr := vars["uniqueID"]

		fb, _ := bp.GetBrickByUniqueIDstr(brickUniqueIDstr)
		if fb == nil {
			resp := struct {
				Msg string `json:"msg"`
			}{"Not Brick found."}
			jsonBytes, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusNotFound)
			w.Write(jsonBytes)
			return
		}

		dataIDstr := vars["dataID"]

		ta := time.Now().UnixNano()
		dataPoint, _ := fb.FindDataPointByDataIDstr(dataIDstr)
		tb := time.Now().UnixNano()
		elapsedTime := tb - ta

		if dataPoint == nil {
			resp := struct {
				Msg string `json:"msg"`
			}{"No DataPoint found."}
			jsonBytes, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusNotFound)
			w.Write(jsonBytes)
			return

		}

		resp := struct {
			DataID     string    `json:"dataID"`
			Available  bool      `json:"available"`
			PosVector  []float64 `json:"posVector"`
			Hash       string    `json:"hash"`
			CreatedAt  time.Time `json:"createdAt"`
			SearchTime int64     `json:"searchTime"`
		}{
			dataPoint.GetDataIDstr(),
			dataPoint.Available,
			dataPoint.PosVector.Vals,
			dataPoint.PosVector.Hash,
			dataPoint.CreatedAt,
			elapsedTime,
		}
		jsonBytes, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	}
}

func handlerOfDetailOfBrick(bp *brick.BrickPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Invalid method"))
			return
		}

		vars := mux.Vars(r)
		IDstr := vars["uniqueID"]

		fb, _ := bp.GetBrickByUniqueIDstr(IDstr)
		if fb == nil {
			resp := struct {
				Msg string `json:"msg"`
			}{"Not Found target brick."}
			jsonBytes, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusNotFound)
			w.Write(jsonBytes)
			return
		}

		resp := state.BrickInfo{
			UniqueID:             fb.GetUniqueIDstr(),
			BrickID:              fb.GetBrickIDstr(),
			FeatureGroupID:       fb.GetFeatureGroupIDint(),
			NumOfBrickTotalCap:   fb.NumOfBrickTotalCap,
			NumOfAvailablePoints: fb.NumOfAvailablePoints,
		}
		jsonBytes, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	}
}

func handlerOfDataPoints(bp *brick.BrickPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Invalid method"))
			return
		}

		vars := mux.Vars(r)
		IDstr := vars["uniqueID"]

		fb, _ := bp.GetBrickByUniqueIDstr(IDstr)
		if fb == nil {
			resp := struct {
				Msg string `json:"msg"`
			}{"Not Found target brick."}
			jsonBytes, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusNotFound)
			w.Write(jsonBytes)
			return
		}

		dataPoints := map[string]string{}
		for _, dp := range fb.DataPoints {
			if !dp.Available {
				continue
			}
			hash, _ := dp.PosVector.CalcHash()
			dataPoints[dp.GetDataIDstr()] = hash
		}

		resp := struct {
			DataPoints map[string]string `json: "dataPoints"`
		}{
			DataPoints: dataPoints,
		}
		jsonBytes, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	}
}

func handlerOfBricks(bp *brick.BrickPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Invalid method"))
			return
		}

		bricks, _ := bp.GetAllBricks()
		resp := make([]state.BrickInfo, 0, len(bricks))
		for _, fb := range bricks {
			resp = append(resp, state.BrickInfo{
				UniqueID:             fb.GetUniqueIDstr(),
				BrickID:              fb.GetBrickIDstr(),
				FeatureGroupID:       fb.GetFeatureGroupIDint(),
				NumOfBrickTotalCap:   fb.NumOfBrickTotalCap,
				NumOfAvailablePoints: fb.NumOfAvailablePoints,
			})
		}
		jsonBytes, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	}
}

func handlerOfQueryAPI(bp *brick.BrickPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		span := tracer.StartSpan("handlerOfQueryAPI")
		defer span.Finish()

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

		// Parse GET Query
		v := r.URL.Query()
		/*
			for key, vs := range v {
				fmt.Printf("%s = %s\n", key, vs[0])
			}
		*/

		if _, ok := v["featureGroupID"]; !ok {
			// featureGroupID is Necesarry parameter
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"GroupID must be specified"})
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(jsonBytes)
			return
		}
		featureGroupIDint, err := strconv.Atoi(v["featureGroupID"][0])
		if err != nil {
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"Invalid GroupID"})
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(jsonBytes)
			return
		}
		featureGroupID := brick.BrickFeatureGroupID(featureGroupIDint)

		onlyRegister := false
		if _, ok := v["onlyRegister"]; ok {
			onlyRegister, err = strconv.ParseBool(v["onlyRegister"][0])
			if err != nil {
				jsonBytes, _ := json.Marshal(struct {
					Msg string `json:"msg"`
				}{"Invalid Flag (onlyRegister)"})
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write(jsonBytes)
				return
			}
		}

		calcMode := string(api.CalcModeNaive)
		if _, ok := v["calcMode"]; ok {
			calcMode = v["calcMode"][0]
		}

		// Parse payload from client
		var queryInputForm proxy.QueryInputForm
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"Failed to read payload."})
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(jsonBytes)
			return
		}
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&queryInputForm)
		if err != nil {
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"Failed to parse json."})
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write(jsonBytes)
			return
		}

		target := data.PosVector{}
		target.InitVector(false, 512)
		target.LoadPositionFromArray(queryInputForm.Vals)
		fps, _ := bp.GetBrickByGroupID(featureGroupID)
		if fps == nil {
			jsonBytes, _ := json.Marshal(struct {
				Msg string `json:"msg"`
			}{"Not found Brick"})
			w.WriteHeader(http.StatusNotFound)
			w.Write(jsonBytes)
			return
		}
		fp := fps[0]

		if onlyRegister {
			ta := time.Now().UnixNano()
			datPoint, err := fp.AddNewDataPoint(&target)
			tb := time.Now().UnixNano()
			elapsedTime := tb - ta
			if err != nil {
				jsonBytes, _ := json.Marshal(struct {
					Msg string `json:"msg"`
				}{"Failed to register new dataPoint"})
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(jsonBytes)
				return
			}
			jsonBytes, _ := json.Marshal(struct {
				DataID      string  `json:"dataID"`
				Distance    float64 `json:"distance"`
				ElapsedTime int64   `json:"elapsedTime"`
				Registered  bool    `json:"registered"`
			}{
				DataID:      datPoint.GetDataIDstr(),
				Distance:    -1,
				ElapsedTime: elapsedTime,
				Registered:  true,
			})
			w.WriteHeader(http.StatusOK)
			w.Write(jsonBytes)
		} else {
			ta := time.Now().UnixNano()
			ret, _ := fp.FindSimilarDataPoint(&target, calcMode)
			tb := time.Now().UnixNano()
			elapsedTime := tb - ta
			jsonBytes, _ := json.Marshal(struct {
				DataID      string  `json:"dataID"`
				Distance    float64 `json:"distance"`
				ElapsedTime int64   `json:"elapsedTime"`
				Registered  bool    `json:"registered"`
				CalcMode    string  `json:"calcMode"`
			}{
				DataID:      ret.Result.GetDataIDstr(),
				Distance:    ret.Distance,
				ElapsedTime: elapsedTime,
				Registered:  false,
				CalcMode:    calcMode,
			})
			w.WriteHeader(http.StatusOK)
			w.Write(jsonBytes)
		}
	}
}
