package pipeline

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"
)

type Cancel func(error)

type Pipeline struct {
	cancel    context.CancelFunc
	parents   []*Pipeline
	result    interface{}
	err       error
	next      <-chan interface{}
	context   context.Context
	group     *errgroup.Group
	wg        *sync.WaitGroup
	numStages int
}

type Seed func(context.Context) (<-chan interface{}, func() error)

type Stage func(context.Context, <-chan interface{}) (<-chan interface{}, func() error)

type Sink func(context.Context, <-chan interface{}) (interface{}, error)

func New(seed Seed, ctx context.Context) *Pipeline {
	ctx, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)
	p := Pipeline{
		cancel:    cancel,
		context:   ctx,
		group:     g,
		numStages: 1,
	}
	out, fn := seed(ctx)
	p.next = out
	g.Go(fn)
	return &p
}

func Merge(pipelines []*Pipeline, ctx context.Context) (*Pipeline, error) {
	chs := make([]chan interface{}, len(pipelines))
	for i, pipeline := range pipelines {
		if pipeline.next == nil {
			return nil, errors.New("Cannot merge a pipeline that has been 'Sinked'")
		}
		chs[i] = MakeGenericChannel()
		go pipeline.Sink(syncSinkFactory(chs[i]))
	}
	p := New(faninSeedFactory(chs, pipelines), ctx)
	p.parents = pipelines
	return p, nil
}

func (p *Pipeline) Result() (interface{}, error) {
	if p.next != nil {
		return nil, errors.New("Sink has not been called")
	}

	return p.result, p.err
}

func (p *Pipeline) Stage(stage Stage) error {
	if p.next == nil {
		return errors.New("Cannot add stage after sink")
	}
	out, fn := stage(p.context, p.next)
	p.next = out
	p.numStages++
	p.group.Go(fn)
	return nil
}

func (p *Pipeline) Sink(sink Sink) error {
	defer p.cancel()
	if p.next == nil {
		return errors.New("Cannot add another sink")
	}
	next := p.next
	p.next = nil
	p.group.Go(func() error {
		// when our sink is done callin the sink function for the pipeline
		// and there is a wait group (fanout seed function)
		// we need to wait intill all fanedout seed functions are done before we allow
		// the fanout seed function to stop and close the out channels. If not they will close early and we will not
		// drain the channel
		defer func() {
			if p.wg != nil {
				p.wg.Done()
			}
		}()
		var err error
		p.result, err = sink(p.context, next)
		return err
	})
	p.err = p.group.Wait()
	// if a parrent errored report the parents first error
	if p.parents != nil {
		var parentErr error
		for _, parent := range p.parents {
			parentErr = parent.group.Wait()
			if parentErr != nil {
				p.err = parentErr
				break
			}
		}
	}
	return nil
}

func (p *Pipeline) Split(numPipelines int) ([]*Pipeline, error) {
	if p.next == nil {
		return nil, errors.New("Cannot split after sink")
	}
	if numPipelines <= 1 {
		return nil, errors.New("Number of pipeliens must be greater than one")
	}

	pipelines := make([]*Pipeline, numPipelines)
	//set up a wait group for this pipeline so that we dont kill our context early
	var wg sync.WaitGroup
	wg.Add(numPipelines)
	chs := make([]chan interface{}, numPipelines)
	for i := 0; i < numPipelines; i++ {
		ch := MakeGenericChannel()
		chs[i] = ch
		pipe := New(splitSeedFactory(ch, p.cancel), p.context)
		pipe.parents = []*Pipeline{p}
		pipe.wg = &wg
		pipelines[i] = pipe
	}
	go p.Sink(fanoutSinkFactory(&wg, chs...))
	return pipelines, nil
}

func (p *Pipeline) MakeGenericChannel(capacity ...int) chan interface{} {
	if capacity != nil && len(capacity) >= 1 {
		return make(chan interface{}, capacity[0]+p.numStages)
	}
	return MakeGenericChannel(p.numStages)
}

func splitSeedFactory(in chan interface{}, cancel context.CancelFunc) Seed {
	return func(ctx context.Context) (<-chan interface{}, func() error) {
		out := MakeGenericChannel()
		return out, func() error {
			defer close(out)
			for i := range in {
				select {
				case out <- i:
				case <-ctx.Done():
					// stop the parent to avoid a deadlock
					// since the child will read the parent in case of an error
					cancel()
					return nil
				}
			}
			return nil
		}
	}
}

func closeChannels(chs ...chan interface{}) {
	for _, ch := range chs {
		close(ch)
	}
}

func fanoutSinkFactory(wg *sync.WaitGroup, chs ...chan interface{}) Sink {
	return func(ctx context.Context, in <-chan interface{}) (interface{}, error) {
		for i := range in {
			select {
			case <-ctx.Done():
				{
					closeChannels(chs...)
					return nil, nil
				}
			default:
				{
					for _, ch := range chs {
						select {
						case <-ctx.Done():
							closeChannels(chs...)
							return nil, nil
						case ch <- i:
						}
					}
				}
			}
		}

		// close the channels so that any waiting pipes can also close and propagate
		closeChannels(chs...)
		// waiting for all pipelines to complete reading the channels. If we just exit the errgroup will cancel the context
		// causing the forward pipes to exit early
		wg.Wait()
		return nil, nil
	}
}

func syncSinkFactory(out chan<- interface{}) Sink {
	return func(ctx context.Context, in <-chan interface{}) (interface{}, error) {
		// close the out channels so listeners iterating over values know that this channel is now closed
		defer func() {
			close(out)
		}()
		for i := range in {
			select {
			case <-ctx.Done():
				return nil, nil
			case out <- i:
			}
		}
		return nil, nil
	}
}

func faninSeedFactory(chs []chan interface{}, parents []*Pipeline) Seed {
	return func(ctx context.Context) (<-chan interface{}, func() error) {
		out := MakeGenericChannel()
		return out, func() error {
			var wg sync.WaitGroup
			output := func(c <-chan interface{}) {
				defer wg.Done()
				for i := range c {
					select {
					case <-ctx.Done():
						return
					case out <- i:
					}
				}
			}
			wg.Add(len(chs))
			for _, c := range chs {
				go output(c)
			}

			go func() {
				//when all parent channels are complete or more important the merged pipeline is canceled or has errored
				//we need to close all parent papelines as to avoid a deadlock when the mereged pipeline starts reading
				//parent pipelines to report any errors
				defer func() {
					close(out)
					for _, parent := range parents {
						parent.cancel()
					}
				}()
				wg.Wait()
			}()
			return nil
		}
	}
}

func MakeGenericChannel(capacity ...int) chan interface{} {
	if capacity != nil && len(capacity) >= 1 {
		return make(chan interface{}, capacity[0])
	}
	return make(chan interface{})
}
