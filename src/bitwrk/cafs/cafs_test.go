package cafs

import (
	"fmt"
	"math/rand"
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
	f1.Dispose()
	s.DumpStatistics()
	f2 := addData(t, s, 350)
	f2.Dispose()
	s.DumpStatistics()
	f3 := addData(t, s, 250)
	f3.Dispose()
	s.DumpStatistics()
	f4 := addData(t, s, 450)
	f4.Dispose()
	s.DumpStatistics()
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
	if f, err := s.Get(&key); err != nil {
		t.Fatalf("f4 should be stored. err:%v", err)
	} else {
		f.Dispose()
	}
	key = f3.Key()
	if f, err := s.Get(&key); err != nil {
		t.Fatalf("f3 should not have been removed. err:%v", err)
	} else {
		f.Dispose()
	}

	s.DumpStatistics()

	// Now f3 is youngest, then f4 (f1 and f2 are gone)
	addData(t, s, 500).Dispose()

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
				t.Fatalf("Expected to recover from something other than: %v", v)
			}
		}()
		addData(t, s, 1010)
	}
}

func TestCompression(t *testing.T) {
	s := NewRamStorage(1000000)
	f1 := addData(t, s, 1000001)
	f1.Dispose()
}

func TestCompression2(t *testing.T) {
	s := NewRamStorage(1000000)
	temp := s.Create("Adding cyclic random data")
	defer temp.Dispose()
	cycle := 65536
	times := 24
	r := rand.New(rand.NewSource(0))
	data := make([]byte, cycle)
	for i := 0; i < cycle; i++ {
		data[i] = byte(r.Int())
	}
	t.Logf("data=%016x...", data[:8])
	for i := 0; i < times; i++ {
		if _, err := temp.Write(data); err != nil {
			t.Errorf("Error on Write: %v", err)
		}
	}
	if err := temp.Close(); err != nil {
		t.Errorf("Error on Close: %v", err)
	}

	f := temp.File()
	defer f.Dispose()
	w := f.Open()
	data2 := make([]byte, 1)
	for i := 0; i < times*cycle; i++ {
		if n, err := w.Read(data2); err != nil || n != 1 {
			t.Fatalf("Error on Read: %v (n=%d)", err, n)
		}
		if data2[0] != data[i%cycle] {
			t.Fatalf("Data read != data written on byte %d: %02x != %02x", i, data2[0], data[i%cycle])
		}
	}
}

func addData(t *testing.T, s FileStorage, size int) File {
	temp := s.Create(fmt.Sprintf("Adding %v bytes object", size))
	defer temp.Dispose()
	for size > 0 {
		if _, err := temp.Write([]byte{byte(size)}); err != nil {
			panic(err)
		}
		size--
	}
	if err := temp.Close(); err != nil {
		panic(err)
	}
	return temp.File()
}
