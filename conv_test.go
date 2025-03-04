package pg_test

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	pg "github.com/yz89122/pgorm/v12"
	"github.com/yz89122/pgorm/v12/orm"
	"github.com/yz89122/pgorm/v12/pgjson"
	"github.com/yz89122/pgorm/v12/types"
)

type JSONMap map[string]interface{}

func (m *JSONMap) Scan(b interface{}) error {
	if b == nil {
		*m = nil

		return nil
	}

	return pgjson.Unmarshal(b.([]byte), m)
}

func (m JSONMap) Value() (driver.Value, error) {
	b, err := pgjson.Marshal(m)
	if err != nil {

		return nil, err
	}

	return string(b), nil
}

//------------------------------------------------------------------------------

type Valuer struct {
	v string
}

var _ driver.Valuer = (*Valuer)(nil)

func (v Valuer) Value() (driver.Value, error) {

	return v.v, nil
}

var _ sql.Scanner = (*Valuer)(nil)

func (v *Valuer) Scan(src interface{}) error {
	switch src := src.(type) {
	case nil:
		v.v = ""

		return nil
	case []byte:
		v.v = string(src)

		return nil
	default:

		return fmt.Errorf("unsupported type: %T", src)
	}
}

type Struct struct {
	Foo string
}

type conversionTest struct {
	i                int
	src, dst, wanted interface{}
	pgtype           string

	wanterr     string
	wantnil     bool
	wantzero    bool
	wantnothing bool
}

func unwrap(v interface{}) interface{} {
	if arr, ok := v.(*types.Array); ok {

		return arr.Value()
	}
	if hstore, ok := v.(*types.Hstore); ok {

		return hstore.Value()
	}

	return v
}

func deref(vi interface{}) interface{} {
	v := reflect.ValueOf(vi)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.IsValid() {

		return v.Interface()
	}

	return nil
}

func zero(v interface{}) interface{} {

	return reflect.Zero(reflect.ValueOf(v).Elem().Type()).Interface()
}

func (test *conversionTest) String() string {

	return fmt.Sprintf("#%d src=%#v dst=%#v", test.i, test.src, test.dst)
}

func (test *conversionTest) Assert(t *testing.T, err error) {
	if test.wanterr != "" {
		if err == nil || err.Error() != test.wanterr {
			t.Fatalf("got error %v, wanted %q (%s)", err, test.wanterr, test)
		}

		return
	}

	if err != nil {
		t.Fatalf("got error %q, wanted nil (%s)", err, test)
	}

	if test.wantnothing {

		return
	}

	dst := reflect.Indirect(reflect.ValueOf(unwrap(test.dst))).Interface()

	if test.wantnil {
		dstValue := reflect.ValueOf(dst)
		if !dstValue.IsValid() {

			return
		}
		if dstValue.IsNil() {

			return
		}
		t.Fatalf("got %#v, wanted nil (%s)", dst, test)

		return
	}

	// Remove any intermediate pointers to compare values.
	dst = deref(unwrap(dst))
	src := deref(unwrap(test.src))

	if test.wantzero {
		dstValue := reflect.ValueOf(dst)
		switch dstValue.Kind() {
		case reflect.Slice, reflect.Map:
			if dstValue.IsNil() {
				t.Fatalf("got nil, wanted zero value")
			}
			if dstValue.Len() != 0 {
				t.Fatalf("got %d items, wanted 0", dstValue.Len())
			}
		default:
			zero := zero(test.dst)
			if dst != zero {
				t.Fatalf("%#v != %#v (%s)", dst, zero, test)
			}
		}

		return
	}

	if dstTime, ok := dst.(time.Time); ok {
		srcTime := src.(time.Time)
		if dstTime.Unix() != srcTime.Unix() {
			t.Fatalf("%#v != %#v", dstTime, srcTime)
		}

		return
	}

	if dstTimes, ok := dst.([]time.Time); ok {
		srcTimes := src.([]time.Time)
		for i, dstTime := range dstTimes {
			srcTime := srcTimes[i]
			if dstTime.Unix() != srcTime.Unix() {
				t.Fatalf("%#v != %#v", dstTime, srcTime)
			}
		}

		return
	}

	wanted := test.wanted
	if wanted == nil {
		wanted = src
	}
	if !reflect.DeepEqual(dst, wanted) {
		t.Fatalf("%#v != %#v (%s)", dst, wanted, test)
	}
}

func conversionTests() []conversionTest {

	return []conversionTest{
		{src: nil, dst: nil, wanterr: "pg: Scan(nil)"},
		{src: nil, dst: new(uintptr), wanterr: "pg: Scan(unsupported uintptr)"},

		{src: nil, dst: true, pgtype: "bool", wanterr: "pg: Scan(non-pointer bool)"},
		{src: nil, dst: new(*bool), pgtype: "bool", wantnil: true},
		{src: nil, dst: new(bool), pgtype: "bool", wantzero: true},
		{src: true, dst: new(bool), pgtype: "bool"},
		{src: true, dst: new(*bool), pgtype: "bool"},
		{src: 1, dst: new(bool), wanted: true},

		{src: nil, dst: "", pgtype: "text", wanterr: "pg: Scan(non-pointer string)"},
		{src: nil, dst: new(string), pgtype: "text", wantzero: true},
		{src: nil, dst: new(*string), pgtype: "text", wantnil: true},
		{src: "hello world", dst: new(string), pgtype: "text"},
		{src: "hello world", dst: new(*string), pgtype: "text"},
		{src: "'\"\000\xa7", dst: new(string), wanted: `'"�`, pgtype: "text"},

		{src: nil, dst: []byte(nil), pgtype: "bytea", wanterr: "pg: Scan(non-pointer []uint8)"},
		{src: nil, dst: new([]byte), pgtype: "bytea", wantnil: true},
		{src: []byte("hello world\000"), dst: new([]byte), pgtype: "bytea"},
		{src: []byte{}, dst: new([]byte), pgtype: "bytea", wantzero: true},
		{src: [3]byte{}, dst: new([3]byte), pgtype: "bytea"},

		{src: nil, dst: int8(0), pgtype: "smallint", wanterr: "pg: Scan(non-pointer int8)"},
		{src: nil, dst: new(int8), pgtype: "smallint", wantzero: true},
		{src: int8(math.MaxInt8), dst: new(int8), pgtype: "smallint"},
		{src: int8(math.MaxInt8), dst: new(*int8), pgtype: "smallint"},
		{src: int8(math.MinInt8), dst: new(int8), pgtype: "smallint"},

		{src: nil, dst: int16(0), pgtype: "smallint", wanterr: "pg: Scan(non-pointer int16)"},
		{src: nil, dst: new(int16), pgtype: "smallint", wantzero: true},
		{src: int16(math.MaxInt16), dst: new(int16), pgtype: "smallint"},
		{src: int16(math.MaxInt16), dst: new(*int16), pgtype: "smallint"},
		{src: int16(math.MinInt16), dst: new(int16), pgtype: "smallint"},

		{src: nil, dst: int32(0), pgtype: "int", wanterr: "pg: Scan(non-pointer int32)"},
		{src: nil, dst: new(int32), pgtype: "int", wantzero: true},
		{src: int32(math.MaxInt32), dst: new(int32), pgtype: "int"},
		{src: int32(math.MaxInt32), dst: new(*int32), pgtype: "int"},
		{src: int32(math.MinInt32), dst: new(int32), pgtype: "int"},

		{src: nil, dst: int64(0), pgtype: "bigint", wanterr: "pg: Scan(non-pointer int64)"},
		{src: nil, dst: new(int64), pgtype: "bigint", wantzero: true},
		{src: int64(math.MaxInt64), dst: new(int64), pgtype: "bigint"},
		{src: int64(math.MaxInt64), dst: new(*int64), pgtype: "bigint"},
		{src: int64(math.MinInt64), dst: new(int64), pgtype: "bigint"},

		{src: nil, dst: 0, pgtype: "bigint", wanterr: "pg: Scan(non-pointer int)"},
		{src: nil, dst: new(int), pgtype: "bigint", wantzero: true},
		{src: int64(math.MaxInt64), dst: new(int64), pgtype: "bigint"},
		{src: int64(math.MaxInt64), dst: new(*int64), pgtype: "bigint"},
		{src: int64(math.MinInt32), dst: new(int64), pgtype: "bigint"},

		{src: nil, dst: uint8(0), pgtype: "smallint", wanterr: "pg: Scan(non-pointer uint8)"},
		{src: nil, dst: new(uint8), pgtype: "smallint", wantzero: true},
		{src: uint8(math.MaxUint8), dst: new(uint8), pgtype: "smallint"},
		{src: uint8(math.MaxUint8), dst: new(*uint8), pgtype: "smallint"},

		{src: nil, dst: uint16(0), pgtype: "smallint", wanterr: "pg: Scan(non-pointer uint16)"},
		{src: nil, dst: new(uint16), pgtype: "smallint", wantzero: true},
		{src: uint16(math.MaxUint16), dst: new(uint16), pgtype: "int"},
		{src: uint16(math.MaxUint16), dst: new(*uint16), pgtype: "int"},

		{src: nil, dst: uint32(0), pgtype: "bigint", wanterr: "pg: Scan(non-pointer uint32)"},
		{src: nil, dst: new(uint32), pgtype: "bigint", wantzero: true},
		{src: uint32(math.MaxUint32), dst: new(uint32), pgtype: "bigint"},
		{src: uint32(math.MaxUint32), dst: new(*uint32), pgtype: "bigint"},

		{src: nil, dst: uint64(0), pgtype: "bigint", wanterr: "pg: Scan(non-pointer uint64)"},
		{src: nil, dst: new(uint64), pgtype: "bigint", wantzero: true},
		{src: uint64(math.MaxUint64), dst: new(uint64)},
		{src: uint64(math.MaxUint64), dst: new(*uint64)},
		{src: uint64(math.MaxUint32), dst: new(uint64), pgtype: "bigint"},

		{src: nil, dst: uint(0), pgtype: "smallint", wanterr: "pg: Scan(non-pointer uint)"},
		{src: nil, dst: new(uint), pgtype: "bigint", wantzero: true},
		{src: uint64(math.MaxUint64), dst: new(uint64)},
		{src: uint64(math.MaxUint64), dst: new(*uint64)},
		{src: uint64(math.MaxUint32), dst: new(uint64), pgtype: "bigint"},

		{src: nil, dst: float32(0), pgtype: "decimal", wanterr: "pg: Scan(non-pointer float32)"},
		{src: nil, dst: new(float32), pgtype: "decimal", wantzero: true},
		{src: float32(math.MaxFloat32), dst: new(float32), pgtype: "decimal"},
		{src: float32(math.MaxFloat32), dst: new(*float32), pgtype: "decimal"},
		{src: float32(math.SmallestNonzeroFloat32), dst: new(float32), pgtype: "decimal"},

		{src: nil, dst: float64(0), pgtype: "decimal", wanterr: "pg: Scan(non-pointer float64)"},
		{src: nil, dst: new(float64), pgtype: "decimal", wantzero: true},
		{src: float64(math.MaxFloat64), dst: new(float64), pgtype: "decimal"},
		{src: float64(math.MaxFloat64), dst: new(*float64), pgtype: "decimal"},
		{src: float64(math.SmallestNonzeroFloat64), dst: new(float64), pgtype: "decimal"},

		{src: nil, dst: []int(nil), pgtype: "jsonb", wanterr: "pg: Scan(non-pointer []int)"},
		{src: nil, dst: new([]int), pgtype: "jsonb", wantnil: true},
		{src: []int(nil), dst: new([]int), pgtype: "jsonb", wantnil: true},
		{src: []int{}, dst: new([]int), pgtype: "jsonb", wantzero: true},
		{src: []int{1, 2, 3}, dst: new([]int), pgtype: "jsonb"},
		{src: [3]int{1, 2, 3}, dst: new([3]int), pgtype: "jsonb"},

		{src: nil, dst: pg.Array([]int(nil)), pgtype: "int[]", wanterr: "pg: Array(non-pointer []int)"},
		{src: pg.Array([]int(nil)), dst: pg.Array(new([]int)), pgtype: "int[]", wantnil: true},
		{src: pg.Array([]int{}), dst: pg.Array(new([]int)), pgtype: "int[]"},
		{src: pg.Array([]int{1, 2, 3}), dst: pg.Array(new([]int)), pgtype: "int[]"},
		{src: pg.Array(&[3]int{1, 2, 3}), dst: pg.Array(new([3]int)), pgtype: "int[]"},

		{src: nil, dst: pg.Array([]int64(nil)), pgtype: "bigint[]", wanterr: "pg: Array(non-pointer []int64)"},
		{src: nil, dst: pg.Array(new([]int64)), pgtype: "bigint[]", wantnil: true},
		{src: pg.Array([]int64(nil)), dst: pg.Array(new([]int64)), pgtype: "bigint[]", wantnil: true},
		{src: pg.Array([]int64{}), dst: pg.Array(new([]int64)), pgtype: "bigint[]"},
		{src: pg.Array([]int64{1, 2, 3}), dst: pg.Array(new([]int64)), pgtype: "bigint[]"},

		{src: nil, dst: pg.Array([]float64(nil)), pgtype: "decimal[]", wanterr: "pg: Array(non-pointer []float64)"},
		{src: nil, dst: pg.Array(new([]float64)), pgtype: "decimal[]", wantnil: true},
		{src: pg.Array([]float64(nil)), dst: pg.Array(new([]float64)), pgtype: "decimal[]", wantnil: true},
		{src: pg.Array([]float64{}), dst: pg.Array(new([]float64)), pgtype: "decimal[]"},
		{src: pg.Array([]float64{1.1, 2.22, 3.333}), dst: pg.Array(new([]float64)), pgtype: "decimal[]"},
		{src: pg.Array([]float64{math.NaN(), math.Inf(+1), math.Inf(-1)}), dst: pg.Array(new([]float64)), pgtype: "float[]", wantnothing: true},

		{src: nil, dst: pg.Array([]string(nil)), pgtype: "text[]", wanterr: "pg: Array(non-pointer []string)"},
		{src: nil, dst: pg.Array(new([]string)), pgtype: "text[]", wantnil: true},
		{src: pg.Array([]string(nil)), dst: pg.Array(new([]string)), pgtype: "text[]", wantnil: true},
		{src: pg.Array([]string{}), dst: pg.Array(new([]string)), pgtype: "text[]"},
		{src: pg.Array([]string{"one", "two", "three"}), dst: pg.Array(new([]string)), pgtype: "text[]"},
		{src: pg.Array([]string{`''"{}`}), dst: pg.Array(new([]string)), pgtype: "text[]"},

		{src: nil, dst: pg.Array([][]string(nil)), pgtype: "text[][]", wanterr: "pg: Array(non-pointer [][]string)"},
		{src: nil, dst: pg.Array(new([][]string)), pgtype: "text[][]", wantnil: true},
		{src: pg.Array([][]string(nil)), dst: pg.Array(new([]string)), pgtype: "text[][]", wantnil: true},
		{src: pg.Array([][]string{}), dst: pg.Array(new([][]string)), pgtype: "text[][]"},
		{src: pg.Array([][]string{{"one", "two"}, {"three", "four"}}), dst: pg.Array(new([][]string)), pgtype: "text[][]"},
		{src: pg.Array([][]string{{`'"\{}`}}), dst: pg.Array(new([][]string)), pgtype: "text[][]"},

		{src: pg.Array([][]byte{[]byte(`'"\{}`)}), dst: pg.Array(new([][]byte)), pgtype: "bytea[]"},

		{src: nil, dst: pg.Hstore(map[string]string(nil)), pgtype: "hstore", wanterr: "pg: Hstore(non-pointer map[string]string)"},
		{src: nil, dst: pg.Hstore(new(map[string]string)), pgtype: "hstore", wantnil: true},
		{src: pg.Hstore(map[string]string(nil)), dst: pg.Hstore(new(map[string]string)), pgtype: "hstore", wantnil: true},
		{src: pg.Hstore(map[string]string{}), dst: pg.Hstore(new(map[string]string)), pgtype: "hstore"},
		{src: pg.Hstore(map[string]string{"foo": "bar"}), dst: pg.Hstore(new(map[string]string)), pgtype: "hstore"},
		{src: pg.Hstore(map[string]string{`'"\{}=>`: `'"\{}=>`}), dst: pg.Hstore(new(map[string]string)), pgtype: "hstore"},

		{src: nil, dst: sql.NullBool{}, pgtype: "bool", wanterr: "pg: Scan(non-pointer sql.NullBool)"},
		{src: nil, dst: new(*sql.NullBool), pgtype: "bool", wantnil: true},
		{src: nil, dst: new(sql.NullBool), pgtype: "bool", wanted: sql.NullBool{}},
		{src: &sql.NullBool{}, dst: new(sql.NullBool), pgtype: "bool"},
		{src: &sql.NullBool{Valid: true}, dst: new(sql.NullBool), pgtype: "bool"},
		{src: &sql.NullBool{Valid: true, Bool: true}, dst: new(sql.NullBool), pgtype: "bool"},

		{src: &sql.NullString{}, dst: new(sql.NullString), pgtype: "text"},
		{src: &sql.NullString{Valid: true}, dst: new(sql.NullString), pgtype: "text"},
		{src: &sql.NullString{Valid: true, String: "foo"}, dst: new(sql.NullString), pgtype: "text"},

		{src: &sql.NullInt64{}, dst: new(sql.NullInt64), pgtype: "bigint"},
		{src: &sql.NullInt64{Valid: true}, dst: new(sql.NullInt64), pgtype: "bigint"},
		{src: &sql.NullInt64{Valid: true, Int64: math.MaxInt64}, dst: new(sql.NullInt64), pgtype: "bigint"},

		{src: &sql.NullFloat64{}, dst: new(sql.NullFloat64), pgtype: "decimal"},
		{src: &sql.NullFloat64{Valid: true}, dst: new(sql.NullFloat64), pgtype: "decimal"},
		{src: &sql.NullFloat64{Valid: true, Float64: math.MaxFloat64}, dst: new(sql.NullFloat64), pgtype: "decimal"},

		{src: nil, dst: customStrSlice{}, wanterr: "pg: Scan(non-pointer pg_test.customStrSlice)"},
		{src: nil, dst: new(customStrSlice), wantnil: true},
		{src: nil, dst: new(*customStrSlice), wantnil: true},
		{src: customStrSlice{}, dst: new(customStrSlice), wantzero: true},
		{src: customStrSlice{"one", "two"}, dst: new(customStrSlice)},

		{src: nil, dst: time.Time{}, wanterr: "pg: Scan(non-pointer time.Time)"},
		{src: nil, dst: new(time.Time), pgtype: "timestamptz", wantzero: true},
		{src: nil, dst: new(*time.Time), pgtype: "timestamptz", wantnil: true},
		{src: time.Now(), dst: new(time.Time), pgtype: "timestamptz"},
		{src: time.Now(), dst: new(*time.Time), pgtype: "timestamptz"},
		{src: time.Now().UTC(), dst: new(time.Time), pgtype: "timestamptz"},
		{src: time.Time{}, dst: new(time.Time), pgtype: "timestamptz"},

		{src: nil, dst: pg.Array([]time.Time(nil)), pgtype: "timestamptz[]", wanterr: "pg: Array(non-pointer []time.Time)"},
		{src: nil, dst: pg.Array(new([]time.Time)), pgtype: "timestamptz[]", wantnil: true},
		{src: pg.Array([]time.Time(nil)), dst: pg.Array(new([]time.Time)), pgtype: "timestamptz[]", wantnil: true},
		{src: pg.Array([]time.Time{}), dst: pg.Array(new([]time.Time)), pgtype: "timestamptz[]"},
		{src: pg.Array([]time.Time{time.Now(), time.Now(), time.Now()}), dst: pg.Array(new([]time.Time)), pgtype: "timestamptz[]"},

		{src: nil, dst: pg.Ints{}, wanterr: "pg: Scan(non-pointer pg.Ints)"},
		{src: 1, dst: new(pg.Ints), wanted: pg.Ints{1}},

		{src: nil, dst: pg.Strings{}, wanterr: "pg: Scan(non-pointer pg.Strings)"},
		{src: "hello", dst: new(pg.Strings), wanted: pg.Strings{"hello"}},

		{src: nil, dst: pg.IntSet{}, wanterr: "pg: Scan(non-pointer pg.IntSet)"},
		{src: 1, dst: new(pg.IntSet), wanted: pg.IntSet{1: struct{}{}}},

		{src: nil, dst: JSONMap{}, pgtype: "json", wanterr: "pg: Scan(non-pointer pg_test.JSONMap)"},
		{src: nil, dst: new(JSONMap), pgtype: "json", wantnil: true},
		{src: nil, dst: new(*JSONMap), pgtype: "json", wantnil: true},
		{src: JSONMap{}, dst: new(JSONMap), pgtype: "json"},
		{src: JSONMap{}, dst: new(*JSONMap), pgtype: "json"},
		{src: JSONMap{"foo": "bar"}, dst: new(JSONMap), pgtype: "json"},
		{src: `{"foo": "bar"}`, dst: new(JSONMap), pgtype: "json", wanted: JSONMap{"foo": "bar"}},

		{src: nil, dst: Struct{}, pgtype: "json", wanterr: "pg: Scan(non-pointer pg_test.Struct)"},
		{src: nil, dst: new(*Struct), pgtype: "json", wantnil: true},
		{src: nil, dst: new(Struct), pgtype: "json", wantzero: true},
		{src: Struct{}, dst: new(Struct), pgtype: "json"},
		{src: Struct{Foo: "bar"}, dst: new(Struct), pgtype: "json"},
		{src: `{"foo": "bar"}`, dst: new(Struct), wanted: Struct{Foo: "bar"}},

		{src: nil, dst: new(net.IP), wanted: net.IP(nil), pgtype: "inet"},
		{src: net.ParseIP("127.0.0.1"), dst: new(net.IP), pgtype: "inet"},
		{src: net.ParseIP("::10.2.3.4"), dst: new(net.IP), pgtype: "inet"},
		{src: net.ParseIP("::ffff:10.4.3.2"), dst: new(net.IP), pgtype: "inet"},

		{src: nil, dst: (*net.IPNet)(nil), pgtype: "cidr", wanterr: "pg: Scan(non-settable *net.IPNet)"},
		{src: nil, dst: new(net.IPNet), wanted: net.IPNet{}, pgtype: "cidr"},
		{src: nil, dst: mustParseCIDR("192.168.100.128/25"), wanted: net.IPNet{}, pgtype: "cidr"},
		{src: mustParseCIDR("192.168.100.128/25"), dst: new(net.IPNet), pgtype: "cidr"},
		{src: mustParseCIDR("2001:4f8:3:ba::/64"), dst: new(net.IPNet), pgtype: "cidr"},
		{src: mustParseCIDR("2001:4f8:3:ba:2e0:81ff:fe22:d1f1/128"), dst: new(net.IPNet), pgtype: "cidr"},

		{src: nil, dst: new(Valuer), wanted: Valuer{}},
		{src: (*Valuer)(nil), dst: new(Valuer), wanted: Valuer{}},
		{src: new(Valuer), dst: new(Valuer), wanted: Valuer{}},
		{src: Valuer{v: "hello"}, dst: new(Valuer)},
	}
}

func TestConversion(t *testing.T) {
	db := pg.Connect(pgOptions())
	defer db.Close()

	for i, test := range conversionTests() {
		test.i = i

		var scanner orm.ColumnScanner
		if v, ok := test.dst.(orm.ColumnScanner); ok {
			scanner = v
		} else {
			scanner = pg.Scan(test.dst)
		}

		_, err := db.QueryOne(scanner, "SELECT (?) AS dst", test.src)
		test.Assert(t, err)
	}

	for i, test := range conversionTests() {
		test.i = i

		var scanner orm.ColumnScanner
		if v, ok := test.dst.(orm.ColumnScanner); ok {
			scanner = v
		} else {
			scanner = pg.Scan(test.dst)
		}

		err := db.Model().ColumnExpr("(?) AS dst", test.src).Select(scanner)
		test.Assert(t, err)
	}

	for i, test := range conversionTests() {
		test.i = i

		if test.pgtype == "" {
			continue
		}

		stmt, err := db.Prepare(fmt.Sprintf("SELECT ($1::%s) AS dst", test.pgtype))
		if err != nil {
			t.Fatal(err)
		}

		var scanner orm.ColumnScanner
		if v, ok := test.dst.(orm.ColumnScanner); ok {
			scanner = v
		} else {
			scanner = pg.Scan(test.dst)
		}

		_, err = stmt.QueryOne(scanner, test.src)
		test.Assert(t, err)

		if err := stmt.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func mustParseCIDR(s string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}

	return ipnet
}

func TestReadColumnValue(t *testing.T) {
	db := pg.Connect(pgOptions())
	defer db.Close()

	type Test struct {
		pgtype  string
		value   interface{}
		isArray bool
	}

	tests := []Test{
		{pgtype: "boolean", value: true},

		{pgtype: "int2", value: int16(math.MaxInt16)},
		{pgtype: "int4", value: int32(math.MaxInt32)},
		{pgtype: "int8", value: int64(math.MaxInt64)},

		{pgtype: "float4", value: float32(1.2345)},
		{pgtype: "float8", value: float64(1.2345)},

		{pgtype: "decimal", value: types.RawValue{
			Type:  1700,
			Value: "111111111111111111111111111111111111111111111",
		}},
		{pgtype: "numeric", value: types.RawValue{
			Type:  1700,
			Value: "222222222222222222222222222222222222222222222",
		}},

		{pgtype: "text", value: "hello"},
		{pgtype: "varchar(1000)", value: "hello"},
		{pgtype: "bytea", value: []byte("hello")},
		{pgtype: "json", value: json.RawMessage("[]")},
		{pgtype: "jsonb", value: json.RawMessage("[]")},

		{pgtype: "int8[]", value: []int64{1, 2, 3}, isArray: true},
		{pgtype: "float8[]", value: []float64{1.23, 4.567}, isArray: true},
		{pgtype: "text[]", value: []string{"foo", "bar"}, isArray: true},

		{pgtype: "timestamptz", value: time.Unix(0, 0)},
		{pgtype: "timestamp", value: time.Unix(0, 0).UTC()},

		{pgtype: "uuid", value: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"},
	}

	for _, test := range tests {
		value := test.value
		if test.isArray {
			value = pg.Array(value)
		}

		var m map[string]interface{}
		err := db.Model().ColumnExpr("?::? AS col", value, pg.Safe(test.pgtype)).Select(&m)
		assert.Nil(t, err)
		assert.Equal(t, m["col"], test.value, test.pgtype)
	}

	var mm []map[string]interface{}
	err := db.Model().ColumnExpr("1").Union(
		db.Model().ColumnExpr("2"),
	).Select(&mm)
	assert.Nil(t, err)
	assert.Equal(t, []map[string]interface{}{
		{"?column?": int32(1)},
		{"?column?": int32(2)},
	}, mm)
}
