package pg_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	pg "github.com/yz89122/pgorm/v12"
	"github.com/yz89122/pgorm/v12/orm"
	"github.com/yz89122/pgorm/v12/types"
)

func benchmarkDB() *pg.DB {

	return pg.Connect(&pg.Options{
		User:         pgUser(),
		Password:     pgPassword(),
		Database:     pgDatabase(),
		Addr:         pgAddr(),
		DialTimeout:  30 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		PoolSize:     10,
		PoolTimeout:  30 * time.Second,
	})
}

func BenchmarkQueryRowsGopgDiscard(b *testing.B) {
	seedDB()

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := db.Query(pg.Discard, `SELECT * FROM records LIMIT 100`)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkQueryRowsGopgOptimized(b *testing.B) {
	seedDB()

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var rs OptRecords
			_, err := db.Query(&rs, `SELECT * FROM records LIMIT 100`)
			if err != nil {
				b.Fatal(err)
			}
			if len(rs.C) != 100 {
				b.Fatalf("got %d, wanted 100", len(rs.C))
			}
		}
	})
}

func BenchmarkQueryRowsGopgReflect(b *testing.B) {
	seedDB()

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var rs []Record
			_, err := db.Query(&rs, `SELECT * FROM records LIMIT 100`)
			if err != nil {
				b.Fatal(err)
			}
			if len(rs) != 100 {
				b.Fatalf("got %d, wanted 100", len(rs))
			}
		}
	})
}

func BenchmarkQueryRowsGopgORM(b *testing.B) {
	seedDB()

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var rs []Record
			err := db.Model(&rs).Limit(100).Select()
			if err != nil {
				b.Fatal(err)
			}
			if len(rs) != 100 {
				b.Fatalf("got %d, wanted 100", len(rs))
			}
		}
	})
}

func BenchmarkModelHasOneGopg(b *testing.B) {
	seedDB()

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var books []Book
			err := db.Model(&books).Column("book.*").Relation("Author").Limit(100).Select()
			if err != nil {
				b.Fatal(err)
			}

			if len(books) != 100 {
				b.Fatalf("got %d, wanted 100", len(books))
			}
		}
	})
}

func BenchmarkModelHasManyGopg(b *testing.B) {
	seedDB()

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var books []Book
			err := db.Model(&books).Column("book.*").Relation("Translations").Limit(100).Select()
			if err != nil {
				b.Fatal(err)
			}

			if len(books) != 100 {
				b.Fatalf("got %d, wanted 100", len(books))
			}
			for _, book := range books {
				if len(book.Translations) != 10 {
					b.Fatalf("got %d, wanted 10", len(book.Translations))
				}
			}
		}
	})
}

func BenchmarkModelHasMany2ManyGopg(b *testing.B) {
	seedDB()

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var books []Book
			err := db.Model(&books).
				Column("book.*").Relation("Genres").
				Limit(100).
				Select()
			if err != nil {
				b.Fatal(err)
			}

			if len(books) != 100 {
				b.Fatalf("got %d, wanted 100", len(books))
			}
			for _, book := range books {
				if len(book.Genres) != 10 {
					b.Fatalf("got %d, wanted 10", len(book.Genres))
				}
			}
		}
	})
}

func BenchmarkQueryRow(b *testing.B) {
	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var dst numLoader
		_, err := db.QueryOne(&dst, `SELECT ?::bigint AS num`, 1)
		if err != nil {
			b.Fatal(err)
		}
		if dst.Num != 1 {
			b.Fatalf("got %d, wanted 1", dst.Num)
		}
	}
}

func BenchmarkQueryRowStmt(b *testing.B) {
	db := benchmarkDB()
	defer db.Close()

	stmt, err := db.Prepare(`SELECT $1::bigint AS num`)
	if err != nil {
		b.Fatal(err)
	}
	defer stmt.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var dst numLoader
		_, err := stmt.QueryOne(&dst, 1)
		if err != nil {
			b.Fatal(err)
		}
		if dst.Num != 1 {
			b.Fatalf("got %d, wanted 1", dst.Num)
		}
	}
}

func BenchmarkQueryRowScan(b *testing.B) {
	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var n int64
			_, err := db.QueryOne(pg.Scan(&n), `SELECT ? AS num`, 1)
			if err != nil {
				b.Fatal(err)
			}
			if n != 1 {
				b.Fatalf("got %d, wanted 1", n)
			}
		}
	})
}

func BenchmarkQueryRowStmtScan(b *testing.B) {
	db := benchmarkDB()
	defer db.Close()

	stmt, err := db.Prepare(`SELECT $1::bigint AS num`)
	if err != nil {
		b.Fatal(err)
	}
	defer stmt.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var n int64
		_, err := stmt.QueryOne(pg.Scan(&n), 1)
		if err != nil {
			b.Fatal(err)
		}
		if n != 1 {
			b.Fatalf("got %d, wanted 1", n)
		}
	}
}

func BenchmarkExec(b *testing.B) {
	db := benchmarkDB()
	defer db.Close()

	qs := []string{
		`DROP TABLE IF EXISTS exec_test`,
		`CREATE TABLE exec_test(id bigint, name varchar(500))`,
	}
	for _, q := range qs {
		_, err := db.Exec(q)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := db.Exec(`INSERT INTO exec_test (id, name) VALUES (?, ?)`, 1, "hello world")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkExecWithError(b *testing.B) {
	db := benchmarkDB()
	defer db.Close()

	qs := []string{
		`DROP TABLE IF EXISTS exec_with_error_test`,
		`CREATE TABLE exec_with_error_test(id bigint PRIMARY KEY, name varchar(500))`,
	}
	for _, q := range qs {
		_, err := db.Exec(q)
		if err != nil {
			b.Fatal(err)
		}
	}

	_, err := db.Exec(`
		INSERT INTO exec_with_error_test(id, name) VALUES(?, ?)
	`, 1, "hello world")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := db.Exec(`INSERT INTO exec_with_error_test(id) VALUES(?)`, 1)
			if err == nil {
				b.Fatalf("got nil error, expected integrity violation")
			} else if pgErr, ok := err.(pg.Error); !ok || !pgErr.IntegrityViolation() {
				b.Fatalf("got %s, expected integrity violation", err)
			}
		}
	})
}

func BenchmarkExecStmt(b *testing.B) {
	db := benchmarkDB()
	defer db.Close()

	_, err := db.Exec(`CREATE TEMP TABLE statement_exec(id bigint, name varchar(500))`)
	if err != nil {
		b.Fatal(err)
	}

	stmt, err := db.Prepare(`INSERT INTO statement_exec (id, name) VALUES ($1, $2)`)
	if err != nil {
		b.Fatal(err)
	}
	defer stmt.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := stmt.Exec(1, "hello world")
		if err != nil {
			b.Fatal(err)
		}
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

type Record struct {
	Num1, Num2, Num3 int64
	Str1, Str2, Str3 string
}

func (r *Record) GetNum1() int64 {

	return r.Num1
}

func (r *Record) GetNum2() int64 {

	return r.Num2
}

func (r *Record) GetNum3() int64 {

	return r.Num3
}

func (r *Record) GetStr1() string {

	return r.Str1
}

func (r *Record) GetStr2() string {

	return r.Str2
}

func (r *Record) GetStr3() string {

	return r.Str3
}

type OptRecord struct {
	Num1, Num2, Num3 int64
	Str1, Str2, Str3 string
}

var _ orm.ColumnScanner = (*OptRecord)(nil)

func (r *OptRecord) ScanColumn(col types.ColumnInfo, rd types.Reader, n int) error {
	tmp, err := rd.ReadFullTemp()
	if err != nil {

		return err
	}

	switch col.Name {
	case "num1":
		r.Num1, err = strconv.ParseInt(string(tmp), 10, 64)
	case "num2":
		r.Num2, err = strconv.ParseInt(string(tmp), 10, 64)
	case "num3":
		r.Num3, err = strconv.ParseInt(string(tmp), 10, 64)
	case "str1":
		r.Str1 = string(tmp)
	case "str2":
		r.Str2 = string(tmp)
	case "str3":
		r.Str3 = string(tmp)
	default:

		return fmt.Errorf("unknown column: %q", col.Name)
	}

	return err
}

type OptRecords struct {
	C []OptRecord
}

var _ orm.HooklessModel = (*OptRecords)(nil)

func (rs *OptRecords) Init() error {

	return nil
}

func (rs *OptRecords) NextColumnScanner() orm.ColumnScanner {
	rs.C = append(rs.C, OptRecord{})

	return &rs.C[len(rs.C)-1]
}

func (OptRecords) AddColumnScanner(_ orm.ColumnScanner) error {

	return nil
}

var seedDBOnce sync.Once

func seedDB() {
	seedDBOnce.Do(func() {
		if err := _seedDB(); err != nil {
			panic(err)
		}
	})
}

func _seedDB() error {
	db := benchmarkDB()
	defer db.Close()

	_, err := db.Exec(`DROP TABLE IF EXISTS records`)
	if err != nil {

		return err
	}

	_, err = db.Exec(`
		CREATE TABLE records(
			num1 serial,
			num2 serial,
			num3 serial,
			str1 text,
			str2 text,
			str3 text
		)
	`)
	if err != nil {

		return err
	}

	for i := 0; i < 1000; i++ {
		_, err = db.Exec(`
			INSERT INTO records (str1, str2, str3) VALUES (?, ?, ?)
		`, randSeq(100), randSeq(200), randSeq(300))
		if err != nil {

			return err
		}
	}

	err = createTestSchema(db)
	if err != nil {

		return err
	}

	for i := 1; i < 100; i++ {
		genre := &Genre{
			ID:   i,
			Name: fmt.Sprintf("genre %d", i),
		}
		_, err = db.Model(genre).Insert()
		if err != nil {

			return err
		}

		author := &Author{
			ID:   i,
			Name: fmt.Sprintf("author %d", i),
		}
		_, err = db.Model(author).Insert()
		if err != nil {

			return err
		}
	}

	for i := 1; i <= 1000; i++ {
		book := &Book{
			ID:        i,
			Title:     fmt.Sprintf("book %d", i),
			AuthorID:  rand.Intn(99) + 1,
			CreatedAt: time.Now(),
		}
		_, err = db.Model(book).Insert()
		if err != nil {

			return err
		}

		for j := 1; j <= 10; j++ {
			bookGenre := &BookGenre{
				BookID:  i,
				GenreID: j,
			}
			_, err = db.Model(bookGenre).Insert()
			if err != nil {

				return err
			}

			translation := &Translation{
				BookID: i,
				Lang:   fmt.Sprintf("%d", j),
			}
			_, err = db.Model(translation).Insert()
			if err != nil {

				return err
			}
		}
	}

	return nil
}

func BenchmarkForEachReal(b *testing.B) {
	const N = 100000

	type Model struct {
		Id int
		_  [1000]byte
	}

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var i int
		err := db.Model().
			TableExpr("generate_series(1, ?) as id", N).
			ForEach(func(m *Model) error {
				i++

				return nil
			})
		if err != nil {
			b.Fatal(err)
		}
		if i != N {
			b.Fatalf("got %d, wanted %d", i, N)
		}
	}
}

func BenchmarkForEachInMemory(b *testing.B) {
	const N = 100000

	type Model struct {
		Id int
		_  [1000]byte
	}

	db := benchmarkDB()
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var model []Model
		err := db.Model().
			TableExpr("generate_series(1, ?) as id", N).
			Select(&model)
		if err != nil {
			b.Fatal(err)
		}
		if len(model) != N {
			b.Fatalf("got %d, wanted %d", len(model), N)
		}
	}
}
