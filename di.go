package di

import (
	"errors"
	"fmt"
)

type (
	Container struct {
		entities map[string]entity
		cleanup  []cleanup
		errs     []error
	}
	entity interface {
		setup() (cleanup, error)
	}
	cleanup func() error
)

func New() *Container {
	return &Container{
		entities: make(map[string]entity),
		errs:     make([]error, 0),
		cleanup:  make([]cleanup, 0),
	}
}

// Cleanup will deinitialize entities in opposite order as it was setuped.
func (c *Container) Cleanup() error {
	for i := len(c.cleanup) - 1; i >= 0; i-- {
		c.errs = append(c.errs, c.cleanup[i]())
	}

	return errors.Join(c.errs...)
}

type entityImpl[T any] struct {
	setupFn   func() (T, error)
	cleanupFn func(T) error

	noReuse bool
	val     T
}

func (e *entityImpl[T]) setup() (cleanup, error) {
	cleanup := func() error { return nil }

	if e.setupFn == nil {
		return cleanup, nil
	}

	val, err := e.setupFn()
	if err != nil {
		return cleanup, err
	}

	e.val = val

	if !e.noReuse {
		e.setupFn = nil
	}

	if e.cleanupFn != nil {
		cleanup = func() error { return e.cleanupFn(val) }
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

// Set entity into container
func Set[T any](c *Container, opts ...func(*entityImpl[T])) {
	SetNamed(c, "", opts...)
}

// SetNamed entity to manually resolve collisions
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

// Get entity from container
func Get[T any](c *Container) T {
	return GetNamed[T](c, "")
}

// GetNamed enntity to manually resolve collisions
func GetNamed[T any](c *Container, name string) T {
	entityName := genName[*entityImpl[T]](name)
	entity, ok := c.entities[entityName]
	if !ok {
		err := fmt.Errorf("dependency not found: %s", entityName)
		c.errs = append(c.errs, err)
		panic(err.Error())
	}

	cleanup, err := entity.setup()
	if err != nil {
		err := fmt.Errorf("setup dependency %s: %w", entity, err)
		c.errs = append(c.errs, err)
		panic(err.Error())
	}

	c.cleanup = append(c.cleanup, cleanup)

	return entity.(*entityImpl[T]).val
}

// OptSetup entity "constructor"
func OptSetup[T any](f func() (T, error)) func(*entityImpl[T]) {
	return func(s *entityImpl[T]) { s.setupFn = f }
}

// OptNoReuse will recreate entity on each call
func OptNoReuse[T any]() func(*entityImpl[T]) {
	return func(s *entityImpl[T]) { s.noReuse = true }
}

// OptMiddleware allows to provide additional configuration
// while entity already preserved in container
func OptMiddleware[T any](f func(T) (T, error)) func(*entityImpl[T]) {
	return func(s *entityImpl[T]) {
		setupFn := s.setupFn
		s.setupFn = func() (T, error) {
			val, err := setupFn()
			if err != nil {
				return empty[T](), err
			}

			return f(val)
		}
	}
}

// OptCleanup entity "destructor"
func OptCleanup[T any](f func(T) error) func(*entityImpl[T]) {
	return func(s *entityImpl[T]) { s.cleanupFn = f }
}
