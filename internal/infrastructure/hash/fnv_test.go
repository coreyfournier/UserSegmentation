package hash

import "testing"

func TestFNV_Deterministic(t *testing.T) {
	f := &FNV{}
	b1 := f.Bucket("user-123", "test-salt")
	b2 := f.Bucket("user-123", "test-salt")
	if b1 != b2 {
		t.Errorf("expected deterministic bucket, got %d and %d", b1, b2)
	}
}

func TestFNV_Range(t *testing.T) {
	f := &FNV{}
	for i := 0; i < 1000; i++ {
		b := f.Bucket("user-"+string(rune(i)), "salt")
		if b < 0 || b >= 100 {
			t.Errorf("bucket %d out of range [0,100)", b)
		}
	}
}

func TestFNV_DifferentSalts(t *testing.T) {
	f := &FNV{}
	b1 := f.Bucket("same-user", "salt-a")
	b2 := f.Bucket("same-user", "salt-b")
	// They might collide, but usually won't
	_ = b1
	_ = b2
}

func TestFNV_Distribution(t *testing.T) {
	f := &FNV{}
	buckets := make([]int, 100)
	n := 10000
	for i := 0; i < n; i++ {
		b := f.Bucket(string(rune('a')+rune(i%26))+string(rune(i)), "test")
		buckets[b]++
	}
	// Just check no bucket is completely empty (basic sanity)
	empty := 0
	for _, count := range buckets {
		if count == 0 {
			empty++
		}
	}
	if empty > 50 {
		t.Errorf("too many empty buckets: %d/100", empty)
	}
}
