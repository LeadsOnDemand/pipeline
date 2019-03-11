package main

import (
	"context"
	"fmt"
	"os"
	"runtime/trace"
	"sync"

	"github.com/LeadsOnDemand/pipeline"
)

func main() {
	terr := trace.Start(os.Stdout)
	if terr != nil {
		panic(terr)
	}
	defer trace.Stop()
	numItems := 1000
	numStages := 1000
	numPipes := 100

	p := pipeline.New(context.Background(), func(ctx context.Context) (<-chan interface{}, func() error) {
		out := pipeline.MakeGenericChannel()
		return out, func() error {
			defer close(out)
			for i := 0; i < numItems; i++ {
				out <- i
			}
			return nil
		}
	})

	var pipes []*pipeline.Pipeline

	if numPipes > 1 {
		pps, splitErr := p.Split(numPipes)
		if splitErr != nil {
			panic(splitErr)
		}
		pipes = pps
	} else {
		pipes = []*pipeline.Pipeline{p}
	}

	passthrough := func(p *pipeline.Pipeline) pipeline.Stage {
		return func(ctx context.Context, in <-chan interface{}) (<-chan interface{}, func() error) {
			out := p.MakeGenericChannel()
			return out, func() error {
				defer close(out)
				for i := range in {
					select {
					case <-ctx.Done():
						return nil
					case out <- i:
					}
				}
				return nil
			}
		}
	}

	for _, pipe := range pipes {
		for i := 0; i < numStages; i++ {
			pipe.Stage(passthrough(pipe))
		}
	}

	sink := func(crx context.Context, in <-chan interface{}) (interface{}, error) {
		sum := 0
		for i := range in {
			sum += i.(int)
		}
		return sum, nil
	}

	var wg sync.WaitGroup
	wg.Add(numPipes)
	for _, pipe := range pipes {
		pipe := pipe
		go func() {
			defer wg.Done()
			pipe.Sink(sink)
			result, resultErr := pipe.Result()
			if resultErr != nil {
				fmt.Printf("err = %v\n", resultErr)
			}
			fmt.Printf("result = %v\n", result)
		}()
	}
	wg.Wait()
}
