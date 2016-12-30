// Copyright 2016 Yahoo Inc.
// Licensed under the terms of the Apache license. Please see LICENSE.md file distributed with this work for terms.

package main

import (
	"testing"
	"io/ioutil"
	"encoding/json"
	"github.com/ardielle/ardielle-go/rdl"
	"github.com/yahoo/parsec-rdl-gen/utils"
	"bufio"
	"bytes"
	"os"
)

func TestGenerateInterface(test *testing.T) {
	data, err := ioutil.ReadFile("../../testdata/sample.json")
	if err != nil {
		test.Error("can not read sample file ")
		os.Exit(1)
	}
	var schema rdl.Schema
	err = json.Unmarshal(data, &schema)
	if err != nil {
		test.Error("unmarshal sample data fail")
		os.Exit(1)
	}

	reg := rdl.NewTypeRegistry(&schema)
	cName := utils.Capitalize(string(schema.Name))

	buf := new (bytes.Buffer)
	writer := bufio.NewWriter(buf)
	gen := &javaClientGenerator{reg, &schema, cName, writer, nil, "test", "", ""}
	gen.processTemplate(javaClientInterfaceTemplate)
	writer.Flush()
	realClientInterface := buf.String()
	expectedSampleClientInterface, err := ioutil.ReadFile("../../testdata/SampleClient.txt")
	if realClientInterface != string(expectedSampleClientInterface) {
		test.Errorf("sample client interface not generated as expected, real: \n%s\n, expected: \n%s\n",
			realClientInterface, expectedSampleClientInterface)
	}
}

func TestGenerateImpl(test *testing.T) {
	data, err := ioutil.ReadFile("../../testdata/sample.json")
	if err != nil {
		test.Error("can not read sample file ")
		os.Exit(1)
	}
	var schema rdl.Schema
	err = json.Unmarshal(data, &schema)
	if err != nil {
		test.Error("unmarshal sample data fail")
		os.Exit(1)
	}

	reg := rdl.NewTypeRegistry(&schema)
	cName := utils.Capitalize(string(schema.Name))

	buf := new (bytes.Buffer)
	writer := bufio.NewWriter(buf)
	gen := &javaClientGenerator{reg, &schema, cName, writer, nil, "test", "", ""}
	gen.processTemplate(javaClientTemplate)
	writer.Flush()
	realClientImpl := buf.String()
	expectedSampleClientImpl, err := ioutil.ReadFile("../../testdata/SampleClientImpl.txt")
	if realClientImpl != string(expectedSampleClientImpl) {
		test.Errorf("sample client impl not generated as expected, real: \n%s\n, expected: \n%s\n",
			realClientImpl, expectedSampleClientImpl)
	}
}

func TestUriConstruct(test *testing.T) {
	gen := &javaClientGenerator{nil, nil, "", nil, nil, "test", "", ""}
	inputs := []*rdl.ResourceInput{{Name: "id", PathParam: true}}
	r := &rdl.Resource{Inputs: inputs}
	realOut := gen.builderExt(r)
	expectedOut := `
                            .resolveTemplate("id", id)
                            .build();`
	if realOut != expectedOut {
		test.Errorf("uri builder not generate as expected: real: \n%s\n, expected: \n%s\n",
			realOut, expectedOut)
	}
}