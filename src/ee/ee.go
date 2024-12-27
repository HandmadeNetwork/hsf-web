package ee

import (
	"fmt"

	"github.com/go-stack/stack"
	"github.com/rs/zerolog"
)

/*
 * The `error` interface in Go is very simple:
 *
 *   Error() string
 *
 * This is often nowhere near enough information to reliably diagnose errors -
 * for example, it makes no provision for a stack trace. This package provides
 * an error type, ee.Error, that can both wrap another error and provide a
 * stack trace, so as to provide a clear chain of causation.
 *
 * The ee.Error type implements the Unwrap() interface introduced in Go 1.13,
 * allowing the use of errors.Is and errors.As.
 *
 * This error type is intended for "exceptions", not for simple error values
 * like io.EOF that do not merit a stack trace. There is no shame in using a
 * custom error type or value for errors you expect the programmer to handle in
 * some way.
 */

type Error struct {
	Message string
	Wrapped error
	Stack   CallStack
}

func New(wrapped error, format string, args ...interface{}) error {
	return &Error{
		Message: fmt.Sprintf(format, args...),
		Wrapped: wrapped,
		Stack:   Trace()[1:], // NOTE(asaf): Remove the call to New from the stack
	}
}

var _ error = &Error{}

func (e *Error) Error() string {
	if e.Wrapped == nil {
		return e.Message
	} else {
		return fmt.Sprintf("%s: %v", e.Message, e.Wrapped)
	}
}

func (e *Error) Unwrap() error {
	return e.Wrapped
}

type CallStack []StackFrame

func (s CallStack) MarshalZerologArray(a *zerolog.Array) {
	for _, frame := range s {
		a.Object(frame)
	}
}

type StackFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

func (f StackFrame) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("file", f.File).
		Int("line", f.Line).
		Str("function", f.Function)
}

var ZerologStackMarshaler = func(err error) interface{} {
	if asEE, ok := err.(*Error); ok {
		return asEE.Stack
	}
	// NOTE(asaf): If we got here, it means zerolog is trying to output a non-EE error.
	//			       We remove this call and the zerolog caller from the stack.
	return Trace()[2:]
}

func Trace() CallStack {
	trace := stack.Trace().TrimRuntime()[1:]
	frames := make(CallStack, len(trace))
	for i, call := range trace {
		callFrame := call.Frame()
		frames[i] = StackFrame{
			File:     callFrame.File,
			Line:     callFrame.Line,
			Function: callFrame.Function,
		}
	}

	return frames
}
