package main

import "testing"

func Test_getAllExecutables(t *testing.T) {
	exes := getAllExecutables()
	t.Logf("exec: %v", exes)
}
