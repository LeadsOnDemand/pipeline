package main

import (
	"context"
	"fmt"
	"os"
	"runtime/trace"

	"github.com/LeadsOnDemand/pipeline"
)

func main() {
	trace.Start(os.Stdout)
	defer trace.Stop()
	numItems := 2000

	p := pipeline.New(func(ctx context.Context) (<-chan interface{}, func() error) {
		out := pipeline.MakeGenericChannel()
		return out, func() error {
			defer close(out)
			for i := 0; i < numItems; i++ {
				out <- i
			}
			return nil
		}
	}, context.Background())
	fibonacci := func(ctx context.Context, in <-chan interface{}) (<-chan interface{}, func() error) {
		var sequence []int
		out := pipeline.MakeGenericChannel()
		return out, func() error {
			defer close(out)
			for i := range in {
				sequence = append(sequence, i.(int))
				out <- sequence
			}
			return nil
		}
	}
	p.Stage(fibonacci)
	sink := func(crx context.Context, in <-chan interface{}) (interface{}, error) {
		var result []int
		for i := range in {
			sequence := i.([]int)
			f := 0
			for _, v := range sequence {
				f += v
			}
			result = append(result, f)
		}
		return result, nil
	}

	p.Sink(sink)
	result, err := p.Result()
	if err != nil {
		fmt.Println(err.Error)
	} else {
		fmt.Println(result.([]int))
	}
}
