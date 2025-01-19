package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type MyError struct{}

func (err *MyError) Error() string {
	return "I want to get off MR BONES WILD RIDE"
}

func TestMust(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		f := func() error { return nil }
		Must(f())
	})
	t.Run("non-nil error", func(t *testing.T) {
		f := func() error { return &MyError{} }
		assert.Panics(t, func() {
			Must(f())
		})
	})
	t.Run("nil *MyError", func(t *testing.T) {
		f := func() *MyError { return nil }
		Must(f())
	})
	t.Run("non-nil *MyError", func(t *testing.T) {
		f := func() *MyError { return &MyError{} }
		assert.Panics(t, func() {
			Must(f())
		})
	})
}

func TestMust1(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		f := func() (int, error) { return 0, nil }
		Must1(f())
	})
	t.Run("non-nil error", func(t *testing.T) {
		f := func() (int, error) { return 0, &MyError{} }
		assert.Panics(t, func() {
			Must1(f())
		})
	})
	t.Run("nil *MyError", func(t *testing.T) {
		f := func() (int, *MyError) { return 0, nil }
		Must1(f())
	})
	t.Run("non-nil *MyError", func(t *testing.T) {
		f := func() (int, *MyError) { return 0, &MyError{} }
		assert.Panics(t, func() {
			Must1(f())
		})
	})
}
