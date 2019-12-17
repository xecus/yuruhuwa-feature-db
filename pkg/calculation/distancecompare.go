package calculation

import (
	"github.com/abeja-inc/feature-search-db/pkg/data"
)

type DistanceComparingState struct {
	Result   *data.DataPoint
	Distance float64
}

func (dcs *DistanceComparingState) SetCandidate(result *data.DataPoint, distance float64) {
	dcs.Result = result
	dcs.Distance = distance
}

func (dcs *DistanceComparingState) UpdateIfFindMinimum(dp *data.DataPoint, input *data.PosVector) {
	val := dp.GetDistance(input)
	if val < dcs.Distance {
		dcs.Result = dp
		dcs.Distance = val
	}
}
