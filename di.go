package di

import (
	"errors"
	"fmt"
)

type (
	Container struct {
		entities map[string]entity
		cleanup  []func() error
		errs     []error
	}
	entity interface {
		Setup() (func() error, error)
	}
)

func New() *Container {
	return &Container{
		entities: make(map[string]entity),
		errs:     make([]error, 0),
		cleanup:  make([]func() error, 0),
	}
}

// Cleanup will cleanup entities in opposite order as it was setuped.
func (c *Container) Cleanup() error {
	for i := len(c.cleanup) - 1; i >= 0; i-- {
		if err := c.cleanup[i](); err != nil {
			c.errs = append(c.errs, err)
		}
	}

	return errors.Join(c.errs...)
}

type entityImpl[T any] struct {
	setup   func() (T, error)
	cleanup func(T) error

	noReuse bool
	val     T
}

func (e *entityImpl[T]) Setup() (func() error, error) {
	cleanup := func() error { return nil }

	if e.setup == nil {
		return cleanup, nil
	}

	val, err := e.setup()
	if err != nil {
		return cleanup, err
	}

	e.val = val

	if !e.noReuse {
		e.setup = nil
	}

	if e.cleanup != nil {
		cleanup = func() error { return e.cleanup(val) }
	}

	return cleanup, nil
}

func empty[T any]() (t T) { return }

func genName[T any](name string) string {
	entityName := fmt.Sprintf("%T", empty[T]())
	if entityName == "<nil>" {
		entityName = fmt.Sprintf("%T", new(T))
	}

	return fmt.Sprintf("%s<%s>", name, entityName)
}

func Set[T any](c *Container, opts ...func(*entityImpl[T])) {
	SetNamed(c, "", opts...)
}

func SetNamed[T any](c *Container, name string, opts ...func(*entityImpl[T])) {
	entityName := genName[*entityImpl[T]](name)
	entity, ok := c.entities[entityName].(*entityImpl[T])
	if !ok {
		entity = new(entityImpl[T])
	}

	for _, opt := range opts {
		opt(entity)
	}

	c.entities[entityName] = entity
}

func Get[T any](c *Container) T {
	return GetNamed[T](c, "")
}

func GetNamed[T any](c *Container, name string) T {
	entityName := genName[*entityImpl[T]](name)
	entity, ok := c.entities[entityName]
	if !ok {
		err := fmt.Errorf("dependency not found: %s", entityName)
		c.errs = append(c.errs, err)
		panic(err.Error())
	}

	cleanup, err := entity.Setup()
	if err != nil {
		err := fmt.Errorf("setup dependency %s: %w", entity, err)
		c.errs = append(c.errs, err)
		panic(err.Error())
	}

	c.cleanup = append(c.cleanup, cleanup)

	return entity.(*entityImpl[T]).val
}

func OptSetup[T any](f func() (T, error)) func(*entityImpl[T]) {
	return func(s *entityImpl[T]) { s.setup = f }
}

func OptNoReuse[T any]() func(*entityImpl[T]) {
	return func(s *entityImpl[T]) { s.noReuse = true }
}

func OptMiddleware[T any](f func(T) (T, error)) func(*entityImpl[T]) {
	return func(s *entityImpl[T]) {
		setup := s.setup
		s.setup = func() (T, error) {
			val, err := setup()
			if err != nil {
				return empty[T](), err
			}

			return f(val)
		}
	}
}

func OptCleanup[T any](f func(T) error) func(*entityImpl[T]) {
	return func(s *entityImpl[T]) { s.cleanup = f }
}
