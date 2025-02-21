package di_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/irr123/di"
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

	c := di.New()
	// clean up stuff on the end
	defer func() {
		if err := c.Cleanup(); err != nil {
			panic(err)
		}
		fmt.Printf("the end")
	}()

	// registering components

	// same db connection with different params
	di.Set(c, di.OptSetup(func() (db, error) { // "default"
		db, err := initDBconn("master")
		fmt.Printf("setup %T-%v\n", db, db)
		return db, err
	}), di.OptCleanup(func(db db) error { // will be called during Cleanup() in proper order
		fmt.Printf("cleanup %T-%v\n", db, db)
		return nil
	}))
	di.SetNamed(c, "replica", di.OptSetup(func() (db, error) {
		db, err := initDBconn("replica")
		fmt.Printf("setup %T-%v\n", db, db)
		return db, err
	}), di.OptCleanup(func(db db) error {
		fmt.Printf("cleanup %T-%v\n", db, db)
		return nil
	}), di.OptNoReuse[db]()) // on each Get() di will return new instance and each instance will be cleanuped

	di.Set(c, di.OptSetup(func() (repo, error) {
		repo, err := createDBRepo(di.Get[db](c)) // "default" with master
		fmt.Printf("setup %T-%v\n", repo, repo)
		return repo, err
	})) // no need to cleanup
	di.SetNamed(c, "replica", di.OptSetup(func() (repo, error) {
		repo, err := createDBRepo(di.GetNamed[db](c, "replica"))
		fmt.Printf("setup %T-%v\n", repo, repo)
		return repo, err
	}))

	di.Set(c, di.OptSetup(func() (businessService1, error) {
		bs1, err := setupBusinessService1(
			di.Get[repo](c), // "default" master
			di.GetNamed[repo](c, "replica"),
		)
		fmt.Printf("setup %T-%v\n", bs1, bs1)
		return bs1, err
	}))
	di.Set(c, di.OptSetup(func() (businessService2, error) {
		bs2 := setupBusinessService2(di.GetNamed[repo](c, "replica"))
		fmt.Printf("setup %T-%v\n", bs2, bs2)
		return bs2, nil
	}))

	di.Set(c, di.OptSetup(func() (string, error) {
		unused := "unused thing"
		fmt.Printf("setup %T-%v\n", unused, unused)
		return unused, nil
	}), di.OptCleanup(func(s string) error {
		fmt.Printf("cleanup %T-%v\n", s, s)
		return nil
	}))

	di.Set(c, di.OptSetup(func() (httpServer, error) {
		srv := buildServer(
			di.GetNamed[db](c, "replica"), // thx God, not from master
			di.Get[businessService1](c),
			di.Get[businessService2](c),
		)
		fmt.Printf("setup %T-%v\n", srv, srv)
		return srv, nil
	}), di.OptCleanup(func(srv httpServer) error {
		fmt.Printf("cleanup %T-%v\n", srv, srv)
		return nil
	}))

	// You did initial configuration separately and on main.go wants to complete it
	di.Set(c, di.OptMiddleware(func(srv1 httpServer) (httpServer, error) {
		srv2 := fmt.Sprintf("%T-%v", srv1, srv1)
		fmt.Printf("middleware %v\n", srv2)
		return httpServer(srv2), nil
	}))

	// start initialize things here
	fmt.Printf("imagine %v.Serve() here\n", di.Get[httpServer](c))

	// Output:
	//setup di_test.db-replica
	//setup di_test.db-master
	//setup di_test.repo-master
	//setup di_test.db-replica
	//setup di_test.repo-replica
	//setup di_test.businessService1-master+replica
	//setup di_test.businessService2-replica
	//setup di_test.httpServer-srv
	//middleware di_test.httpServer-srv
	//imagine di_test.httpServer-srv.Serve() here
	//cleanup di_test.httpServer-di_test.httpServer-srv
	//cleanup di_test.db-replica
	//cleanup di_test.db-master
	//cleanup di_test.db-replica
	//the end
}

func TestSetupOnlyNeeded(*testing.T) {
	c := di.New()

	di.Set(c, di.OptSetup(func() (map[string]any, error) {
		return map[string]any{"a": 1}, nil
	}))
	di.Set(c, di.OptSetup(func() ([]int, error) {
		return []int{2}, nil
	}))
	di.Set(c, di.OptSetup(func() (string, error) {
		panic("no need to init 'string'")
	}))
	di.SetNamed(c, "main", di.OptSetup(func() (any, error) {
		return map[string]any{
			"b": di.Get[map[string]any](c),
			"c": di.Get[[]int](c),
		}, nil
	}))

	di.GetNamed[any](c, "main")
}

func TestReuse(t *testing.T) {
	c := di.New()
	count := new(int)

	di.Set(c, di.OptSetup(func() (*int, error) {
		*count++
		return count, nil
	}), di.OptNoReuse[*int]())

	for i := 1; i < 5; i++ {
		if val := di.Get[*int](c); *val != i {
			t.Errorf("Unexpected val: %v", *val)
		}
	}
}

func TestReuseWithMiddleware(t *testing.T) {
	c := di.New()
	count := new(int)

	di.Set(c, di.OptSetup(func() (*int, error) {
		*count++
		return count, nil
	}), di.OptMiddleware(func(i *int) (*int, error) {
		*i--
		return i, nil
	}), di.OptNoReuse[*int]())

	for i := 0; i < 5; i++ {
		if val := di.Get[*int](c); *val != 0 {
			t.Errorf("Unexpected val: %v", *val)
		}
	}
}

func TestCleanup(t *testing.T) {
	var (
		c    = di.New()
		err1 = errors.New("1")
		err2 = errors.New("2")
		err3 = errors.New("3")
	)

	di.Set(c, di.OptSetup(func() (int, error) {
		return 42, nil
	}), di.OptCleanup(func(int) error {
		return err1
	}))
	di.Set(c, di.OptSetup(func() (string, error) {
		return strconv.Itoa(di.Get[int](c)), nil
	}), di.OptCleanup(func(string) error {
		return err2
	}))
	di.SetNamed(c, "format", di.OptSetup(func() (string, error) {
		return "format: " + di.Get[string](c), nil
	}), di.OptCleanup(func(string) error {
		return err3
	}))

	result := di.GetNamed[string](c, "format")
	if result != "format: 42" {
		t.Errorf("Unexpected: %v", result)
	}

	err := c.Cleanup()
	if err == nil {
		t.Errorf("Cleanup should return error")
	}

	if err.Error() != "3\n2\n1" {
		t.Errorf("Unexpected: %v", err)
	}
}

func TestMultiCleanup(t *testing.T) {
	var (
		c    = di.New()
		err1 = errors.New("1")
	)

	di.Set(c, di.OptSetup(func() (int, error) {
		return 42, nil
	}), di.OptCleanup(func(int) error {
		return err1
	}))

	one := di.Get[int](c)
	if one != 42 {
		t.Errorf("Unexpected: %d", one)
	}

	two := di.Get[int](c)
	if two != 42 {
		t.Errorf("Unexpected: %d", two)
	}

	err := c.Cleanup()
	if err == nil {
		t.Errorf("Cleanup should return error")
	}

	if err.Error() != "1" {
		t.Errorf("Unexpected: %v", err)
	}
}
