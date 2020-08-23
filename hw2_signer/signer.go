package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// сюда писать код

var mux sync.Mutex

// ExecutePipeline ...
func ExecutePipeline(jobs ...job) {
	if len(jobs) == 0 {
		return
	}
	var wg sync.WaitGroup
	in := make(chan interface{})
	defer close(in)
	for _, job := range jobs {
		out := make(chan interface{})
		wg.Add(1)
		go runWorker(&wg, job, in, out)
		in = out
	}
	wg.Wait()
}

func runWorker(wg *sync.WaitGroup, j job, in, out chan interface{}) {
	j(in, out)
	wg.Done()
	close(out)
}

// SingleHash ...
func SingleHash(in, out chan interface{}) {
	var wgr sync.WaitGroup
	for step0 := range in {
		wgr.Add(1)
		go func(step0 interface{}) {
			data := fmt.Sprintf("%v", step0)

			mux.Lock()
			md5 := DataSignerMd5(data) // 0.1 sec
			mux.Unlock()

			var hash1 string
			var hash2 string

			var wg sync.WaitGroup
			wg.Add(1)
			go Crc32(&wg, &hash1, data)
			wg.Add(1)
			go Crc32(&wg, &hash2, md5)
			wg.Wait() // 1 sec

			result := hash1 + "~" + hash2

			out <- result
			wgr.Done()
		}(step0)
	}
	wgr.Wait()
}

// Crc32 ...
func Crc32(wg *sync.WaitGroup, res *string, data string) {
	*res = DataSignerCrc32(data) // 1 sec
	wg.Done()
}

// MultiHash ...
func MultiHash(in, out chan interface{}) {
	var wgr sync.WaitGroup
	for step1 := range in {
		wgr.Add(1)
		go func(step1 interface{}) {
			data := fmt.Sprintf("%v", step1)
			var wg sync.WaitGroup
			mult := make([]string, 6)
			for i := 0; i < 6; i++ {
				wg.Add(1)
				go func(j int) {
					mult[j] = DataSignerCrc32(strconv.Itoa(j) + data)
					wg.Done()
				}(i)
			}
			wg.Wait() // 1 sec

			result := strings.Join(mult, "")

			out <- result
			wgr.Done()
		}(step1)
	}
	wgr.Wait()
}

// CombineResults ...
func CombineResults(in, out chan interface{}) {
	var wgr sync.WaitGroup
	var mx sync.RWMutex

	result := make([]string, 0)
	for step2 := range in {
		wgr.Add(1)
		go func(step2 interface{}) {
			data, ok := step2.(string)
			if ok {
				mx.Lock()
				fmt.Println(data, " : ", len(result))
				result = append(result, data)
				mx.Unlock()
			}
			wgr.Done()
		}(step2)
	}
	wgr.Wait()
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	combination := strings.Join(result, "_")
	out <- combination

}

func main() {
	inputData := []int{0, 1, 1, 2, 3, 5, 8}

	hashSignJobs := []job{
		job(func(in, out chan interface{}) {
			for _, fibNum := range inputData {
				out <- fibNum
			}
		}),
		job(SingleHash),
		job(MultiHash),
		job(CombineResults),
		job(func(in, out chan interface{}) {
			dataRaw := <-in
			data, ok := dataRaw.(string)
			if !ok {

			}
			testResult := data
			fmt.Print(testResult)
		}),
	}

	ExecutePipeline(hashSignJobs...)
}
