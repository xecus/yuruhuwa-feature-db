package data

import (
	"math"
	"time"

	"github.com/rs/xid"
)

type DataPoint struct {
	DataID    DataID
	Available bool
	PosVector PosVector
	CreatedAt time.Time
}

func (dp *DataPoint) GetDataIDstr() string {
	return xid.ID(dp.DataID).String()
}

func (dp *DataPoint) GetDistance(pv *PosVector) float64 {
	var acc float64
	acc = 0.0
	for i, _ := range dp.PosVector.Vals {
		acc += (dp.PosVector.Vals[i] - pv.Vals[i]) * (dp.PosVector.Vals[i] - pv.Vals[i])
	}
	return math.Sqrt(acc)
}
