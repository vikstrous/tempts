# tempts Testing

## Overview

tempts provides three testing approaches:
1. **Unit tests** -- Full integration test with all implementations registered in a test environment
2. **Mock tests** -- Mock specific activities/workflows and test workflow logic in isolation
3. **Replay tests** -- Fixture-based tests that verify workflow backward compatibility

All approaches use the standard Temporal `testsuite` package.

## Unit Tests

Register all implementations via `tempts.NewWorker`, then use `wrk.Register(we)` to register them in the test environment. Use `ExecuteInTest` to run the workflow and get typed results.

```go
package myapp

import (
    "testing"

    "github.com/vikstrous/tempts"
    "go.temporal.io/sdk/testsuite"
)

func TestGreetWorkflow(t *testing.T) {
    // Create worker with all implementations (same as production)
    wf := workflowTypeGreet.WithImplementation(greetWorkflow)
    wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
        activityTypeGreet.WithImplementation(greetActivity),
        wf,
    })
    if err != nil {
        t.Fatal(err)
    }

    // Set up test environment
    ts := testsuite.WorkflowTestSuite{}
    ts.SetDisableRegistrationAliasing(true)
    we := ts.NewTestWorkflowEnvironment()
    wrk.Register(we)

    // Execute and get typed result
    result, err := wf.ExecuteInTest(we, GreetParams{Name: "World"})
    if err != nil {
        t.Fatal(err)
    }
    if result.Message != "Hello, World" {
        t.Fatalf("Expected 'Hello, World', got %s", result.Message)
    }
}
```

`SetDisableRegistrationAliasing(true)` is required for tempts to work correctly with the test environment.

## Testing Signals

Use `RegisterDelayedCallback` to send signals during workflow execution:

```go
func TestSignal(t *testing.T) {
    wf := workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet)
    wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
        activityTypeFormatName.WithImplementation(activityFormatName),
        activityTypeGreet.WithImplementation(activityGreet),
        wf,
        workflowTypeJustGreet.WithImplementation(workflowJustGreet),
    })
    if err != nil {
        t.Fatal(err)
    }

    ts := testsuite.WorkflowTestSuite{}
    ts.SetDisableRegistrationAliasing(true)
    we := ts.NewTestWorkflowEnvironment()
    wrk.Register(we)

    // Send signal after workflow starts
    we.RegisterDelayedCallback(func() {
        we.SignalWorkflow(signalUpdateSuffix.Name(), UpdateSuffixParams{Suffix: "!!!"})
    }, 0)

    result, err := wf.ExecuteInTest(we, FormatAndGreetParams{Name: "viktor"})
    if err != nil {
        t.Fatal(err)
    }
    if result.Name != "VIKTOR!!!" {
        t.Fatalf("Expected VIKTOR!!!, got %s", result.Name)
    }
}
```

Use `signalUpdateSuffix.Name()` to get the signal channel name for `SignalWorkflow`.

## Mock Tests

Use `RegisterMockFallbacks` to register panicking fallbacks for all activities and workflows on a queue, then mock specific ones with `OnActivity`/`OnWorkflow`:

```go
package myapp

import (
    "testing"

    "github.com/stretchr/testify/mock"
    "go.temporal.io/sdk/testsuite"
)

func TestMocks(t *testing.T) {
    ts := testsuite.WorkflowTestSuite{}
    ts.SetDisableRegistrationAliasing(true)
    we := ts.NewTestWorkflowEnvironment()

    // Register panicking fallbacks for all queue types
    queueMain.RegisterMockFallbacks(we)

    // Mock specific activities/workflows
    we.OnActivity(activityTypeFormatName.Name, mock.Anything, mock.Anything).
        Return(FormatNameResult{Name: "VIKTOR"}, nil)
    we.OnWorkflow(workflowTypeJustGreet.Name(), mock.Anything, mock.Anything).
        Return(JustGreetResult{Name: "Hello, VIKTOR"}, nil)

    wf := workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet)
    result, err := wf.ExecuteInTest(we, FormatAndGreetParams{Name: "viktor"})
    if err != nil {
        t.Fatal(err)
    }
    if result.Name != "VIKTOR" {
        t.Fatal("Expected VIKTOR, got", result.Name)
    }
}
```

Use `activityTypeFormatName.Name` (field, not method) for the activity name in mocks, and `workflowTypeJustGreet.Name()` (method) for the workflow name.

## Replay / Fixture Tests

Replay tests verify that workflow code changes are backward compatible with previous executions. tempts provides `GetWorkflowHistoriesBundle` to capture histories and `ReplayWorkflow` to replay them.

### Recording Fixtures

Connect to a Temporal server with existing workflow executions and capture their histories:

```go
func TestRecordFixtures(t *testing.T) {
    c, err := tempts.Dial(client.Options{})
    if err != nil {
        t.Fatal(err)
    }
    historiesData, err := tempts.GetWorkflowHistoriesBundle(ctx, c, workflowTypeFormatAndGreet)
    if err != nil {
        t.Fatal(err)
    }
    err = os.WriteFile("histories/format_and_greet.json", historiesData, 0o644)
    if err != nil {
        t.Fatal(err)
    }
}
```

`GetWorkflowHistoriesBundle` fetches up to 10 open and 10 closed executions for the given workflow type.

### Replaying from Fixtures

```go
func TestReplayFormatAndGreet(t *testing.T) {
    historiesData, err := os.ReadFile("histories/format_and_greet.json")
    if err != nil {
        t.Fatal(err)
    }
    err = tempts.ReplayWorkflow(historiesData, workflowFormatAndGreet, worker.WorkflowReplayerOptions{})
    if err != nil {
        t.Fatal(err)
    }
}
```

### Recording via Test Flag

A common pattern is to use a test flag to toggle between recording and replaying:

```go
var record bool

func init() {
    flag.BoolVar(&record, "tempts.record", false, "set this to update temporal history fixtures")
}

func TestFormatAndGreetReplayability(t *testing.T) {
    filename := fmt.Sprintf("histories/%s.json", workflowTypeFormatAndGreet.Name())
    testReplayability(t, workflowTypeFormatAndGreet, workflowFormatAndGreet, filename)
}

func testReplayability(t *testing.T, workflowDeclaration tempts.WorkflowDeclaration, fn any, filename string) {
    var historiesData []byte
    if record {
        ctx := context.Background()
        c, err := tempts.Dial(client.Options{})
        if err != nil {
            t.Fatal(err)
        }
        historiesData, err = tempts.GetWorkflowHistoriesBundle(ctx, c, workflowDeclaration)
        if err != nil {
            t.Fatal(err)
        }
        err = os.WriteFile(filename, historiesData, 0o644)
        if err != nil {
            t.Fatal(err)
        }
    } else {
        var err error
        historiesData, err = os.ReadFile(filename)
        if err != nil {
            t.Fatal(err)
        }
    }

    err := tempts.ReplayWorkflow(historiesData, fn, worker.WorkflowReplayerOptions{})
    if err != nil {
        t.Fatal(err)
    }
}
```

Record: `go test -run TestFormatAndGreetReplayability -tempts.record`

Replay (CI): `go test -run TestFormatAndGreetReplayability`
