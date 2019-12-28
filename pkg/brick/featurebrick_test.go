package brick

import (
	"github.com/abeja-inc/feature-search-db/pkg/data"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"
)

const DataCap = 30000

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "TestFeatureBrick: ", log.LstdFlags)
}

func TestFeatureBrick(t *testing.T) {
	t.Run("it testFeatureBrick_Find successfully", testFeatureBrick_Find)
}

func testFeatureBrick_Find(t *testing.T) {
	// prepare
	rand.Seed(time.Now().UnixNano())
	strategy := NewLinerFindStrategy()
	brick := NewBrick(DataCap,
		BrickFeatureGroupID(0),
		strategy,
	)
	_ = InsertRandomValuesIntoPool(&brick, DataCap)

	// Test exist posVector in brick
	{
		// prepare
		randI := rand.Intn(DataCap)
		posVector := brick.DataPoints[randI].PosVector
		param := strategy.CreateSearchParameter(map[string]interface{}{
			"posVector": &posVector,
			"numOfAvailablePoints": brick.NumOfAvailablePoints,
		})
		// exec
		distCompState := brick.Find(param)
		logger.Printf("distance = %f", distCompState.Distance)
		// assert
		if distCompState.Distance != 0 {
			t.Fatal("fail. distance not match.")
		}
	}

	// Test not exist posVector in brick
	{
		// prepare
		posVector := data.NewPosVector(true, 512)
		param := strategy.CreateSearchParameter(map[string]interface{}{
			"posVector": &posVector,
			"numOfAvailablePoints": brick.NumOfAvailablePoints,
		})
		// exec
		distCompState := brick.Find(param)
		logger.Printf("distance = %f", distCompState.Distance)
		// assert
		if distCompState.Distance == 0 {
			t.Fatal("fail. distance not match.")
		}
	}
}

func BenchmarkFeatureBrick_Find_naive(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	strategy := NewLinerFindStrategy()
	brick := NewBrick(DataCap,
		BrickFeatureGroupID(0),
		strategy,
	)
	_ = InsertRandomValuesIntoPool(&brick, DataCap)

	{
		randI := rand.Intn(DataCap)
		posVector := brick.DataPoints[randI].PosVector
		param := strategy.CreateSearchParameter(map[string]interface{}{
			"posVector": &posVector,
			"numOfAvailablePoints": brick.NumOfAvailablePoints,
		})
		b.ResetTimer()
		distCompState := brick.Find(param)

		if distCompState.Distance != 0 {
			b.Fatal("fail. distance not match.")
		}
	}
}

func BenchmarkFeatureBrick_Find_divideWith2Goroutine(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	strategy := NewLinerDividingFindStrategy(2)
	brick := NewBrick(DataCap,
		BrickFeatureGroupID(0),
		strategy,
	)
	_ = InsertRandomValuesIntoPool(&brick, DataCap)

	{
		randI := rand.Intn(DataCap)
		posVector := brick.DataPoints[randI].PosVector
		param := strategy.CreateSearchParameter(map[string]interface{}{
			"posVector": &posVector,
			"numOfAvailablePoints": brick.NumOfAvailablePoints,
		})
		b.ResetTimer()
		distCompState := brick.Find(param)

		if distCompState.Distance != 0 {
			b.Fatal("fail. distance not match.")
		}
	}
}
