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
	"strings"
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
	// AppVersion, when set, will be sent along with every error notice
	AppVersion string

	// AppURL, when set, will be sent along with every error notice
	AppURL string

	// User details (for single-user applications)
	// You can change these later-on via SetUserDetails(..)
	UserID    string
	UserName  string
	UserEmail string

	// DebugLogOut, when set, data sentto airbrake servers is written to this io.Writer
	// Useful for debugging.
	DebugLogOut io.Writer

	// DebugLogIn, when set, data received from airbrake servers is written to this io.Writer
	// Useful for debugging.
	DebugLogIn io.Writer

	// LogWriter, when not nil, will write logs to it.
	// These are the same logs as written to Stdout by default.
	LogWriter io.Writer

	// LogStdoutSilent, when true, won't log anything to Stdout
	LogStdoutSilent bool

	// URLService, when set, will try to shorten the url with given service
	// When this fails, url is not shortened and original url is used.
	// Aitbat url is calculated client-side and does not require an extra API call
	URLService string
}

const (
	// URLServiceNone disables short url service
	URLServiceNone = ""

	// URLServiceAirbat enables airb.at short url service for airbrake
	URLServiceAirbat = "airbat"

	// URLServiceTinyurl provides short url via tinyurl
	URLServiceTinyurl = "tinyurl"
	// URLServiceIsgd provides short url via is.gd
	URLServiceIsgd = "isgd"
	// URLServiceGitio provides short url via git.io
	URLServiceGitio = "gitio"
	// URLServiceBitly provides short url via bit.ly
	URLServiceBitly = "bitly"
	// URLServiceLns provides short url via lns
	URLServiceLns = "lns"
	// URLServiceShorl provides short url via shorl
	URLServiceShorl = "shorl"
	// URLServiceVamu provides short url via vamu
	URLServiceVamu = "vamu"
	// URLServiceMoourl provides short url via moourl
	URLServiceMoourl = "moourl"
	// URLServiceCligs provides short url via cligs
	URLServiceCligs = "cligs"
	// URLServiceSnipurl provides short url via snipurl
	URLServiceSnipurl = "snipurl"
	// URLServiceAdfly provides short url via adfly
	URLServiceAdfly = "adfly"
	// URLServiceGoogl provides short url via goo.gl
	URLServiceGoogl = "googl"
	// URLServiceGggg provides short url via gggg
	URLServiceGggg = "gggg"
	// URLServiceParapt provides short url via parapt
	URLServiceParapt = "parapt"
	// URLServicePendekin provides short url via pendekin
	URLServicePendekin = "pendekin"
	// URLServiceCatchy provides short url via catchy
	URLServiceCatchy = "catchy"
	// URLServiceRddme provides short url via rddme
	URLServiceRddme = "rddme"
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

	return b
}

// SetUserDetails changes the UserDetails on a Brake
// This overwrites the UserID, UserName and UserEmail values that you might've set with Config (NewBrake() argument)
func (b *Brake) SetUserDetails(id, name, email string) {
	b.context.UserID = id
	b.context.UserName = name
	b.context.UserEmail = email
}

func (b *Brake) humanLog(msg string) {
	if !b.config.LogStdoutSilent {
		io.WriteString(os.Stdout, msg)
	}
	if b.config.LogWriter != nil {
		io.WriteString(b.config.LogWriter, msg)
	}
}

func (b *Brake) processNotice(not *notice) {
	// setup notice
	not.Notifier = Notifier
	not.Context = b.context

	// create backtrace
	//++ TODO: multi-thread (multi-goroutine) backtraces
	//++ TODO: find out the limit of backtraces for a notify and limit to that
	if not.Errors[0].Backtrace == nil {
		not.Errors[0].Backtrace = make([]line, 0, 4)
	}

	// get stack
	for skip := 1; ; skip++ {
		// get caller details
		pc, callerFile, callerLine, ok := runtime.Caller(skip)
		if !ok {
			break
		}

		// get func for pc
		f := runtime.FuncForPC(pc)
		funcName := f.Name()
		// only skip leading frames
		if len(not.Errors[0].Backtrace) == 0 {
			// skip leading frames from the actual error into this package
			if strings.Contains(funcName, "airbrake.(*Brake).") {
				continue
			}
			// skip the runtime panic/recovery
			if funcName == "runtime.panic" {
				continue
			}
		}

		// add line to backtrace
		not.Errors[0].Backtrace = append(not.Errors[0].Backtrace, line{
			File:     callerFile,
			Line:     callerLine,
			Function: funcName,
		})
	}

	// get ns
	ns, err := b.sendNotice(not)
	if err != nil {
		b.humanLog(fmt.Sprintf("error processing notice: %s\n", err))
		return
	}

	// get url (depending on config URLService)
	var url string
	switch b.config.URLService {
	case URLServiceNone:
		url = ns.URL
	case URLServiceAirbat:
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

	// write notice json to buffer
	buf := bytes.NewBuffer(nil)
	wr := io.Writer(buf)
	if b.config.DebugLogOut != nil {
		wr = io.MultiWriter(wr, b.config.DebugLogOut)
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
		if b.config.DebugLogIn != nil {
			rd = io.TeeReader(resp.Body, b.config.DebugLogIn)
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

// Notify logs an error to the airbrake server
//
// example:
// 	brake.Notify("EOF", "could not read from file")
func (b *Brake) Notify(errorClass string, errorMessage string) {
	n := &notice{
		Errors: []*airError{
			&airError{
				Type:    errorClass,
				Message: errorMessage,
			},
		},
	}
	b.processNotice(n)
}

// Notifyf logs an error to the airbrake server with a format/values error message
// This is acutally just a shorthand for Error(errorClass, fmt.Sprintf("format %s %d", str, integer))
//
// example:
// 	brake.Notifyf("error", "could not read from file %s", filename)
func (b *Brake) Notifyf(errorClass string, format string, values ...interface{}) {
	b.Error(errorClass, fmt.Sprintf(format, values...))
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
//
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

// NotifyData sends an error with data to airbrake
//
// example:
// brake.NotifyData("EOF", "could not read from file", airbrake.Data{
// 	Environment: airbrake.Vars{"GOPATH": os.Getenv("GOPATH")},
// 	Session:     airbrake.Vars{"AccountID": 1337},
// 	Params:      airbrake.Vars{"filename": "foo.bar", "object": airbrake.Vars{"foo": "bar", "number": 42}},
// })
func (b *Brake) NotifyData(errorClass string, errorMessage string, data Data) {
	n := &notice{
		Errors: []*airError{
			&airError{
				Type:    errorClass,
				Message: errorMessage,
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

// Notifier holds information about this go package (the notifier)
// these values are exported so you can override them if you wish
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
