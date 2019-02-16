package pipeline

import (
	"context"
	"errors"
	"sync"
)

type Pipeline struct {
	Next    <-chan interface{}
	Context context.Context
	Cancel  context.CancelFunc
	Errcs   []<-chan error
}

type Seed func(context.Context) (<-chan interface{}, <-chan error, error)

type Stage func(context.Context, <-chan interface{}) (<-chan interface{}, <-chan error, error)

type Sink func(context.Context, <-chan interface{}) (<-chan error, error)

func SeedPipeline(seed Seed) (*Pipeline, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	p := Pipeline{
		Cancel:  cancelFunc,
		Context: ctx,
	}
	out, errc, err := seed(ctx)
	if err != nil {
		return nil, err
	}
	p.Errcs = []<-chan error{errc}
	p.Next = out
	p.Context = ctx
	return &p, nil
}

func (p *Pipeline) StagePipeline(stage Stage) error {
	if p.Next == nil {
		return errors.New("Cannot add stage after sink")
	}
	out, errc, err := stage(p.Context, p.Next)
	if err != nil {
		return err
	}
	p.Next = out

	p.Errcs = append(p.Errcs, errc)
	return nil
}

func (p *Pipeline) SinkPipeline(sink Sink) error {
	if p.Next == nil {
		return errors.New("Cannot add another sink")
	}
	errc, err := sink(p.Context, p.Next)
	if err != nil {
		return err
	}
	p.Next = nil
	p.Errcs = append(p.Errcs, errc)

	return waitForErrors(p.Errcs...)
}

func mergeErrors(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	// we do not want to block so if every error closes and returns
	// we want to be able to buffer that return
	out := make(chan error, len(cs))

	// create a function to loop through all the items in a channel and add merge it
	// to the out channel
	output := func(c <-chan error) {
		for e := range c {
			out <- e
		}
		wg.Done()
	}

	// set the wait group to wait for all channels
	wg.Add(len(cs))
	// watch all error channels
	for _, c := range cs {
		go output(c)
	}

	// start a goroutine to wait for the wait group before closing
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func waitForErrors(errs ...<-chan error) error {
	errc := mergeErrors(errs...)

	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
}

func MakeGenericChannel(capacity ...int) chan interface{} {
	if capacity != nil && len(capacity) >= 1 {
		return make(chan interface{}, capacity[0])
	}
	return make(chan interface{})
}

func MakeErrorChannel(capacity ...int) chan error {
	if capacity != nil && len(capacity) >= 1 {
		return make(chan error, capacity[0])
	}
	return make(chan error, 1)
}
