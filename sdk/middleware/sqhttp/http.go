// Copyright (c) 2016 - 2019 Sqreen. All Rights Reserved.
// Please refer to our terms for more information:
// https://www.sqreen.io/terms.html

package sqhttp

import (
	"net/http"

	"github.com/sqreen/go-agent/agent/sqlib/sqhook"
	"github.com/sqreen/go-agent/sdk"
	"golang.org/x/xerrors"
)

// Middleware is Sqreen's middleware function for `net/http` to monitor and
// protect received requests. In protection mode, it can block and redirect
// requests according to its IP address or identified user using `Identify()`
// and `MatchSecurityResponse()` methods during from the request handler.
//
// SDK methods can be called from request handlers by using the request event
// record. It can be accessed using `sdk.FromContext()` on a request context.
// The middleware function stores it into the request context.
//
// Usage example:
//
//	fn := func(w http.ResponseWriter, r *http.Request) {
//		// Get the request record.
//		sqreen := sdk.FromContext(r.Context())
//
//		// Example of sending a custom event.
//		sqreen.TrackEvent("my.event")
//
//		// Example of globally identifying a user and checking if the request
//		// should be aborted.
//		uid := sdk.EventUserIdentifiersMap{"uid": "my-uid"}
//		sqUser := sqreen.ForUser(uid)
//		sqUser.Identify() // Globally associate this user to the current request
//		if match, _ := sqUser.MatchSecurityResponse(); match {
//			// Return to stop further handling the request and let Sqreen's
//			// middleware apply and abort the request.
//			return
//		}
//		// Not blocked.
//		fmt.Fprintf(w, "OK")
//	}
//	http.Handle("/foo", sqhttp.Middleware(http.HandlerFunc(fn)))
//
func Middleware(next http.Handler) http.Handler {
	// Simply adapt http.Handler to Handler in order to call MiddlewareWithError
	// to get the middleware function.
	m := MiddlewareWithError(HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		next.ServeHTTP(w, r)
		return nil
	}))
	// And now return a function adapting Handler to http.Handler
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = m.ServeHTTP(w, r)
	})
}

// MiddlewareWithError is a helper middleware to define other middlewares for
// other frameworks thanks to the error returned by the handlers in order
// to know if a request is being aborted.
func MiddlewareWithError(next Handler) Handler {
	// TODO: move this middleware function into the agent internal package (which
	//  needs restructuring the SDK)
	return HandlerFunc(func(w http.ResponseWriter, r *http.Request) (err error) {
		if err := addSecurityHeaders(w); err != nil {
			return err
		}
		// Create a new sqreen request wrapper.
		req := sdk.NewHTTPRequest(r)
		defer req.Close()
		// Use the newly created request compliant with `sdk.FromContext()`.
		r = req.Request()
		// Check if an early security action is already required such as based on
		// the request IP address.
		if handler := req.SecurityResponse(); handler != nil {
			handler.ServeHTTP(w, r)
			return AbortRequestError{}
		}
		// Call next handler.
		err = next.ServeHTTP(w, r)
		// If the returned error is not nil nor a security response, return it now.
		var secResponse sdk.SecurityResponseMatch
		if err != nil && !xerrors.As(err, &secResponse) {
			return err
		}
		// Otherwise check if a security response should be applied now, after
		// having used `Identify()` and `MatchSecurityResponse()`.
		if handler := req.UserSecurityResponse(); handler != nil {
			handler.ServeHTTP(w, r)
			return AbortRequestError{}
		}
		return nil
	})
}

// Handler is equivalent to http.Handler but returns an error when the request
// should no longer be handled.
type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request) error
}

// HandlerFunc is equivalent to http.HandlerFunc but returns an error when the
// request should no longer be handled.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// ServeHTTP calls f(w, r).
func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return f(w, r)
}

// AbortRequestError is returned by handlers when some security response was
// triggered and handled the response. The request handling should therefore
// stop.
type AbortRequestError struct {
	Message string
}

func (AbortRequestError) Error() string {
	return "request aborted"
}

// addSecurityHeaders is a mean to add a hook to the function closure returned
// by MiddlewareWithError() since it is not possible to get the symbol of
// function closures at compilation-time, so it is not possible to create a hook
// with the address of the function closure. The solution for this precise case
// where only a prolog is enough is therefore to simply define a function having
// a hook and called by the closure.
func addSecurityHeaders(w http.ResponseWriter) (err error) {
	{
		type Prolog = func(*sqhook.Context, *http.ResponseWriter) error
		type Epilog = func(*sqhook.Context, *error)
		ctx := sqhook.Context{}
		prolog, epilog := addSecurityHeaderHook.Callbacks()
		if epilog, ok := epilog.(Epilog); ok {
			defer epilog(&ctx, &err)
		}
		if prolog, ok := prolog.(Prolog); ok {
			if err := prolog(&ctx, &w); err != nil {
				return err
			}
		}
	}

	return nil
}

var addSecurityHeaderHook *sqhook.Hook

func init() {
	addSecurityHeaderHook = sqhook.New(addSecurityHeaders)
}
