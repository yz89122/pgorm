package pg_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	pg "github.com/yz89122/pgorm/v12"
	"github.com/yz89122/pgorm/v12/orm"
)

var (
	ctx  = context.Background()
	pgdb *pg.DB
)

func init() {
	pgdb = connect()

	err := pgdb.Model((*Video)(nil)).DropTable(&orm.DropTableOptions{
		IfExists: true,
		Cascade:  true,
	})
	panicIf(err)

	err = pgdb.Model((*Video)(nil)).CreateTable(&orm.CreateTableOptions{})
	panicIf(err)
}

func connect() *pg.DB {

	return pg.Connect(pgOptions())
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

func ExampleConnect() {
	db := pg.Connect(&pg.Options{
		User:      pgUser(),
		Password:  pgPassword(),
		Addr:      pgAddr(),
		TLSConfig: getTLSConfig(),
	})
	defer db.Close()

	var n int
	_, err := db.QueryOne(pg.Scan(&n), "SELECT 1")
	panicIf(err)
	fmt.Println(n)
	// Output: 1
}

func ExampleDB_QueryOne() {
	var user struct {
		Name string
	}

	res, err := pgdb.QueryOne(&user, `
        WITH users (name) AS (VALUES (?))
        SELECT * FROM users
    `, "admin")
	panicIf(err)
	fmt.Println(res.RowsAffected())
	fmt.Println(user)
	// Output: 1
	// {admin}
}

func ExampleDB_QueryOne_returning_id() {
	_, err := pgdb.Exec(`CREATE TEMP TABLE users(id serial, name varchar(500))`)
	panicIf(err)

	var user struct {
		Id   int32
		Name string
	}
	user.Name = "admin"

	_, err = pgdb.QueryOne(&user, `
        INSERT INTO users (name) VALUES (?name) RETURNING id
    `, &user)
	panicIf(err)
	fmt.Println(user)
	// Output: {1 admin}
}

func ExampleDB_Exec() {
	res, err := pgdb.Exec(`CREATE TEMP TABLE test()`)
	panicIf(err)
	fmt.Println(res.RowsAffected())
	// Output: -1
}

func ExampleListener() {
	ln := pgdb.Listen(ctx, "mychan")
	defer ln.Close()

	ch := ln.Channel()

	go func() {
		time.Sleep(time.Millisecond)
		_, err := pgdb.Exec("NOTIFY mychan, ?", "hello world")
		panicIf(err)
	}()

	notif := <-ch
	fmt.Println(notif)
	// Output: {mychan hello world}
}

func txExample() *pg.DB {
	db := pg.Connect(&pg.Options{
		User:      pgUser(),
		Password:  pgPassword(),
		Addr:      pgAddr(),
		TLSConfig: getTLSConfig(),
	})

	queries := []string{
		`DROP TABLE IF EXISTS tx_test`,
		`CREATE TABLE tx_test(counter int)`,
		`INSERT INTO tx_test (counter) VALUES (0)`,
	}
	for _, q := range queries {
		_, err := db.Exec(q)
		panicIf(err)
	}

	return db
}

func ExampleDB_Begin() {
	db := txExample()

	incrInTx := func(db *pg.DB) error {
		tx, err := db.Begin()
		if err != nil {

			return err
		}
		// Rollback tx on error.
		defer tx.Rollback()

		var counter int
		_, err = tx.QueryOne(
			pg.Scan(&counter), `SELECT counter FROM tx_test FOR UPDATE`)
		if err != nil {

			return err
		}

		counter++

		_, err = tx.Exec(`UPDATE tx_test SET counter = ?`, counter)
		if err != nil {

			return err
		}

		return tx.Commit()
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := incrInTx(db)
			panicIf(err)
		}()
	}
	wg.Wait()

	var counter int
	_, err := db.QueryOne(pg.Scan(&counter), `SELECT counter FROM tx_test`)
	panicIf(err)
	fmt.Println(counter)
	// Output: 10
}

func ExampleDB_RunInTransaction() {
	db := txExample()

	incrInTx := func(db *pg.DB) error {
		// Transaction is automatically rolled back on error.

		return db.RunInTransaction(ctx, func(tx *pg.Tx) error {
			var counter int
			_, err := tx.QueryOne(
				pg.Scan(&counter), `SELECT counter FROM tx_test FOR UPDATE`)
			if err != nil {

				return err
			}

			counter++

			_, err = tx.Exec(`UPDATE tx_test SET counter = ?`, counter)

			return err
		})
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := incrInTx(db)
			panicIf(err)
		}()
	}
	wg.Wait()

	var counter int
	_, err := db.QueryOne(pg.Scan(&counter), `SELECT counter FROM tx_test`)
	panicIf(err)
	fmt.Println(counter)
	// Output: 10
}

func ExampleDB_Prepare() {
	stmt, err := pgdb.Prepare(`SELECT $1::text, $2::text`)
	panicIf(err)

	var s1, s2 string
	_, err = stmt.QueryOne(pg.Scan(&s1, &s2), "foo", "bar")
	panicIf(err)
	fmt.Println(s1, s2)
	// Output: foo bar
}

func ExampleDB_Model_createTable() {
	type Model1 struct {
		Id int
	}

	type Model2 struct {
		Id   int
		Name string

		Model1Id int     `pg:"on_delete:RESTRICT,on_update: CASCADE"`
		Model1   *Model1 `pg:"rel:has-one"`
	}

	for _, model := range []interface{}{&Model1{}, &Model2{}} {
		err := pgdb.Model(model).CreateTable(&orm.CreateTableOptions{
			Temp:          true, // create temp table
			FKConstraints: true,
		})
		panicIf(err)
	}

	var info []struct {
		ColumnName string
		DataType   string
	}
	_, err := pgdb.Query(&info, `
		SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_name = 'model2'
	`)
	panicIf(err)
	fmt.Println(info)
	// Output: [{id bigint} {name text} {model1_id bigint}]
}

func ExampleInts() {
	var nums pg.Ints
	_, err := pgdb.Query(&nums, `SELECT generate_series(0, 10)`)
	panicIf(err)
	fmt.Println(nums)
	// Output: [0 1 2 3 4 5 6 7 8 9 10]
}

func ExampleStrings() {
	var strs pg.Strings
	_, err := pgdb.Query(&strs, `
		WITH users AS (VALUES ('foo'), ('bar')) SELECT * FROM users
	`)
	panicIf(err)
	fmt.Println(strs)
	// Output: [foo bar]
}

func ExampleDB_CopyFrom() {
	_, err := pgdb.Exec(`CREATE TEMP TABLE words(word text, len int)`)
	panicIf(err)

	r := strings.NewReader("hello,5\nfoo,3\n")
	_, err = pgdb.CopyFrom(r, `COPY words FROM STDIN WITH CSV`)
	panicIf(err)

	var buf bytes.Buffer
	_, err = pgdb.CopyTo(&buf, `COPY words TO STDOUT WITH CSV`)
	panicIf(err)
	fmt.Println(buf.String())
	// Output: hello,5
	// foo,3
}

func ExampleDB_WithTimeout() {
	var count int
	// Use bigger timeout since this query is known to be slow.
	_, err := pgdb.WithTimeout(time.Minute).QueryOne(pg.Scan(&count), `
		SELECT count(*) FROM big_table
	`)
	panicIf(err)
}

func ExampleScan() {
	var s1, s2 string
	_, err := pgdb.QueryOne(pg.Scan(&s1, &s2), `SELECT ?, ?`, "foo", "bar")
	panicIf(err)
	fmt.Println(s1, s2)
	// Output: foo bar
}

func ExampleError() {
	video := &Video{
		Id: 123,
	}
	_, err := pgdb.Model(video).Insert()
	panicIf(err)

	_, err = pgdb.Model(video).Insert()
	if err != nil {
		pgErr, ok := err.(pg.Error)
		if ok && pgErr.IntegrityViolation() {
			fmt.Println("video already exists:", err)
		} else if pgErr.Field('S') == "PANIC" {
			panic(err)
		}
	}
	// Output: video already exists: ERROR #23505 duplicate key value violates unique constraint "videos_pkey"
}
