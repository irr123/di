package di

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
)

func Example() {
	type (
		db               string
		repo             string
		businessService1 string
		businessService2 string
		httpServer       string
	)

	var (
		// just example entities
		initDBconn = func(name string) (db, error) {
			return db(name), nil
		}
		createDBRepo = func(db db) (repo, error) {
			return repo(db), nil
		}
		setupBusinessService1 = func(master, replica repo) (businessService1, error) {
			return businessService1(fmt.Sprintf("%s+%s", master, replica)), nil
		}
		setupBusinessService2 = func(repo repo) businessService2 {
			return businessService2(repo)
		}
		buildServer = func(
			db, // because you hacker and want to do raw query from handler
			businessService1,
			businessService2,
		) httpServer {
			return httpServer("srv")
		}
	)

	c := New()
	// clean up stuff on the end
	defer func() {
		if err := c.Cleanup(); err != nil {
			panic(err)
		}
		fmt.Printf("the end")
	}()

	// registering components

	// same db connection with different params
	Set(c, OptSetup(func() (db, error) { // "default"
		db, err := initDBconn("master")
		fmt.Printf("setup %T-%v\n", db, db)
		return db, err
	}), OptCleanup(func(db db) error { // will be called during Cleanup() in proper order
		fmt.Printf("cleanup %T-%v\n", db, db)
		return nil
	}))
	SetNamed(c, "replica", OptSetup(func() (db, error) {
		db, err := initDBconn("replica")
		fmt.Printf("setup %T-%v\n", db, db)
		return db, err
	}), OptCleanup(func(db db) error {
		fmt.Printf("cleanup %T-%v\n", db, db)
		return nil
	}), OptNoReuse[db]()) // on each Get() di will return new instance and each instance will be cleanuped

	Set(c, OptSetup(func() (repo, error) {
		repo, err := createDBRepo(Get[db](c)) // "default" with master
		fmt.Printf("setup %T-%v\n", repo, repo)
		return repo, err
	})) // no need to cleanup
	SetNamed(c, "replica", OptSetup(func() (repo, error) {
		repo, err := createDBRepo(GetNamed[db](c, "replica"))
		fmt.Printf("setup %T-%v\n", repo, repo)
		return repo, err
	}))

	Set(c, OptSetup(func() (businessService1, error) {
		bs1, err := setupBusinessService1(
			Get[repo](c), // "default" master
			GetNamed[repo](c, "replica"),
		)
		fmt.Printf("setup %T-%v\n", bs1, bs1)
		return bs1, err
	}))
	Set(c, OptSetup(func() (businessService2, error) {
		bs2 := setupBusinessService2(GetNamed[repo](c, "replica"))
		fmt.Printf("setup %T-%v\n", bs2, bs2)
		return bs2, nil
	}))

	Set(c, OptSetup(func() (string, error) {
		unused := "unused thing"
		fmt.Printf("setup %T-%v\n", unused, unused)
		return unused, nil
	}), OptCleanup(func(s string) error {
		fmt.Printf("cleanup %T-%v\n", s, s)
		return nil
	}))

	Set(c, OptSetup(func() (httpServer, error) {
		srv := buildServer(
			GetNamed[db](c, "replica"),
			Get[businessService1](c),
			Get[businessService2](c),
		)
		fmt.Printf("setup %T-%v\n", srv, srv)
		return srv, nil
	}), OptCleanup(func(srv httpServer) error {
		fmt.Printf("cleanup %T-%v\n", srv, srv)
		return nil
	}))

	// You did initial configuration separately and on main.go wants to complete it
	Set(c, OptMiddleware(func(srv1 httpServer) (httpServer, error) {
		srv2 := fmt.Sprintf("%T-%v", srv1, srv1)
		fmt.Printf("middleware %v\n", srv2)
		return httpServer(srv2), nil
	}))

	// start initialize things here
	fmt.Printf("imagine %v.Serve() here\n", Get[httpServer](c))

	// Output:
	//setup di.db-replica
	//setup di.db-master
	//setup di.repo-master
	//setup di.db-replica
	//setup di.repo-replica
	//setup di.businessService1-master+replica
	//setup di.businessService2-replica
	//setup di.httpServer-srv
	//middleware di.httpServer-srv
	//imagine di.httpServer-srv.Serve() here
	//cleanup di.httpServer-di.httpServer-srv
	//cleanup di.db-replica
	//cleanup di.db-master
	//cleanup di.db-replica
	//the end
}

func TestReleaseErrfmt(t *testing.T) {
	c := New()

	err := c.Cleanup()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	c.errs = append(c.errs, errors.New("1"))
	err = c.Cleanup()

	if err.Error() != "1" {
		t.Errorf("Unexpected error: %v", err)
	}

	c.errs = append(c.errs, errors.New("2"))
	err = c.Cleanup()

	if err.Error() != "1\n2" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestInitOnlyNeeded(*testing.T) {
	c := New()

	Set(c, OptSetup(func() (map[string]any, error) {
		return map[string]any{"a": 1}, nil
	}))
	Set(c, OptSetup(func() ([]int, error) {
		return []int{2}, nil
	}))
	Set(c, OptSetup(func() (string, error) {
		panic("no need to init 'string'")
	}))
	SetNamed(c, "main", OptSetup(func() (any, error) {
		return map[string]any{
			"b": Get[map[string]any](c),
			"c": Get[[]int](c),
		}, nil
	}))

	GetNamed[any](c, "main")
}

func TestReuse(t *testing.T) {
	c := New()
	count := new(int)

	Set(c, OptSetup(func() (*int, error) {
		*count++
		return count, nil
	}), OptNoReuse[*int]())

	for i := 1; i < 5; i++ {
		if val := Get[*int](c); *val != i {
			t.Errorf("Unexpected val: %v", *val)
		}
	}
}

func TestReuseWithMiddleware(t *testing.T) {
	c := New()
	count := new(int)

	Set(c, OptSetup(func() (*int, error) {
		*count++
		return count, nil
	}), OptMiddleware(func(i *int) (*int, error) {
		*i--
		return i, nil
	}), OptNoReuse[*int]())

	for i := 0; i < 5; i++ {
		if val := Get[*int](c); *val != 0 {
			t.Errorf("Unexpected val: %v", *val)
		}
	}
}

func TestDeinit(t *testing.T) {
	var (
		c    = New()
		err1 = errors.New("1")
		err2 = errors.New("2")
		err3 = errors.New("3")
	)

	Set(c, OptSetup(func() (int, error) {
		return 42, nil
	}), OptCleanup(func(int) error {
		return err1
	}))
	Set(c, OptSetup(func() (string, error) {
		return strconv.Itoa(Get[int](c)), nil
	}), OptCleanup(func(string) error {
		return err2
	}))
	SetNamed(c, "format", OptSetup(func() (string, error) {
		return "format: " + Get[string](c), nil
	}), OptCleanup(func(string) error {
		return err3
	}))

	result := GetNamed[string](c, "format")
	if result != "format: 42" {
		t.Errorf("Unexpected: %v", result)
	}

	err := c.Cleanup()
	if err == nil {
		t.Errorf("Release should return error")
	}

	if err.Error() != "3\n2\n1" {
		t.Errorf("Unexpected: %v", err)
	}
}

func TestMultiDeinit(t *testing.T) {
	var (
		c    = New()
		err1 = errors.New("1")
	)

	Set(c, OptSetup(func() (int, error) {
		return 42, nil
	}), OptCleanup(func(int) error {
		return err1
	}))

	one := Get[int](c)
	if one != 42 {
		t.Errorf("Unexpected: %d", one)
	}

	two := Get[int](c)
	if two != 42 {
		t.Errorf("Unexpected: %d", two)
	}

	err := c.Cleanup()
	if err == nil {
		t.Errorf("Release should return error")
	}

	if err.Error() != "1" {
		t.Errorf("Unexpected: %v", err)
	}
}
