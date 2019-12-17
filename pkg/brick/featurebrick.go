package brick

import (
	"bytes"
	"encoding/gob"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
	"errors"

	"github.com/abeja-inc/feature-search-db/pkg/calculation"
	"github.com/abeja-inc/feature-search-db/pkg/data"

	"github.com/rs/xid"
)

type BrickID xid.ID
type BrickFeatureGroupID int

type FeatureBrick struct {
	UniqueID             BrickID
	BrickID              BrickID
	FeatureGroupID       BrickFeatureGroupID
	NumOfBrickTotalCap   int
	NumOfAvailablePoints int
	DataPoints           []data.DataPoint
	DataPointMapper      map[data.DataID]*data.DataPoint
	mutex                *sync.Mutex
}

func (fp *FeatureBrick) InitBrick(numOfTotalCap int, featureGroupID BrickFeatureGroupID) error {
	fp.UniqueID = BrickID(xid.New())
	fp.BrickID = BrickID(xid.New())
	fp.FeatureGroupID = featureGroupID
	fp.NumOfBrickTotalCap = numOfTotalCap
	fp.NumOfAvailablePoints = 0
	fp.DataPoints = make([]data.DataPoint, numOfTotalCap, numOfTotalCap)
	for i, _ := range fp.DataPoints {
		fp.DataPoints[i].Available = false
		fp.DataPoints[i].PosVector.InitVector(false, 512)
	}
	fp.DataPointMapper = map[data.DataID]*data.DataPoint{}
	var mutex sync.Mutex
	fp.mutex = &mutex
	return nil
}

func (fp *FeatureBrick) FindDataPointByDataIDstr(dataIDstr string) (*data.DataPoint, error) {
	dataID, err := xid.FromString(dataIDstr)
	if err != nil {
		return nil, nil
	}
	return fp.DataPointMapper[data.DataID(dataID)], nil
}

func (fp *FeatureBrick) GetUniqueIDstr() string {
	return xid.ID(fp.UniqueID).String()
}

func (fp *FeatureBrick) GetBrickIDstr() string {
	return xid.ID(fp.BrickID).String()
}

func (fp *FeatureBrick) GetFeatureGroupIDint() int {
	return int(fp.FeatureGroupID)
}

func (fp *FeatureBrick) Encode() []byte {
	buf := bytes.NewBuffer(nil)
	_ = gob.NewEncoder(buf).Encode(fp)
	return buf.Bytes()

}

func (fp *FeatureBrick) ShowDebug() {
	log.Printf("fp.BrickID=%v\n", fp.BrickID)
	log.Printf("fp.NumOfAvailablePoints=%d\n", fp.NumOfAvailablePoints)
	log.Printf("fp.NumOfBrickTotalCap=%d\n", fp.NumOfBrickTotalCap)
	log.Printf("len(fp.DataPoints)=%d\n", len(fp.DataPoints))
	log.Printf("cap(fp.DataPoints)=%d\n", cap(fp.DataPoints))
}

func (fp *FeatureBrick) AddNewDataPoint(pv *data.PosVector) (*data.DataPoint, error) {
	fp.mutex.Lock()
	defer fp.mutex.Unlock()
	var newDataPoint *data.DataPoint
	if fp.NumOfAvailablePoints == fp.NumOfBrickTotalCap {
		return nil, errors.New("This Pool is full.")
	}
	newDataPoint = &fp.DataPoints[fp.NumOfAvailablePoints]
	newDataPoint.DataID = data.DataID(xid.New())
	newDataPoint.Available = true
	newDataPoint.PosVector.LoadPosition(pv)
	newDataPoint.CreatedAt = time.Now()
	fp.DataPointMapper[newDataPoint.DataID] = newDataPoint
	fp.NumOfAvailablePoints += 1
	return newDataPoint, nil
}

func (fp *FeatureBrick) FindSimilarDataPoint(pv *data.PosVector, method string) (*calculation.DistanceComparingState, error) {
	if fp.NumOfAvailablePoints == 0 {
		return nil, errors.New("No Points available.")
	}
	// 正直にfor分でカリカリ距離計算する
	if method == "naive" {
		return fp.findSimilarDataPointWithNaive(pv)
	}
	// GoRoutineである程度並列しながら、カリカリ計算をする
	if strings.HasPrefix(method, "goroutine_") {
		splittedMethods := strings.Split(method, "_")
		numOfRoutine, err := strconv.Atoi(splittedMethods[1])
		if err != nil {
			return nil, errors.New("Invalid Num of go-routine")
		}
		if numOfRoutine < 1 || numOfRoutine > 100 {
			return nil, errors.New("Num of go-routine must be in range(1-100)")
		}
		return fp.findSimilarDataPointWithGoRoutine(pv, numOfRoutine)
	}
	return nil, errors.New("Unknown method type")
}

func (fp *FeatureBrick) findSimilarDataPointWithNaive(pv *data.PosVector) (*calculation.DistanceComparingState, error) {
	var ret calculation.DistanceComparingState
	ret.SetCandidate(&fp.DataPoints[0], fp.DataPoints[0].GetDistance(pv))
	for i := 1; i < fp.NumOfAvailablePoints; i++ {
		ret.UpdateIfFindMinimum(&fp.DataPoints[i], pv)
	}
	return &ret, nil
}

func (fp *FeatureBrick) findSimilarDataPointWithGoRoutine(pv *data.PosVector, div int) (*calculation.DistanceComparingState, error) {
	resc := make(chan calculation.DistanceComparingState)
	wg := sync.WaitGroup{}
	// 計算範囲の分割と各GoRoutineの起動
	for i := 0; i < div; i++ {
		var start int
		var end int
		start = int(fp.NumOfAvailablePoints/div) * i
		if i == (div - 1) {
			end = fp.NumOfAvailablePoints
		} else {
			end = int(fp.NumOfAvailablePoints/div) * (i + 1)
		}
		wg.Add(1)
		go func(start int, end int, resc chan calculation.DistanceComparingState) {
			var tmp calculation.DistanceComparingState
			tmp.SetCandidate(&fp.DataPoints[start], fp.DataPoints[start].GetDistance(pv))
			for j := start + 1; j < end; j++ {
				tmp.UpdateIfFindMinimum(&fp.DataPoints[j], pv)
			}
			resc <- tmp
			wg.Done()
		}(start, end, resc)
	}
	// 各GoRoutineの計算結果の比較
	ret := <-resc
	for i := 0; i < (div - 1); i++ {
		tmp := <-resc
		ret.UpdateIfFindMinimum(tmp.Result, pv)
	}
	wg.Wait()
	return &ret, nil
}
