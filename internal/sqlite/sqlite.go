// Package sqlite is a deliberately small, synchronous binding to the system
// SQLite library. It uses prepared statements only. Connections never escape
// this package; long-running network work must happen outside Transaction.
package sqlite

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
static int bind_text_copy(sqlite3_stmt *s, int i, const char *v, int n) {
  return sqlite3_bind_text(s, i, v, n, SQLITE_TRANSIENT);
}
*/
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

type Row map[string]any

func (r Row) String(k string) string { v, _ := r[k].(string); return v }
func (r Row) Int(k string) int64     { v, _ := r[k].(int64); return v }

type DB struct {
	mu     sync.Mutex
	conn   *C.sqlite3
	closed bool
}
type Tx struct{ db *DB }

func Open(path string) (*DB, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	d := &DB{}
	rc := C.sqlite3_open_v2(p, &d.conn, C.SQLITE_OPEN_READWRITE|C.SQLITE_OPEN_CREATE|C.SQLITE_OPEN_FULLMUTEX, nil)
	if rc != C.SQLITE_OK {
		e := d.err(rc)
		if d.conn != nil {
			C.sqlite3_close_v2(d.conn)
		}
		return nil, e
	}
	C.sqlite3_busy_timeout(d.conn, 5000)
		for _, s := range []string{"PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA foreign_keys=ON"} {
		if _, e := d.Exec(s); e != nil {
			d.Close()
			return nil, e
		}
	}
	return d, nil
}
func (d *DB) err(rc C.int) error {
	if d.conn == nil {
		return fmt.Errorf("sqlite error %d", int(rc))
	}
	return fmt.Errorf("sqlite %d: %s", int(rc), C.GoString(C.sqlite3_errmsg(d.conn)))
}
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	rc := C.sqlite3_close_v2(d.conn)
	if rc != C.SQLITE_OK {
		return d.err(rc)
	}
	d.closed = true
	return nil
}
func (d *DB) prepare(sql string, args []any) (*C.sqlite3_stmt, error) {
	if d.closed {
		return nil, errors.New("database is closed")
	}
	q := C.CString(sql)
	defer C.free(unsafe.Pointer(q))
	var st *C.sqlite3_stmt
	var tail *C.char
	rc := C.sqlite3_prepare_v2(d.conn, q, -1, &st, &tail)
	if rc != C.SQLITE_OK {
		return nil, d.err(rc)
	}
	if st == nil {
		return nil, errors.New("empty SQL statement")
	}
	if len(args) != int(C.sqlite3_bind_parameter_count(st)) {
		C.sqlite3_finalize(st)
		return nil, errors.New("SQL parameter count mismatch")
	}
	for i, a := range args {
		switch v := a.(type) {
		case nil:
			rc = C.sqlite3_bind_null(st, C.int(i+1))
		case string:
			p := C.CString(v)
			rc = C.bind_text_copy(st, C.int(i+1), p, C.int(len(v)))
			C.free(unsafe.Pointer(p))
		case []byte:
			p := C.CString(string(v))
			rc = C.bind_text_copy(st, C.int(i+1), p, C.int(len(v)))
			C.free(unsafe.Pointer(p))
		case int:
			rc = C.sqlite3_bind_int64(st, C.int(i+1), C.sqlite3_int64(v))
		case int64:
			rc = C.sqlite3_bind_int64(st, C.int(i+1), C.sqlite3_int64(v))
		case bool:
			n := 0
			if v {
				n = 1
			}
			rc = C.sqlite3_bind_int(st, C.int(i+1), C.int(n))
		case float64:
			rc = C.sqlite3_bind_double(st, C.int(i+1), C.double(v))
		default:
			C.sqlite3_finalize(st)
			return nil, fmt.Errorf("unsupported SQL argument %T", a)
		}
		if rc != C.SQLITE_OK {
			e := d.err(rc)
			C.sqlite3_finalize(st)
			return nil, e
		}
	}
	return st, nil
}
func (d *DB) exec(sql string, args ...any) (int64, error) {
	st, e := d.prepare(sql, args)
	if e != nil {
		return 0, e
	}
	defer C.sqlite3_finalize(st)
	for {
		rc := C.sqlite3_step(st)
		if rc == C.SQLITE_DONE {
			return int64(C.sqlite3_changes(d.conn)), nil
		}
		if rc != C.SQLITE_ROW {
			return 0, d.err(rc)
		}
	}
}
func (d *DB) query(sql string, args ...any) ([]Row, error) {
	st, e := d.prepare(sql, args)
	if e != nil {
		return nil, e
	}
	defer C.sqlite3_finalize(st)
	rows := []Row{}
	for {
		rc := C.sqlite3_step(st)
		if rc == C.SQLITE_DONE {
			return rows, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, d.err(rc)
		}
		r := Row{}
		for i := 0; i < int(C.sqlite3_column_count(st)); i++ {
			j := C.int(i)
			k := C.GoString(C.sqlite3_column_name(st, j))
			switch C.sqlite3_column_type(st, j) {
			case C.SQLITE_INTEGER:
				r[k] = int64(C.sqlite3_column_int64(st, j))
			case C.SQLITE_FLOAT:
				r[k] = float64(C.sqlite3_column_double(st, j))
			case C.SQLITE_TEXT:
				r[k] = C.GoStringN((*C.char)(unsafe.Pointer(C.sqlite3_column_text(st, j))), C.sqlite3_column_bytes(st, j))
			case C.SQLITE_NULL:
				r[k] = nil
			default:
				return nil, errors.New("unexpected BLOB column; store JSON as TEXT")
			}
		}
		rows = append(rows, r)
	}
}
func (d *DB) Exec(sql string, args ...any) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.exec(sql, args...)
}
func (d *DB) Query(sql string, args ...any) ([]Row, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.query(sql, args...)
}
func (d *DB) Transaction(fn func(*Tx) error) (err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err = d.exec("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			d.exec("ROLLBACK")
			panic(p)
		}
		if err != nil {
			d.exec("ROLLBACK")
		}
	}()
	if err = fn(&Tx{d}); err != nil {
		return err
	}
	_, err = d.exec("COMMIT")
	return err
}
func (t *Tx) Exec(sql string, args ...any) (int64, error)  { return t.db.exec(sql, args...) }
func (t *Tx) Query(sql string, args ...any) ([]Row, error) { return t.db.query(sql, args...) }
