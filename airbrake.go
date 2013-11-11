package airbrake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/GeertJohan/go.airbat"
	"github.com/subosito/shorturl"
	"io"
	"net/http"
	"os"
	"runtime"
)

const airbrakeNoticeURL = `http://airbrake.io/api/v3/projects/%s/notices?key=%s`

// Brake holds information about the running application
// and provides a set of methods that can be called to send data to the airbrake services.
type Brake struct {
	config    *Config
	context   *context
	projectID string
	apiKey    string
	noticeURL string
}

// Config can be used to set optional preferences and log values
type Config struct {
	// // AppEnvironment, will be sent along with every error log
	// // e.g. "production" or "testing"
	// AppEnvironment string

	// AppVersion, when set, will be sent along with every error log
	AppVersion string

	// AppURL, will be sent along with every error log
	AppURL string

	// User details (for single-user applications)
	UserID    string
	UserName  string
	UserEmail string

	// OutLog, when set, data sentto airbrake is written.
	// Useful for debugging.
	OutLog io.Writer

	// InLog, when set, data received from airbrake is written.
	// Useful for debugging.
	InLog io.Writer

	// HumanLog, when not nil, will write logs to it.
	// These are the same logs as written to Stdout.
	HumanLog io.Writer

	// SilentStdout, when true, won't log anything to Stdout
	SilentStdout bool

	// URLService, when set, will try to shorten the url with given service
	// When this fails, url is not shortened
	// Aitbat url does not require an API call
	URLService string
}

const (
	URLService_None   = ""
	URLService_Airbat = "airbat"

	// other short services as provided by github.com/subosito/shorturl
	URLService_tinyurl  = "tinyurl"
	URLService_isgd     = "isgd"
	URLService_gitio    = "gitio"
	URLService_bitly    = "bitly"
	URLService_lns      = "lns"
	URLService_shorl    = "shorl"
	URLService_vamu     = "vamu"
	URLService_moourl   = "moourl"
	URLService_cligs    = "cligs"
	URLService_snipurl  = "snipurl"
	URLService_adfly    = "adfly"
	URLService_googl    = "googl"
	URLService_gggg     = "gggg"
	URLService_parapt   = "parapt"
	URLService_pendekin = "pendekin"
	URLService_catchy   = "catchy"
	URLService_rddme    = "rddme"
)

var defaultConfig = &Config{}

// NewBrake creates a new *Brake instance
// Config can be nil
func NewBrake(projectID string, key string, environment string, config *Config) *Brake {
	if config == nil {
		config = defaultConfig
	}

	pwd, _ := os.Getwd()

	b := &Brake{
		config: config,
		context: &context{
			OS:            runtime.GOOS + "_" + runtime.GOARCH,
			Language:      runtime.Version(),
			RootDirectory: pwd,

			Environment: environment,
			Version:     config.AppVersion,
			URL:         config.AppURL,

			UserID:    config.UserID,
			UserName:  config.UserName,
			UserEmail: config.UserEmail,
		},
		projectID: projectID,
		apiKey:    key,
		noticeURL: fmt.Sprintf(airbrakeNoticeURL, projectID, key),
	}

	// bezig met het verwerken van sendNotice, schrijf url naar stdout (based on config.SilentStdout), en e.v.t. extra writer (LogWriter)

	return b
}

func (b *Brake) humanLog(msg string) {
	if !b.config.SilentStdout {
		io.WriteString(os.Stdout, msg)
	}
	if b.config.HumanLog != nil {
		io.WriteString(b.config.HumanLog, msg)
	}
}

func (b *Brake) processNotice(not *notice) {
	// get ns
	ns, err := b.sendNotice(not)
	if err != nil {
		b.humanLog(fmt.Sprintf("error processing notice: %s\n", err))
		return
	}

	// get url (depending on config URLService)
	var url string
	switch b.config.URLService {
	case URLService_None:
		url = ns.URL
	case URLService_Airbat:
		url, err = airbat.UintToAirbatURL(ns.ID)
		if err != nil {
			url = ns.URL
		}
	default:
		urlBytes, err := shorturl.Shorten(ns.URL, b.config.URLService)
		if err != nil || len(urlBytes) == 0 {
			// shortening failed, use direct url
			url = ns.URL
		}
		url = string(urlBytes)
	}

	// human log the url
	b.humanLog(fmt.Sprintf("error %s\n", url))
}

func (b *Brake) sendNotice(not *notice) (*noticeSuccess, error) {
	// setup notice
	not.Notifier = Notifier
	not.Context = b.context

	// write notice json to buffer
	buf := bytes.NewBuffer(nil)
	wr := io.Writer(buf)
	if b.config.OutLog != nil {
		wr = io.MultiWriter(wr, b.config.OutLog)
	}
	err := json.NewEncoder(wr).Encode(not)
	if err != nil {
		return nil, fmt.Errorf("error encoding airbrake notice: %s\n", err)
	}

	// make http request to airbrake api
	resp, err := http.Post(b.noticeURL, "application/json", buf)
	if err != nil {
		return nil, fmt.Errorf("error making request to airbake service: %s\n", err)
	}

	// check response to have statuscode 201 created
	if resp.StatusCode == 201 {
		ns := &noticeSuccess{}
		defer resp.Body.Close()

		rd := io.Reader(resp.Body)
		if b.config.InLog != nil {
			rd = io.TeeReader(resp.Body, b.config.InLog)
		}

		err = json.NewDecoder(rd).Decode(ns)
		if err != nil {
			return nil, fmt.Errorf("error decoding response json: %s\n", err)
		}
		return ns, nil
	}

	return nil, fmt.Errorf("unexpected status from api: `%s`", resp.Status)

	//++ TODO handle errors from API
	// defer resp.Body.Close()
	// p, _ := ioutil.ReadAll(resp.Body)
	// os.Stdout.Write(p)

	// // all done
	// return nil, errors.New("didn't finish")
}

type noticeSuccess struct {
	ID  uint64 `json:"id,string"`
	URL string `json:"url"`
}

// // SetUser allows you to set user details on the context of the Brake
// // This avoids creating a new Brake once a user has authenticated in a single-user application.
// // For multi-user applications, use multiple brakes
// func (b *Brake) SetUser(UserID string, UserName string, UserEmail string) {
// 	b.context.UserID = UserID
// 	b.context.UserName = UserName
// 	b.context.UserEmail = UserEmail
// }

// Error logs an error to the airbrake server
//
// example:
// 	brake.Error("EOF", "could not read from file")
func (b *Brake) Error(tipe string, msg string) {
	n := &notice{
		Errors: []*airError{
			&airError{
				Type:    tipe,
				Message: msg,
			},
		},
	}
	b.processNotice(n)
}

// Errorf logs an error to the airbrake server with a format/values error message
// This is acutally just a shorthand for Error(tipe, fmt.Sprintf("format %s %d", str, integer))
//
// example:
// 	brake.Error("EOF", "could not read from file %s", filename)
func (b *Brake) Errorf(tipe string, format string, values ...interface{}) {
	b.Error(tipe, fmt.Sprintf(format, values...))
}

// Recover can be deferred to recover from a panic
//
// example:
// 	func doSomethingDangerous() {
// 		defer brake.Recover()
//
// 		thisMightPanic()
// 		thisMightAlsoPanic()
// 	}
func (b *Brake) Recover() {
	if r := recover(); r != nil {
		b.Error("panic", fmt.Sprint(r))
	}
}

// brakeHTTPHandler implements http.Handler
// it wraps a http.Handler with brake panic recovery
type brakeHTTPHandler struct {
	brake   *Brake
	handler http.Handler
}

// ServeHTTP makes brakeHTTPHandler implement http.Handler
func (h brakeHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer h.brake.Recover()
	h.handler.ServeHTTP(w, r)
}

// WrapHTTPHandler wraps the given http.Handler with in a panic-recovering handler.
// Any recovered panics are reported to airbrake
func (b *Brake) WrapHTTPHandler(handler http.Handler) http.Handler {
	return brakeHTTPHandler{
		brake:   b,
		handler: handler,
	}
}

// WrapHTTPHandlerFunc wraps the given http.HandlerFunc in a panic-recovering handlerFunc.
// Any recovered panics are reported to airbrake
func (b *Brake) WrapHTTPHandlerFunc(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer b.Recover()
		handler(w, r)
	}
}

// type ServeMux struct {
// 	//++
// }

// func (b *Brake) NewServeMux() *ServeMux {
// 	//++
// }

// // Handle registers the handler for the given pattern. If a handler already exists for pattern, Handle panics.
// func (mux *ServeMux) Handle(pattern string, handler Handler) {
// 	//++
// }

// // HandleFunc registers the handler function for the given pattern.
// func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
// 	//++
// }

// // Handler returns the handler to use for the given request, consulting r.Method, r.Host, and r.URL.Path. It always returns a non-nil handler. If the path is not in its canonical form, the handler will be an internally-generated handler that redirects to the canonical path.
// //
// // Handler also returns the registered pattern that matches the request or, in the case of internally-generated redirects, the pattern that will match after following the redirect.
// //
// // If there is no registered handler that applies to the request, Handler returns a “page not found” handler and an empty pattern.
// func (mux *ServeMux) Handler(r *Request) (h Handler, pattern string) {
// 	//++
// }

// // ServeHTTP dispatches the request to the handler whose pattern most closely matches the request URL.
// func (mux *ServeMux) ServeHTTP(w ResponseWriter, r *Request) {
// 	//++
// }

// ErrorData sends an error with data to airbrake
//
// example:
// 	brake.ErrorData("EOF", "could not read from file", airbrake.Data{
// 		Environment: airbrake.Vars{ ... },
// 		Session:     airbrake.vars{"AccountID": accountID},
// 		Params:      airbrake.Vars{"filename": myFile},
// 	})
func (b *Brake) ErrorData(tipe string, msg string, data Data) {
	n := &notice{
		Errors: []*airError{
			&airError{
				Type:    tipe,
				Message: msg,
			},
		},
		Environment: data.Environment,
		Session:     data.Session,
		Params:      data.Params,
	}
	b.processNotice(n)
}

const noticeVersion = "2.3"

type notice struct {
	// Notifier (client library/package)
	Notifier *notifier `json:"notifier"`

	// Context
	Context *context `json:"context"`

	// Error
	Errors []*airError `json:"errors"`

	// Data fields
	Environment Vars `json:"environment,omitempty"`
	Session     Vars `json:"session,omitempty"`
	Params      Vars `json:"params,omitempty"`
}

// Data is to be used with Brake.ErrorData()
// These fields are sent along with the error to airbrake
type Data struct {
	Environment Vars
	Session     Vars
	Params      Vars
}

// Notifier describes the application or library that sends an error to airbrake
type notifier struct {
	// Required. The name of the notifier client submitting the request, such as "hoptoad4j" or "rack-hoptoad."
	Name string `json:"name"`
	// Required. The version number of the notifier client submitting the request.
	Version string `json:"version"`
	// Required. A URL at which more information can be obtained concerning the notifier client.
	URL string `json:"url"`
}

// DefaultNotifier holds information about this go package
// these values are exported so you can override them if you like
var Notifier = &notifier{
	Name:    "go.airbrake",
	Version: "0.1",
	URL:     "https://github.com/GeertJohan/go.airbrake",
}

type context struct {
	OS            string `json:"os"`            // set by pkg (goos+goarch)
	Language      string `json:"language"`      // set by pkg ("go" + version)
	RootDirectory string `json:"rootDirectory"` // set by pkg (pwd)

	Environment string `json:"environment"` // set through config
	Version     string `json:"version"`     // set through config
	URL         string `json:"url"`         // set through config

	UserID    string `json:"userId,omitempty"`    // set through config
	UserName  string `json:"userName,omitempty"`  // set through config
	UserEmail string `json:"userEmail,omitempty"` // set through config
}

// airError contains the error information
type airError struct {
	// The type of error that occurred.
	Type string `json:"type"`
	// A short message describing the error that occurred.
	Message string `json:"message,omitempty"`
	// Stack trace
	Backtrace []line `json:"backtrace,omitempty"`
}

// line from a stack trace
type line struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// Vars types a simple key/value map
type Vars map[string]interface{}
