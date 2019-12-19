package brick

import "github.com/abeja-inc/feature-search-db/pkg/data"

type SearchParameter interface {
	To() interface{}
}

type LinerFindParameter struct {
	numOfAvailablePoints int
	targetVector *data.PosVector
}

type LinerDividingFindParameter struct {
	numOfAvailablePoints int
	targetVector *data.PosVector
}

func (lfp *LinerFindParameter) To() interface{} {
	return lfp
}

func (ldfp *LinerDividingFindParameter) To() interface{} {
	return ldfp
}
