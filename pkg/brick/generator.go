package brick

import (
	"github.com/abeja-inc/feature-search-db/pkg/data"
	"log"
	"math/rand"
	"sync"
	"time"
)

// InsertRandomValuesIntoPool inserts randomized values into brick
func InsertRandomValuesIntoPool(fp *FeatureBrick, capacity int) error {
	ta := time.Now().UnixNano()
	div := 1
	wg := sync.WaitGroup{}
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < div; i++ {
		var start, end int
		start = (capacity / div) * i
		if i == div-1 {
			end = capacity
		} else {
			end = (capacity / div) * (i + 1)
		}
		wg.Add(1)
		go func(start int, end int) {
			log.Printf("start=%d end=%d", start, end)
			for j := start; j < end; j++ {
				a := data.NewPosVector(true, 512)
				newDataPoint, _ := fp.AddNewDataPoint(&a)
				newDataPoint.PosVector.LoadPosition(&a)
			}
			wg.Done()
		}(start, end)
	}
	wg.Wait()
	tb := time.Now().UnixNano()
	log.Printf("Init random values within %d msec", (tb-ta)/1000000.0)
	fp.ShowDebug()
	return nil
}
