package main

type DistanceComparingState struct {
	Result   *DataPoint
	Distance float64
}

func (dcs *DistanceComparingState) SetCandidate(result *DataPoint, distance float64) {
	dcs.Result = result
	dcs.Distance = distance
}

func (dcs *DistanceComparingState) UpdateIfFindMinimum(dp *DataPoint, input *PosVector) {
	val := dp.GetDistance(input)
	if val < dcs.Distance {
		dcs.Result = dp
		dcs.Distance = val
	}
}
