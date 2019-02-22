package pipeline

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoStagePipeline(t *testing.T) {
	pipeline := New(seedTestFactory(10, passthrough), context.Background())
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Nil(t, sinkErr, "should create sink stage")
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, pipelineResults)
}

func TestPipeline(t *testing.T) {
	pipeline := New(seedTestFactory(10, passthrough), context.Background())
	stageErr := pipeline.Stage(double)
	assert.Nil(t, stageErr, "should create double stage")
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Nil(t, sinkErr, "should create sink stage")
	assert.Equal(t, []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18}, pipelineResults)
}

func TestPipelineMultipleStages(t *testing.T) {
	pipeline := New(seedTestFactory(10, passthrough), context.Background())
	stageErr := pipeline.Stage(double)
	numStages := 100
	for i := 0; i < numStages; i++ {
		pipeline.Stage(stageFactory(passthrough))
	}
	assert.Nil(t, stageErr, "should create double stage")
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Nil(t, sinkErr, "should create sink stage")
	assert.Equal(t, []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18}, pipelineResults)
}

func TestPipelineCancelation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pipeline := New(seedTestFactory(10, passthrough), ctx)
	pipeline.Stage(stageFactory(func(i int) (int, error) {
		if i > 5 {
			cancel()
		}
		return i, nil
	}))
	pipeline.Stage(double)
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Nil(t, sinkErr, "should create sink stage")
	assert.True(t, len(pipelineResults.([]int)) < 10)
}
func TestPipelineError(t *testing.T) {
	pipeline := New(seedTestFactory(10, passthrough), context.Background())
	pipeline.Stage(stageFactory(func(i int) (int, error) {
		if i > 5 {
			return i, errors.New("Value greater than 5")
		}
		return i, nil
	}))
	stageErr := pipeline.Stage(double)
	assert.Nil(t, stageErr, "should create stage")
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Equal(t, sinkErr.Error(), "Value greater than 5")
	assert.True(t, len(pipelineResults.([]int)) < 10)
}

func TestPipelineSplit(t *testing.T) {
	numPipes := 200
	numItems := 10
	pipeline := New(seedTestFactory(numItems, passthrough), context.Background())
	stageErr := pipeline.Stage(double)
	assert.Nil(t, stageErr, "should create double stage")
	pipelines, splitErr := pipeline.Split(numPipes)
	assert.NoError(t, splitErr, "split the pipeline")
	var wg sync.WaitGroup
	wg.Add(numPipes)
	for i := 0; i < numPipes; i++ {
		go func(item int) {
			defer wg.Done()
			pipelines[item].Sink(results)
			pipelineResults, sinkErr := pipelines[item].Result()
			assert.Nil(t, sinkErr, "should create sink stage")
			assert.Equal(t, []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18}, pipelineResults)
		}(i)
	}
	wg.Wait()
}

func TestPipelineSplitError(t *testing.T) {
	pipeline := New(seedTestFactory(10, passthrough), context.Background())
	pipeline.Stage(stageFactory(func(i int) (int, error) {
		if i > 5 {
			return i, errors.New("Value greater than 5")
		}
		return i, nil
	}))
	stageErr := pipeline.Stage(double)
	assert.Nil(t, stageErr, "should create double stage")
	numPipes := 5
	pipelines, splitErr := pipeline.Split(numPipes)
	assert.NoError(t, splitErr, "split the pipeline")
	var wg sync.WaitGroup
	wg.Add(numPipes)
	for i := 0; i < numPipes; i++ {
		go func(item int) {
			defer wg.Done()
			pipelines[item].Sink(results)
			_, sinkErr := pipelines[item].Result()
			assert.Error(t, sinkErr, "Value greater than 5")
		}(i)
	}
	wg.Wait()
}

// does not work
func TestPipelineSplitErrorSplit(t *testing.T) {
	pipeline := New(seedTestFactory(10, passthrough), context.Background())
	stageErr := pipeline.Stage(double)
	assert.Nil(t, stageErr, "should create double stage")
	numPipes := 5
	pipelines, splitErr := pipeline.Split(numPipes)
	pipelines[0].Stage(stageFactory(func(i int) (int, error) {
		if i > 5 {
			return i, errors.New("Value greater than 5")
		}
		return i, nil
	}))
	assert.NoError(t, splitErr, "split the pipeline")
	var wg sync.WaitGroup
	wg.Add(numPipes)
	for i := 0; i < numPipes; i++ {
		go func(item int) {
			defer wg.Done()
			pipelines[item].Sink(results)
			_, sinkErr := pipelines[item].Result()
			if item == 0 {
				assert.Error(t, sinkErr, "Value greater than 5")
			} else {
				assert.NoError(t, sinkErr)
			}
		}(i)
	}
	wg.Wait()
}

func TestPipelineSplitCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pipeline := New(seedTestFactory(10, passthrough), ctx)
	pipeline.Stage(stageFactory(func(i int) (int, error) {
		if i > 5 {
			cancel()
		}
		return i, nil
	}))
	stageErr := pipeline.Stage(double)
	assert.Nil(t, stageErr, "should create double stage")
	numPipes := 5
	pipelines, splitErr := pipeline.Split(numPipes)
	assert.NoError(t, splitErr, "split the pipeline")
	var wg sync.WaitGroup
	wg.Add(numPipes)
	for i := 0; i < numPipes; i++ {
		go func(item int) {
			defer wg.Done()
			pipelines[item].Sink(results)
			pipelineResults, sinkErr := pipelines[item].Result()
			assert.Nil(t, sinkErr, "should create sink stage")
			assert.True(t, len(pipelineResults.([]int)) < 10)
		}(i)
	}
	wg.Wait()
}

func TestPipelineSplitCancelSplit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pipeline := New(seedTestFactory(10, passthrough), ctx)
	stageErr := pipeline.Stage(double)
	assert.Nil(t, stageErr, "should create double stage")
	numPipes := 5
	pipelines, splitErr := pipeline.Split(numPipes)
	assert.NoError(t, splitErr, "split the pipeline")
	pipelines[0].Stage(stageFactory(func(i int) (int, error) {
		if i > 5 {
			cancel()
		}
		return i, nil
	}))
	var wg sync.WaitGroup
	wg.Add(numPipes)
	for i := 0; i < numPipes; i++ {
		go func(item int) {
			defer wg.Done()
			pipelines[item].Sink(results)
			pipelineResults, sinkErr := pipelines[item].Result()
			assert.Nil(t, sinkErr, "should create sink stage")
			assert.True(t, len(pipelineResults.([]int)) < 10)
		}(i)
	}
	wg.Wait()
}

func TestPipelineMerge(t *testing.T) {
	numPipes := 100
	numItems := 100
	var pipelines []*Pipeline
	for i := 0; i < numPipes; i++ {
		pipelines = append(pipelines, New(seedTestFactory(numItems, passthrough), context.Background()))
		stageErr := pipelines[i].Stage(double)
		assert.Nil(t, stageErr, "should create double stage")
	}
	pipeline, mergeErr := Merge(pipelines, context.Background())
	assert.Nil(t, mergeErr, "should create merge pipeline")
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Nil(t, sinkErr, "should create sink stage")
	assert.True(t, len(pipelineResults.([]int)) == numItems*numPipes)
}

func TestPipelineMergeCancelPipeline(t *testing.T) {
	numPipes := 100
	numItems := 100
	ctx, cancel := context.WithCancel(context.Background())
	var pipelines []*Pipeline
	for i := 0; i < numPipes; i++ {
		pipelines = append(pipelines, New(seedTestFactory(numItems, passthrough), ctx))
		pipelines[i].Stage(stageFactory(func(item int) (int, error) {
			if item > 5 {
				cancel()
				return item, nil
			}
			return item, nil
		}))
		stageErr := pipelines[i].Stage(double)
		assert.Nil(t, stageErr, "should create double stage")
	}
	pipeline, mergeErr := Merge(pipelines, context.Background())
	assert.Nil(t, mergeErr, "should create merge pipeline")
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Nil(t, sinkErr, "should create sink stage")
	assert.True(t, len(pipelineResults.([]int)) < numItems*numPipes)
}

func TestPipelineMergeCancelSinglePipeline(t *testing.T) {
	numPipes := 100
	numItems := 100
	ctx, cancel := context.WithCancel(context.Background())
	var pipelines []*Pipeline
	for i := 0; i < numPipes; i++ {
		i := i
		pipelines = append(pipelines, New(seedTestFactory(numItems, passthrough), ctx))
		pipelines[i].Stage(stageFactory(func(item int) (int, error) {
			if item > 5 && i == 5 {
				cancel()
				return item, nil
			}
			return item, nil
		}))
		stageErr := pipelines[i].Stage(double)
		assert.Nil(t, stageErr, "should create double stage")
	}
	pipeline, mergeErr := Merge(pipelines, context.Background())
	assert.Nil(t, mergeErr, "should create merge pipeline")
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Nil(t, sinkErr, "should create sink stage")
	assert.True(t, len(pipelineResults.([]int)) < numItems*numPipes)
}

func TestPipelineMergeCancelMerge(t *testing.T) {
	numPipes := 3
	numItems := 100
	ctx, cancel := context.WithCancel(context.Background())
	var pipelines []*Pipeline
	for i := 0; i < numPipes; i++ {
		pipelines = append(pipelines, New(seedTestFactory(numItems, passthrough), context.Background()))
		stageErr := pipelines[i].Stage(double)
		assert.Nil(t, stageErr, "should create double stage")
	}
	pipeline, mergeErr := Merge(pipelines, ctx)
	assert.Nil(t, mergeErr, "should create merge pipeline")
	canceled := false
	pipeline.Stage(stageFactory(func(item int) (int, error) {
		if item > 5 && !canceled {
			canceled = true
			cancel()
			return item, nil
		}
		return item, nil
	}))
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Nil(t, sinkErr, "should create sink stage")
	assert.True(t, len(pipelineResults.([]int)) < numItems*numPipes)
}

func TestPipelineMergeErrorPipeline(t *testing.T) {
	numPipes := 100
	numItems := 100
	var pipelines []*Pipeline
	for i := 0; i < numPipes; i++ {
		pipelines = append(pipelines, New(seedTestFactory(numItems, passthrough), context.Background()))
		pipelines[i].Stage(stageFactory(func(item int) (int, error) {
			if item > 5 {
				return item, errors.New("Value greater than 5")
			}
			return item, nil
		}))
		stageErr := pipelines[i].Stage(double)
		assert.Nil(t, stageErr, "should create double stage")
	}
	pipeline, mergeErr := Merge(pipelines, context.Background())
	assert.Nil(t, mergeErr, "should create merge pipeline")
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Error(t, sinkErr, "Value greater than 5")
	assert.True(t, len(pipelineResults.([]int)) < numItems*numPipes)
}

func TestPipelineMergeErrorOnePipeline(t *testing.T) {
	numPipes := 100
	numItems := 100
	var pipelines []*Pipeline
	for i := 0; i < numPipes; i++ {
		i := i
		pipelines = append(pipelines, New(seedTestFactory(numItems, passthrough), context.Background()))
		pipelines[i].Stage(stageFactory(func(item int) (int, error) {
			if item > 2 && i == 1 {
				return item, errors.New("Value greater than 5")
			}
			return item, nil
		}))
		stageErr := pipelines[i].Stage(double)
		assert.Nil(t, stageErr, "should create double stage")
	}
	pipeline, mergeErr := Merge(pipelines, context.Background())
	assert.Nil(t, mergeErr, "should create merge pipeline")
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Error(t, sinkErr, "Value greater than 5")
	assert.True(t, len(pipelineResults.([]int)) < numItems*numPipes)
}

func TestPipelineMergeErrorMerge(t *testing.T) {
	numPipes := 1
	numItems := 100
	var pipelines []*Pipeline
	for i := 0; i < numPipes; i++ {
		pipelines = append(pipelines, New(seedTestFactory(numItems, passthrough), context.Background()))
		stageErr := pipelines[i].Stage(double)
		assert.Nil(t, stageErr, "should create double stage")
	}
	pipeline, mergeErr := Merge(pipelines, context.Background())
	assert.Nil(t, mergeErr, "should create merge pipeline")
	pipeline.Stage(stageFactory(func(item int) (int, error) {
		if item > 5 {
			return item, errors.New("Value greater than 5")
		}
		return item, nil
	}))
	pipeline.Sink(results)
	pipelineResults, sinkErr := pipeline.Result()
	assert.Error(t, sinkErr, "Value greater than 5")
	assert.True(t, len(pipelineResults.([]int)) < numItems*numPipes)
}

func seedTestFactory(num int, intercept func(int) (int, error)) Seed {
	return func(ctx context.Context) (<-chan interface{}, func() error) {
		out := MakeGenericChannel()
		return out, func() error {
			defer close(out)
			for i := 0; i < num; i++ {
				value, valueError := intercept(i)
				if valueError != nil {
					return valueError
				}
				select {
				case <-ctx.Done():
					return nil
				case out <- value:
				}
			}
			return nil
		}
	}
}

func double(ctx context.Context, in <-chan interface{}) (<-chan interface{}, func() error) {
	out := MakeGenericChannel()
	return out, func() error {
		defer close(out)
		for i := range in {
			select {
			case <-ctx.Done():
				return nil
			case out <- (i.(int) * 2):
			}
		}
		return nil
	}
}

func results(ctx context.Context, in <-chan interface{}) (interface{}, error) {
	var results []int
	for i := range in {
		select {
		case <-ctx.Done():
			return results, nil
		default:
			results = append(results, i.(int))
		}
	}
	return results, nil
}

func stageFactory(intercept func(int) (int, error)) Stage {
	return func(ctx context.Context, in <-chan interface{}) (<-chan interface{}, func() error) {
		out := MakeGenericChannel()
		return out, func() error {
			defer func() {
				close(out)
			}()
			for i := range in {
				value, valueError := intercept(i.(int))
				if valueError != nil {
					return valueError
				}
				select {
				case <-ctx.Done():
					return nil
				case out <- value:
				}
			}
			return nil
		}
	}
}

func passthrough(i int) (int, error) {
	return i, nil
}
