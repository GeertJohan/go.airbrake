package airbrake

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

const airbrakeNoticeURL = `http://api.airbrake.io/notifier_api/v2/notices`

// Brake holds information about the running application
// and provides a set of methods that can be called to send data to the airbrake services.
type Brake struct {
	config      *Config
	environment *environment
	apiKey      string
}

// Config can be used to set optional preferences and log values
type Config struct {
	// AppVersion, when set, will be sent along with every error log
	AppVersion string

	// LogWriter, when not nil, will write logs to it
	LogWriter io.Writer

	// SilentStdout, when true, won't log anything to Stdout
	SilentStdout bool
}

var defaultConfig = &Config{
	AppVersion: "",
}

// NewBrake creates a new *Brake instance
func NewBrake(key string, name string, config *Config) *Brake {
	if config == nil {
		config = defaultConfig
	}

	b := &Brake{
		config: config,
		environment: &environment{
			Name:    name,
			Version: config.AppVersion,
		},
		apiKey: key,
	}

	// bezig met het verwerken van sendNotice, schrijf url naar stdout (based on config.SilentStdout), en e.v.t. extra writer (LogWriter)

	return b
}

// Error logs an error to the airbrake server
// example: brake.Error("EOF", "could not read from file")
func (b *Brake) Error(tipe string, msg string) {
	n := &notice{
		Error: &airError{
			Type:    tipe,
			Message: msg,
		},
	}
	b.sendNotice(n)
}

type noticeSuccess struct {
	XMLName interface{} `xml:"notice"`
	ID      int64       `xml:"id"`
	URL     string      `xml:"url"`
}

func (b *Brake) sendNotice(not *notice) (*noticeSuccess, error) {
	// setup notice
	not.Version = noticeVersion
	not.APIKey = b.apiKey
	not.Notifier = Notifier
	not.Environment = b.environment

	// write notice xml to buffer
	buf := bytes.NewBuffer(nil)
	err := xml.NewEncoder(buf).Encode(not)
	if err != nil {
		return nil, fmt.Errorf("error encoding airbrake notice: %s\n", err)
	}

	// make http request to airbrake api
	resp, err := http.Post(airbrakeNoticeURL, "text/xml", buf)
	if err != nil {
		return nil, fmt.Errorf("error making request to airbake service: %s\n", err)
	}

	// check response
	fmt.Printf("response statuscode: %d\n", resp.StatusCode)
	if resp.StatusCode == 200 {
		ns := &noticeSuccess{}
		defer resp.Body.Close()
		err = xml.NewDecoder(resp.Body).Decode(ns)
		if err != nil {
			return nil, fmt.Errorf("error decoding response xml: %s\n", err)
		}
		return ns, nil
	}

	defer resp.Body.Close()
	p, _ := ioutil.ReadAll(resp.Body)
	os.Stdout.Write(p)

	// all done
	return nil, errors.New("didn't finish")
}

// Errorf logs an error to the airbrake server with a format/values error message
// This is acutally just a shorthand for Error(tipe, fmt.Sprintf("format %s %d", str, integer))
// example: brake.Error("EOF", "could not read from file %s", filename)
func (b *Brake) Errorf(tipe string, format string, values ...interface{}) {
	b.Error(tipe, fmt.Sprintf(format, values...))
}

// DON'T USE! NOT IMPLEMENTED
// defer a call to this method to recover from panics and have the panic logged
func (b *Brake) Recover() {
	//++
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

// DON'T USE! NOT IMPLEMENTED
// error with data
func (b *Brake) ErrorData(tipe string, msg string, data ...Var) {
	//++
}

// Var describes a simple key-value item
type Var struct {
	Key   string `xml:"key,attr"`
	Value string `xml:""`
}

// better alternative to `type Var struct` and `[]Var`
type Vars map[string]string

const noticeVersion = "2.3"

type notice struct {
	XMLName interface{} `xml:"notice"`
	// Required. The version of the API being used. Should be set to "2.3"
	Version string `xml:"version,attr"`
	// Required. The API key for the project that this error belongs to.
	// The API key can be found by viewing the edit project form on the Airbrake site.
	APIKey string `xml:"api-key"`
	// Notifier (client)
	Notifier *notifier `xml:"notifier"`
	// Environment (where did this happen?)
	Environment *environment `xml:"server-environment"`
	// Error
	Error *airError `xml:"error"`
	// Backtrace (stack)
	Backtrace *backtrace `xml:"backtrace"`
	// Request (probably http stuff)
	Request *request `xml:"request"`
}

// Notifier describes the application or library that sends an error to airbrake
type notifier struct {
	// Required. The name of the notifier client submitting the request, such as "hoptoad4j" or "rack-hoptoad."
	Name string `xml:"name"`
	// Required. The version number of the notifier client submitting the request.
	Version string `xml:"version"`
	// Required. A URL at which more information can be obtained concerning the notifier client.
	URL string `xml:"url"`
}

// DefaultNotifier holds information about this go package
// these values are exported so you can override them if you like
var Notifier = &notifier{
	Name:    "go.airbrake",
	Version: "0.1",
	URL:     "https://github.com/GeertJohan/go.airbake",
}

type environment struct {
	// Optional. The path to the project in which the error occurred, such as RAILS_ROOT or DOCUMENT_ROOT.
	//++ TODO: find out using go/build ?
	Root string `xml:"project-root,omitempty"`
	// Required. The name of the server environment in which the error occurred, such as "staging" or "production".
	Name string `xml:"environment-name"`
	// Optional. The version of the application that this error came from. If the App Version is set on the project, then errors older than the project's app version will be ignored. This version field uses Semantic Versioning style versioning.
	Version string `xml:"app-version,omitempty"`
}

// airError contains the error information
type airError struct {
	// Required. The class name or type of error that occurred.
	Type string `xml:"class"`
	// Optional. A short message describing the error that occurred.
	Message string `xml:"message,omitempty"`
}

// /notice/error/backtrace/line
// Required. This element can occur more than once.
// Each line element describes one code location or frame in the backtrace when the error occurred,
// and requires @file and @number attributes.
// If the location includes a method or function, the @method attribute should be used.
type backtrace struct {
	Lines []line `xml:"line"`
}

type line struct {
	File   string `xml:"file,attr"`
	Number string `xml:"number,attr"`
	Method string `xml:"method,attr,omitempty"`
}

// Optional. If this error occurred during an HTTP request,
// the children of this element can be used to describe the request that caused the error.
type request struct {
	// Required. The URL at which the error occurred.
	URL string `xml:"url"`
	// Required. The component in which the error occurred.
	// Otherwise, this can be set to a route or other request category.
	//++ TODO: This should probably be the name of the handler, lets see if we can figure that out
	Compontent string `xml:"component"`
	// Optional. The action in which the error occurred.
	// If each request is routed to a controller action, this should be set here.
	// Otherwise, this can be set to a method or other request subcategory.
	Action string `xml:"action"`
	// Optional. A list of var elements describing request parameters from the query string, POST body, routing, and other inputs.
	Params []Var `xml:"params>var"`
	// Optional. A list of var elements describing session variables from the request. See the section on var elements below.
	Session []Var `xml:"session>var"`
}
