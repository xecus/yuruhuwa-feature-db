package brick

import (
	"math"
	"sync"

	"github.com/abeja-inc/feature-search-db/pkg/calculation"
	"github.com/abeja-inc/feature-search-db/pkg/data"
)

type SearchStrategy interface {
	CreateSearchParameter(map[string]interface{}) SearchParameter
	Search(Data, SearchParameter) *calculation.DistanceComparingState
}

type LinerFindStrategy struct {
}

type LinerDividingFindStrategy struct {
	divideNum int
}

func NewLinerFindStrategy() *LinerFindStrategy {
	return &LinerFindStrategy{}
}

func NewLinerDividingFindStrategy(
	div int,
) *LinerDividingFindStrategy {
	return &LinerDividingFindStrategy{
		div,
	}
}

func (ls *LinerFindStrategy) Search(dataPoints Data, param SearchParameter) *calculation.DistanceComparingState {
	p := param.To().(*LinerFindParameter)
	ret := calculation.DistanceComparingState{}
	ret.SetCandidate(&data.DataPoint{}, math.MaxFloat64)
	for i := 1; i < p.numOfAvailablePoints; i++ {
		ret.UpdateIfFindMinimum(&dataPoints[i], p.targetVector)
	}
	return &ret
}

// TODO: validation before here
func (ls *LinerFindStrategy) CreateSearchParameter(p map[string]interface{}) SearchParameter {
	return &LinerFindParameter{
		targetVector:         p["posVector"].(*data.PosVector),
		numOfAvailablePoints: p["numOfAvailablePoints"].(int),
	}
}

func (ldfs *LinerDividingFindStrategy) Search(dataPoints Data, param SearchParameter) (ret *calculation.DistanceComparingState) {
	p := param.To().(*LinerDividingFindParameter)
	resc := make(chan calculation.DistanceComparingState)
	wg := sync.WaitGroup{}
	// 計算範囲の分割と各GoRoutineの起動
	for i := 0; i < ldfs.divideNum; i++ {
		var start int
		var end int
		start = int(p.numOfAvailablePoints/ldfs.divideNum) * i
		if i == (ldfs.divideNum - 1) {
			end = p.numOfAvailablePoints
		} else {
			end = int(p.numOfAvailablePoints/ldfs.divideNum) * (i + 1)
		}
		wg.Add(1)
		go func(start int, end int, resc chan calculation.DistanceComparingState) {
			var tmp calculation.DistanceComparingState
			// init search value
			tmp.SetCandidate(&dataPoints[start], dataPoints[start].GetDistance(p.targetVector))
			// search loop      liner
			for j := start + 1; j < end; j++ {
				tmp.UpdateIfFindMinimum(&dataPoints[j], p.targetVector)
			}
			resc <- tmp
			wg.Done()
		}(start, end, resc)
	}
	// 各GoRoutineの計算結果の比較
	_ret := <-resc
	ret = &_ret
	for i := 0; i < (ldfs.divideNum - 1); i++ {
		tmp := <-resc
		ret.UpdateIfFindMinimum(tmp.Result, p.targetVector)
	}
	wg.Wait()
	return
}

func (ldfs *LinerDividingFindStrategy) CreateSearchParameter(p map[string]interface{}) SearchParameter {
	return &LinerDividingFindParameter{
		targetVector:         p["posVector"].(*data.PosVector),
		numOfAvailablePoints: p["numOfAvailablePoints"].(int),
	}
}
