# Pipeline

## Purpose
This project was inspired by [The Go Blog](https://blog.golang.org/pipelines) and [Pipeline Patterns in Go by Claudio Fahey](https://medium.com/statuscode/pipeline-patterns-in-go-a37bb3a7e61d). We wanted to build a library that we could use "seed" our pipeline, set up as many "stages" as we wanted, than add a "sink" to manage our results. We wanted a library that would manage linking our stages together and manage the lifecycle of our channels and goroutines.
Features that were important to us are:
1. Cancelation - cancel at any stage all routines will stop
2. Error handling - if an error is reportag in any pipeline everything will stop
3. Splitting
4. Merging

## Usage

```go
import https://github.com/LeadsOnDemand/pipeline
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

    p := pipeline.New(seedFactory(os.Stdin), context.Background())

    translator := func(ctx context.Context, in <-chan interface{}) (<-chan interface{}, func() error) {
        out := pipeline.MakeGenericChannel()
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