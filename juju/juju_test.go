package juju

import "testing"

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore(1, 1)
	limiter1 := store("1")
	if !limiter1.Allow() {
		t.Error("The limiter should allow the first call")
	}
	if limiter1.Allow() {
		t.Error("The limiter should block the second call")
	}
	if store("1").Allow() {
		t.Error("The limiter should block the third call")
	}
	if !store("2").Allow() {
		t.Error("The limiter should allow the fourth call because it requests a new limiter")
	}
}
