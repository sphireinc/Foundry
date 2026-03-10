package diag

import (
	"errors"
	"fmt"
)

type Kind string

const (
	KindUnknown    Kind = "unknown"
	KindUsage      Kind = "usage"
	KindConfig     Kind = "config"
	KindPlugin     Kind = "plugin"
	KindIO         Kind = "io"
	KindDependency Kind = "dependency"
	KindBuild      Kind = "build"
	KindServe      Kind = "serve"
	KindRender     Kind = "render"
	KindInternal   Kind = "internal"
)

type Error struct {
	Kind Kind
	Op   string
	Err  error
}

func (e *Error) Error() string {
	switch {
	case e == nil:
		return ""
	case e.Op != "" && e.Err != nil:
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	case e.Op != "":
		return e.Op
	case e.Err != nil:
		return e.Err.Error()
	default:
		return string(e.Kind)
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(kind Kind, msg string) error {
	return &Error{
		Kind: kind,
		Op:   msg,
	}
}

func Wrap(kind Kind, op string, err error) error {
	if err == nil {
		return nil
	}

	var de *Error
	if errors.As(err, &de) {
		if de.Kind == KindUnknown && kind != KindUnknown {
			return &Error{
				Kind: kind,
				Op:   opOrFallback(op, de.Op),
				Err:  de.Err,
			}
		}
		if op != "" {
			return &Error{
				Kind: de.Kind,
				Op:   op,
				Err:  de.Err,
			}
		}
		return err
	}

	return &Error{
		Kind: kind,
		Op:   op,
		Err:  err,
	}
}

func KindOf(err error) Kind {
	var de *Error
	if errors.As(err, &de) && de.Kind != "" {
		return de.Kind
	}
	return KindUnknown
}

func Present(err error) string {
	if err == nil {
		return ""
	}

	kind := KindOf(err)
	switch kind {
	case KindUsage:
		return fmt.Sprintf("usage error: %v", err)
	case KindConfig:
		return fmt.Sprintf("config error: %v", err)
	case KindPlugin:
		return fmt.Sprintf("plugin error: %v", err)
	case KindIO:
		return fmt.Sprintf("io error: %v", err)
	case KindDependency:
		return fmt.Sprintf("dependency error: %v", err)
	case KindBuild:
		return fmt.Sprintf("build error: %v", err)
	case KindServe:
		return fmt.Sprintf("serve error: %v", err)
	case KindRender:
		return fmt.Sprintf("render error: %v", err)
	case KindInternal:
		return fmt.Sprintf("internal error: %v", err)
	default:
		return err.Error()
	}
}

func ExitCode(err error) int {
	switch KindOf(err) {
	case KindUsage:
		return 2
	default:
		return 1
	}
}

func opOrFallback(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
