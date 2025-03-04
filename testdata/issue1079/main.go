package main

import (
	pg "github.com/yz89122/pgorm/v12"
	"github.com/yz89122/pgorm/v12/orm"
)

type MyType struct {
	MyInfo [3]bool `pg:",array"`
}

func createSchema(db *pg.DB) error {
	for _, model := range []interface{}{(*MyType)(nil)} {
		err := db.Model(model).CreateTable(&orm.CreateTableOptions{
			Temp: true,
		})
		if err != nil {

			return err
		}
	}

	return nil
}

func main() {
	db := pg.Connect(&pg.Options{
		User: "postgres",
	})
	defer db.Close()

	err := createSchema(db)
	if err != nil {
		panic(err)
	}

	thing := &MyType{
		MyInfo: [3]bool{true, false, true},
	}
	_, err = db.Model(thing).Insert()
	if err != nil {
		panic(err)
	}

	thing2 := new(MyType)
	err = db.Model(thing2).Select()
	if err != nil {
		panic(err)
	}

	if thing2.MyInfo != thing.MyInfo {
		panic("not equal")
	}
}
