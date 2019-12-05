package main

import (
	"github.com/rs/xid"
	"math"
)

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
