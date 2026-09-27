package sqlite

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	d, e := Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
func TestPreparedStatementsPreserveData(t *testing.T) {
	d := openTest(t)
	if _, e := d.Exec(`CREATE TABLE sample(id INTEGER PRIMARY KEY,body TEXT,n INTEGER)`); e != nil {
		t.Fatal(e)
	}
	text := "hello\x00'; DROP TABLE sample; -- 👋"
	if _, e := d.Exec(`INSERT INTO sample VALUES(?,?,?)`, 1, text, 7); e != nil {
		t.Fatal(e)
	}
	r, e := d.Query(`SELECT * FROM sample WHERE id=?`, 1)
	if e != nil || len(r) != 1 || r[0].String("body") != text || r[0].Int("n") != 7 {
		t.Fatalf("roundtrip: %+v %v", r, e)
	}
}
func TestTransactionRollbackAndPanic(t *testing.T) {
	d := openTest(t)
	d.Exec(`CREATE TABLE sample(n INTEGER)`)
	e := d.Transaction(func(tx *Tx) error { tx.Exec(`INSERT INTO sample VALUES(1)`); return errors.New("abort") })
	if e == nil {
		t.Fatal("expected rollback")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic not propagated")
			}
		}()
		d.Transaction(func(tx *Tx) error { tx.Exec(`INSERT INTO sample VALUES(2)`); panic("abort") })
	}()
	r, _ := d.Query(`SELECT COUNT(*) n FROM sample`)
	if r[0].Int("n") != 0 {
		t.Fatal("rollback leaked rows")
	}
	if e = d.Transaction(func(tx *Tx) error { _, e := tx.Exec(`INSERT INTO sample VALUES(3)`); return e }); e != nil {
		t.Fatal(e)
	}
}
func TestConcurrentWritesAndClose(t *testing.T) {
	d := openTest(t)
	d.Exec(`CREATE TABLE sample(n INTEGER)`)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, e := d.Exec(`INSERT INTO sample VALUES(?)`, i); e != nil {
				t.Error(e)
			}
		}(i)
	}
	wg.Wait()
	r, e := d.Query(`SELECT COUNT(*) n FROM sample`)
	if e != nil || r[0].Int("n") != 100 {
		t.Fatal(r, e)
	}
	d.Close()
	if _, e = d.Query(`SELECT 1`); e == nil {
		t.Fatal("closed DB accepted query")
	}
}
func TestBindingRejectsMismatchedArguments(t *testing.T) {
	d := openTest(t)
	for _, args := range [][]any{{}, {1, 2}, {struct{}{}}} {
		if _, e := d.Query(`SELECT ?`, args...); e == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
}
