package pg

import "github.com/yz89122/pgorm/v12/internal/pool"

func (db *DB) Pool() pool.Pooler {

	return db.pool
}

func (ln *Listener) CurrentConn() *pool.Conn {

	return ln.cn
}
