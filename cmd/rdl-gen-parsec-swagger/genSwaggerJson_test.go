// Copyright 2016 Yahoo Inc.
// Licensed under the terms of the Apache license. Please see LICENSE.md file distributed with this work for terms.

package main

import (
	"testing"
	"io/ioutil"
	"encoding/json"
	"github.com/ardielle/ardielle-go/rdl"
	"os"
)

func TestGenerateImpl(test *testing.T) {
	data, err := ioutil.ReadFile("../../testdata/rdl-gen-parsec-swagger/sample.json")
	checkErrInTest(err, "can not read sample file", test)

	var schema rdl.Schema
	err = json.Unmarshal(data, &schema)
	checkErrInTest(err, "unmarshal sample data fail", test)

	genParsecError := true
	swaggerData, err := swagger(&schema, genParsecError, "", "", "")
	checkErrInTest(err, "cannot generate swagger", test)
	j, err := json.MarshalIndent(swaggerData, "", "    ")
	checkErrInTest(err, "cannot marshal swagger", test)

	expectedSampleSwagger, err := ioutil.ReadFile("../../testdata/rdl-gen-parsec-swagger/swagger.json")
	checkErrInTest(err, "cannot read swagger json file", test)

	if (string(j) != string(expectedSampleSwagger)) {
		test.Errorf("sample swagger json not generated as expected, real: \n%s\n, expected: \n%s\n",
			string(j), string(expectedSampleSwagger))
	}
}

func checkErrInTest(err error, msg string, test *testing.T) {
	if err != nil {
		test.Error(msg)
		os.Exit(1)
	}
}
