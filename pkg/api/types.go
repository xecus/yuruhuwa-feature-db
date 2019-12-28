package api

type CalcModeType string

var (
	CalcModeNaive     CalcModeType = "naive"
	CalcModeGoRoutine CalcModeType = "goroutine"
)

