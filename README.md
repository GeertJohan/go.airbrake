## go.airbrake

go.airbrake provides airbrake v3 error logging functionality.

[![GoDoc](https://godoc.org/github.com/GeertJohan/go.airbrake?status.png)](https://godoc.org/github.com/GeertJohan/go.airbrake)

### Installation
`go get github.com/GeertJohan/go.airbrake`

### Usage
First, create a Brake:
``` go
brake := airbrake.NewBrake("projectID", "apiKey", "application environment", nil)
```
Then, use the brake when there is a problem:
```go
brake.Errorf("user-problem", "User has problem: %s", problem)
```

You can also use the brake to recover from a panic
```go
func doStuff() {
	defer brake.Recover()

	// do stuff

	// suddenly, a wild panic appears
	panic("oh noes! a panic!")
}
```
The deferred call to `brake.Recover()` recovers from the panic and sends the message to Airbrake.io. Error class will be "panic".

You can use the WrapHTTP* methods to wrap any http.Handler or http.HandlerFunc. This recovers the Handler/HandlerFunc from any panic.
```go
//++ TODO, example
```

### Todo
 - add tests
 - add examples
 - make logged error more descriptive (log the message)
 - make logged error configurable (disable type, disable message, cap message to chars (0=infinite), disable url)