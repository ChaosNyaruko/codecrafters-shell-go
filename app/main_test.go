package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getAllExecutables(t *testing.T) {
	exes := getAllExecutables()
	t.Logf("exec: %v", exes)
}

func Test_parseInput(t *testing.T) {
	line := "ls | wc -l"
	inputs, err := parseInput(line)
	assert.Nil(t, err)
	t.Logf("line: %v, inputs: %+v", line, inputs)

	line = "ls|wc -l "
	inputs, err = parseInput(line)
	assert.Nil(t, nil)
	t.Logf("line: %v, inputs: %+v", line, inputs)

	line = "ls|head -5 | wc -l"
	inputs, err = parseInput(line)
	assert.Nil(t, nil)
	t.Logf("line: %v, inputs: %+v", line, inputs)
}
