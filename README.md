# Pipeline
![Build Status](https://codebuild.us-east-1.amazonaws.com/badges?uuid=eyJlbmNyeXB0ZWREYXRhIjoiU1pLeGhFcWVtRkdMSzREekJMbzlMWTBhbmovMWhpTm0zSkxVSk5CT1JGY1NGcS9Hc0ErSjZpZUNna3dudlRYcGRVUjdtVmhwSWVhSTE5bkFkeDhYNnRJPSIsIml2UGFyYW1ldGVyU3BlYyI6Iko1VXVraVNpbmVNZzNwRGIiLCJtYXRlcmlhbFNldFNlcmlhbCI6MX0%3D&branch=master)

## Purpose
This project was inspired by [The Go Blog](https://blog.golang.org/pipelines) and [Pipeline Patterns in Go by Claudio Fahey](https://medium.com/statuscode/pipeline-patterns-in-go-a37bb3a7e61d). We wanted to build a library that we could use "seed" our pipeline, set up as many "stages" as we wanted, than add a "sink" to manage our results. We wanted a library that would manage linking our stages together and manage the lifecycle of our channels and goroutines.
Features that were important to us are:
1. Cancelation - cancel at any stage all routines will stop
2. Error handling - if an error is reportag in any pipeline everything will stop
3. Splitting
4. Merging

## Usage

```go
import (
    "github.com/kazzcade/pipeline"
)
```

### Create a pipeline
You create a pipeline by calling pipeline.New() passing in a function with the signature

```go
func(context.Context) (<-chan interface{}, func() error)
```

#### Simple example pipeline seeded with 10 numbers

```go
seed := func(ctx context.Context, in <-chan interface{}) (<-chan interface{}, func() error) {
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
pipeline := pipeline.new(seed)
```

#### Something a bit more complicated
We read input data and translate it

```go
seedFactory := func(in *os.File) pipeline.Seed {
        reader := bufio.NewReader(in)
        fmt.Println("Simple Shell")
        fmt.Println("---------------------")
        return func(ctx context.Context) (<-chan interface{}, func() error) {
            out := pipeline.MakeGenericChannel()
            return out, func() error {
                defer close(out)
                for {
                    fmt.Print("-> ")
                    select {
                    // return when canceled
                    case <-ctx.Done():
                    default:
                        text, _ := reader.ReadString('\n')
                        // convert CRLF to LF
                        out <- strings.Replace(text, "\n", "", -1)
                    }
                }
                return nil
            }
        }
    }

    p := pipeline.New(context.Background(), seedFactory(os.Stdin))

    translator := func(ctx context.Context, in <-chan interface{}) (<-chan interface{}, func() error) {
        out := p.MakeGenericChannel()
        return out, func() error {
            // clean up when done
            defer close(out)
            for i := range in {
        select {
        case <- ctx.Done():
          return nil
        default:
          // example google translate not working code
          translation, err := google.translate(i.(string))
          if (err != nil) {
            // stops our pipeline
            return err
          }
          // send the output to our next stage
          out <- translation
        }
            }
            return nil
        }
    }
  p.Stage(translator)
    sink := func(ctx context.Context, in <-chan interface{}) (interface{}, error) {
        transcript := ""
        for i := range in {
            select {
            case <-ctx.Done():
                return transcript, nil
            default:
                transcript = transcript + i.(string) + "\n"
                fmt.Println(i)
            }
        }
        return transcript, nil
    }

    p.Sink(sink)

```

### Performance
Pipeline provided two methods of creating a "generic channel" which is simply `chan interface{}`.
1. pipeline.MakeGenericChannel - this will create an unbuffered channel
2. p.MakeGenericChannel where p is an instance of Pipeline. When using the instance method of MakeGenericChannel we will create a buffered channel with the buffer amount defined by the number of stages. If we have 3 stages we would have seed->{}->{}{}->{}{}{}. Since we have three stages we can seed up to three items before our final stage is full and two items before our second and one before our first. Our pipeline buffer will not block until all buffers in all stages are full, 6 items total, given there are no consumers.

If the production does not need to be controlled by the consumption it is recommended that you use the instance method of MakeGenericChannel as your code will not block for long.

#### Performance time results
On a 2017 MacBook Pro i7 16GB ram pushing 1000 integers through 1000 stages for 100 pipelines
With instance method MakeGenericChannel
```bash
real    0m5.073s
user    0m36.481s
sys     0m0.902s
```

With static method MakeGenericChannel
```bash
real    0m11.651s
user    1m28.406s
sys     0m0.539s
```