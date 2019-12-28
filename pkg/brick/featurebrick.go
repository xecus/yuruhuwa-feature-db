package brick

import (
	"bytes"
	"encoding/gob"
	"errors"
	"log"
	"sync"
	"time"

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
	searchStrategy       SearchStrategy
}

func NewBrick(
	numOfTotalCap int,
	featureGroupID BrickFeatureGroupID,
	strategy SearchStrategy,
) FeatureBrick {
	dataPoints := make([]data.DataPoint, numOfTotalCap, numOfTotalCap)
	for i, _ := range dataPoints {
		dataPoints[i].Available = false
		dataPoints[i].PosVector = data.NewPosVector(false, 512)
	}
	var mutex sync.Mutex
	return FeatureBrick{
		UniqueID:             BrickID(xid.New()),
		BrickID:              BrickID(xid.New()),
		FeatureGroupID:       featureGroupID,
		NumOfBrickTotalCap:   numOfTotalCap,
		NumOfAvailablePoints: 0,
		DataPoints:           dataPoints,
		DataPointMapper:      map[data.DataID]*data.DataPoint{},
		mutex:                &mutex,
		searchStrategy:       strategy,
	}
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

func (fp *FeatureBrick) CreateSearchParam(params map[string]interface{}) SearchParameter {
	return fp.searchStrategy.CreateSearchParameter(params)
}

func (fp *FeatureBrick) Find(param SearchParameter) (ret *calculation.DistanceComparingState) {
	return fp.searchStrategy.Search(fp.DataPoints, param)
}

