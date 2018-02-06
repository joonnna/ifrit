package core

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/joonnna/ifrit/logger"
)

type recorder struct {
	log *logger.Log

	recordFlag      bool
	recordMutex     sync.RWMutex
	recordTimestamp time.Time
	recordDuration  float64

	completedRequests      int
	completedRequestsMutex sync.RWMutex

	failedRequests      int
	failedRequestsMutex sync.RWMutex

	latencies      []float64
	latenciesMutex sync.RWMutex
}

func (r *recorder) addLatency(lat float64) {
	r.latenciesMutex.Lock()
	defer r.latenciesMutex.Unlock()

	r.latencies = append(r.latencies, lat)
}

func (r *recorder) getLatencies() []float64 {
	r.latenciesMutex.RLock()
	defer r.latenciesMutex.RUnlock()

	ret := make([]float64, len(r.latencies))

	copy(ret, r.latencies)

	return ret
}

func (r *recorder) setRecordFlag(value bool) {
	r.recordMutex.Lock()
	defer r.recordMutex.Unlock()

	if !r.recordFlag && value {
		r.recordTimestamp = time.Now()
	}

	r.recordFlag = value
}

func (r *recorder) getRecordFlag() bool {
	r.recordMutex.RLock()
	defer r.recordMutex.RUnlock()

	if !r.recordFlag {
		return r.recordFlag
	}

	since := time.Since(r.recordTimestamp)
	if since.Minutes() > r.recordDuration {
		return false
	}

	return r.recordFlag
}

func (r *recorder) incrementCompleted() {
	r.completedRequestsMutex.Lock()
	defer r.completedRequestsMutex.Unlock()

	r.completedRequests++
}

func (r *recorder) getCompletedRequests() int {
	r.completedRequestsMutex.RLock()
	defer r.completedRequestsMutex.RUnlock()

	return r.completedRequests
}

func (r *recorder) incrementFailed() {
	r.failedRequestsMutex.Lock()
	defer r.failedRequestsMutex.Unlock()

	r.failedRequests++
}

func (r *recorder) getFailedRequests() int {
	r.failedRequestsMutex.RLock()
	defer r.failedRequestsMutex.RUnlock()

	return r.failedRequests
}

func (r *recorder) doExp(args *expArgs) {
	r.log.Info.Printf("Starting experiment, sleeping for %d minutes\n", args.Duration)
	f, err := os.OpenFile("/home/uitple_firechain/res", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		r.log.Err.Println(err)
		return
	}

	time.Sleep(time.Minute * time.Duration(args.Duration))

	completed := r.getCompletedRequests()
	failed := r.getFailedRequests()

	f.Write([]byte(fmt.Sprintf("%d\t%d\t%f\t%d\t%d\n", args.NumRings, args.MaxConc, args.Byz, completed, failed)))
	f.Close()
}
