package main

import (
	"testing"

	"github.com/ardielle/ardielle-go/rdl"
	"github.com/stretchr/testify/assert"
)

func TestGenerateWithoutVersion(t *testing.T) {
	schema, err := rdl.ParseRDLFile("../../testdata/sample.rdl", false, false, false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = GenerateJavaServer("withoutVersion", schema, ".", true, true, true, true, string(schema.Namespace), false)
	assert.Nil(t, err)
}
func TestGenerateWithVersion(t *testing.T) {
	schema, err := rdl.ParseRDLFile("../../testdata/sampleV2.rdl", false, false, false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = GenerateJavaServer("withVersion", schema, ".", true, true, true, true, string(schema.Namespace), false)
	assert.Nil(t, err)
}
