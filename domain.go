package main

import (
	"github.com/rs/xid"
	"sync"
	"time"
)

type DataID xid.ID

type QueryInputForm struct {
	Vals [512]float64 `json:"vals"`
}

type DataPoint struct {
	DataID    DataID
	Available bool
	PosVector PosVector
	CreatedAt time.Time
}

type BrickPool struct {
	mutex                        *sync.Mutex
	UniqueIDRelationMapper       map[BrickID]*FeatureBrick
	BrickIDRelationMapper        map[BrickID][]*FeatureBrick
	FeatureGroupIDRelationMapper map[BrickFeatureGroupID][]*FeatureBrick
}

type CalcModeType string

var (
	CalcModeNaive     CalcModeType = "naive"
	CalcModeGoRoutine CalcModeType = "goroutine"
)
