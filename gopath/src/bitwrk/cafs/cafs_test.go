package cafs

import (
	"fmt"
	"testing"
)

func TestSimple(t *testing.T) {
	s := NewRamStorage(1000)
	_ = addData(t, s, 128)
}

func TestTwo(t *testing.T) {
	s := NewRamStorage(1000)
	f1 := addData(t, s, 128)
	f2 := addData(t, s, 256)
	if f1.Key() == f2.Key() {
		t.FailNow()
	}
}

func TestSame(t *testing.T) {
	s := NewRamStorage(1000)
	f1 := addData(t, s, 128)
	f2 := addData(t, s, 128)
	if f1.Key() != f2.Key() {
		t.FailNow()
	}
}

func TestLRU(t *testing.T) {
	s := NewRamStorage(1000)
	f1 := addData(t, s, 400)
	f2 := addData(t, s, 350)
	f3 := addData(t, s, 250)
	f4 := addData(t, s, 450)
	var key SKey
	key = f1.Key()
	if _, err := s.Get(&key); err != ErrNotFound {
		t.Fatalf("f1 should have been removed. err:%v", err)
	}
	key = f2.Key()
	if _, err := s.Get(&key); err != ErrNotFound {
		t.Fatalf("f2 should have been removed. err:%v", err)
	}
	key = f4.Key()
	if _, err := s.Get(&key); err != nil {
		t.Fatalf("f4 should be stored. err:%v", err)
	}
	key = f3.Key()
	if _, err := s.Get(&key); err != nil {
		t.Fatalf("f3 should not have been removed. err:%v", err)
	}

	// Now f3 is youngest, then f4 (f1 and f2 are gone)
	_ = addData(t, s, 500)

	key = f4.Key()
	if _, err := s.Get(&key); err != ErrNotFound {
		t.Fatalf("f4 should have been removed. err:%v", err)
	}
	key = f3.Key()
	if _, err := s.Get(&key); err != nil {
		t.Fatalf("f3 should be stored. err:%v", err)
	}

	{
		defer func() {
			if v := recover(); v == ErrNotEnoughSpace {
				t.Logf("Expectedly recovered from: %v", v)
			} else {
				t.Fatal("Expected to recover from something other than: %v", v)
			}
		}()
		addData(t, s, 1010)
	}
}

func addData(t *testing.T, s FileStorage, size int) File {
	temp := s.Create(fmt.Sprintf("Adding %v bytes object", size))
	defer temp.Dispose()
	for size > 0 {
		if _, err := temp.Write([]byte{0}); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		size--
	}
	if err := temp.Close(); err != nil {
		panic(err)
	}
	if file, err := temp.File(); err != nil {
		panic(err)
	} else {
		return file
	}
}
