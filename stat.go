package main

import (
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"
)

type Metric struct {
	errResp  bool
	duration int
}

type Stat struct {
	rwm           sync.RWMutex
	totalRequest  int
	totalResponse int
	successTotal  int
	errTotal      int
	duration      []int
}

func InitStat() *Stat {
	return &Stat{
		rwm:      sync.RWMutex{},
		duration: make([]int, 0, 100000),
	}
}

func (s *Stat) calculate(timer time.Duration) {
	maxValue := slices.Max(s.duration)
	avg := s.getAvg()
	minValue := slices.Min(s.duration)

	ms := int(time.Millisecond.Nanoseconds())
	durationSec := int((timer * time.Minute).Seconds())

	//100000 * 600 = 60.000.000
	//
	//9.522.377 /

	fmt.Println("====================== Results ======================")
	fmt.Print(fmt.Sprintf("Total request count:  %d \n", s.totalRequest))
	fmt.Print(fmt.Sprintf("Total response count:  %d \n", s.totalResponse))
	fmt.Print(fmt.Sprintf("Error count:  %d \n", s.errTotal))
	fmt.Print(fmt.Sprintf("Success count:  %d \n", s.successTotal))
	fmt.Print(fmt.Sprintf("RPS:  %d \n", s.totalResponse/durationSec)) // реально отправленные и длительность(это еще норм)
	fmt.Print(fmt.Sprintf("Max:  %dms \n", maxValue/ms))
	fmt.Print(fmt.Sprintf("Min:  %dms \n", minValue/ms))
	fmt.Print(fmt.Sprintf("Avg:  %dms\n", int(avg)/ms))
	fmt.Print(fmt.Sprintf("95:  %dms \n", s.percentile(0.95)/ms))
	fmt.Print(fmt.Sprintf("99: %dms \n", s.percentile(0.99)/ms))
	fmt.Print(fmt.Sprintf("99.9: %dms \n", s.percentile(0.999)/ms))
}

func (s *Stat) percentile(n float64) int {
	sort.Ints(s.duration)
	nineFive := float64(len(s.duration)-1) * n

	newSlice := s.duration[int(nineFive):]
	return newSlice[0]
}

func (s *Stat) readMetrics(mCH chan Metric, readEnd chan bool) {
	for metric := range mCH {
		//metric, ok := <-mCH
		//if !ok {
		//	readEnd <- true
		//	break
		//}

		if metric.errResp {
			s.errTotal++
		} else {
			s.successTotal++
		}

		s.totalResponse++
		s.duration = append(s.duration, metric.duration)
	}

	readEnd <- true
}

func (s *Stat) getAvg() float64 {
	sum := 0
	for _, v := range s.duration {
		sum += v
	}
	return (float64(sum)) / (float64(len(s.duration)))
}

func (s *Stat) incrReq() {
	s.rwm.Lock()
	s.totalRequest++
	s.rwm.Unlock()
}
