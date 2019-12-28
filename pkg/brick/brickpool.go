package brick

import (
	"errors"
	"sync"

	"github.com/rs/xid"
)

type BrickPool struct {
	mutex                        *sync.Mutex
	UniqueIDRelationMapper       map[BrickID]*FeatureBrick
	BrickIDRelationMapper        map[BrickID][]*FeatureBrick
	FeatureGroupIDRelationMapper map[BrickFeatureGroupID][]*FeatureBrick
}

func (bp *BrickPool) InitBrickPool() error {
	var mutex sync.Mutex
	bp.mutex = &mutex
	bp.UniqueIDRelationMapper = map[BrickID]*FeatureBrick{}
	bp.BrickIDRelationMapper = map[BrickID][]*FeatureBrick{}
	bp.FeatureGroupIDRelationMapper = map[BrickFeatureGroupID][]*FeatureBrick{}
	return nil
}

func (bp *BrickPool) RegisterIntoPool(fb *FeatureBrick) error {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if _, ok := bp.UniqueIDRelationMapper[fb.BrickID]; ok {
		return errors.New("Already registered.")
	}
	bp.UniqueIDRelationMapper[fb.UniqueID] = fb

	if _, ok := bp.BrickIDRelationMapper[fb.BrickID]; ok {
		bp.BrickIDRelationMapper[fb.BrickID] = append(
			bp.BrickIDRelationMapper[fb.BrickID], fb)
	} else {
		bp.BrickIDRelationMapper[fb.BrickID] = []*FeatureBrick{fb}
	}

	if _, ok := bp.FeatureGroupIDRelationMapper[fb.FeatureGroupID]; ok {
		bp.FeatureGroupIDRelationMapper[fb.FeatureGroupID] = append(
			bp.FeatureGroupIDRelationMapper[fb.FeatureGroupID], fb)
	} else {
		bp.FeatureGroupIDRelationMapper[fb.FeatureGroupID] = []*FeatureBrick{fb}
	}

	return nil
}

func (bp *BrickPool) GetAllBricks() (map[BrickID]*FeatureBrick, error) {
	return bp.UniqueIDRelationMapper, nil
}

func (bp *BrickPool) GetAllBrickUniqueIDs() ([]string, error) {
	keys := make([]string, 0, len(bp.UniqueIDRelationMapper))
	for k := range bp.UniqueIDRelationMapper {
		keys = append(keys, xid.ID(k).String())
	}
	return keys, nil
}

/*
func (bp *BrickPool) GetFeatureGroupID(brickIDstr string) (BrickFeatureGroupID, error) {
	brickID, err := xid.FromString(brickIDstr)
	if err != nil {
		return BrickFeatureGroupID(-1), nil
	}
	return bp.BrickIDRelationMapper[BrickID(brickID)].FeatureGroupID, nil
}
*/

func (bp *BrickPool) GetBrickByUniqueIDstr(brickIDstr string) (*FeatureBrick, error) {
	brickID, err := xid.FromString(brickIDstr)
	if err != nil {
		return nil, err
	}
	return bp.GetBrickByUniqueID(BrickID(brickID))
}

func (bp *BrickPool) GetBrickByUniqueID(brickID BrickID) (*FeatureBrick, error) {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()
	if val, ok := bp.UniqueIDRelationMapper[brickID]; ok {
		return val, nil
	}
	return nil, errors.New("Could not find target brick")
}

func (bp *BrickPool) GetBricksByBrickID(brickID BrickID) ([]*FeatureBrick, error) {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()
	if val, ok := bp.BrickIDRelationMapper[brickID]; ok {
		return val, nil
	}
	return nil, errors.New("Could not find target brick")
}

func (bp *BrickPool) GetBrickByGroupID(featureGroupID BrickFeatureGroupID) ([]*FeatureBrick, error) {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()
	if val, ok := bp.FeatureGroupIDRelationMapper[featureGroupID]; ok {
		return val, nil
	}
	return nil, errors.New("Could not find target brick")
}
