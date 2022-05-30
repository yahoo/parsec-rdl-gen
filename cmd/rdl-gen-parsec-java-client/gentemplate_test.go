// Copyright 2016 Yahoo Inc.
// Licensed under the terms of the Apache license. Please see LICENSE.md file distributed with this work for terms.

package main

import (
	"testing"
	"io/ioutil"
	"encoding/json"
	"github.com/ardielle/ardielle-go/rdl"
	"github.com/stretchr/testify/assert"
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
	gen := &javaClientGenerator{reg, &schema, cName, writer, nil, "test", "", "", false}
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
	gen := &javaClientGenerator{reg, &schema, cName, writer, nil, "test", "", "", false}
	gen.processTemplate(javaClientTemplate)
	writer.Flush()
	realClientImpl := buf.String()
	expectedSampleClientImpl, err := ioutil.ReadFile("../../testdata/SampleClientImpl.txt")
	if realClientImpl != string(expectedSampleClientImpl) {
		test.Errorf("sample client impl not generated as expected, real: \n%s\n, expected: \n%s\n",
			realClientImpl, expectedSampleClientImpl)
	}
}

func TestGenerateImpl2(test *testing.T) {
	data, err := ioutil.ReadFile("../../testdata/sample2.json")
	if err != nil {
		test.Error("can not read sample2 file ")
		os.Exit(1)
	}
	var schema rdl.Schema
	err = json.Unmarshal(data, &schema)
	if err != nil {
		test.Error("unmarshal sample2 data fail")
		os.Exit(1)
	}

	reg := rdl.NewTypeRegistry(&schema)
	cName := utils.Capitalize(string(schema.Name))

	buf := new (bytes.Buffer)
	writer := bufio.NewWriter(buf)
	gen := &javaClientGenerator{reg, &schema, cName, writer, nil, "test", "", "", false}
	gen.processTemplate(javaClientTemplate)
	writer.Flush()
	realClientImpl := buf.String()
	expectedSampleClientImpl, err := ioutil.ReadFile("../../testdata/Sample2ClientImpl.txt")
	if realClientImpl != string(expectedSampleClientImpl) {
		test.Errorf("sample2 client impl not generated as expected, real: \n%s\n, expected: \n%s\n",
			realClientImpl, expectedSampleClientImpl)
	}
}

func TestUriConstruct(test *testing.T) {
	gen := &javaClientGenerator{nil, nil, "", nil, nil, "test", "", "", false}
	inputs := []*rdl.ResourceInput{{Name: "id", PathParam: true}}
	r := &rdl.Resource{Inputs: inputs}
	realOut := gen.builderExt(r)
	expectedOut := `        xUriBuilder.resolveTemplate("id", id);
`
	if realOut != expectedOut {
		test.Errorf("uri builder not generate as expected: real: \n%s\n, expected: \n%s\n",
			realOut, expectedOut)
	}
}

func TestGenerateClientWithoutVersion(t *testing.T) {
	testOutputDir := getTempDir(t, ".", "testOutput-")
	path := testOutputDir + "/com/yahoo/shopping/parsec_generated/"

	//generate output result
	schema, err := rdl.ParseRDLFile("../../testdata/sampleWithoutVersion.rdl", false, false, false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	GenerateJavaClient("withoutVersion", schema, testOutputDir, string(schema.Namespace), "", false)

	//asserts
	clientContent := checkAndGetFileContent(t, path, "SampleClient.java")
	assert.Contains(t, string(clientContent), "CompletableFuture<User>")
	assert.Contains(t, string(clientContent), "postUser(User")

	clientImplContent := checkAndGetFileContent(t, path, "SampleClientImpl.java")
	assert.Contains(t, string(clientImplContent), "CompletableFuture<User>")
	assert.Contains(t, string(clientImplContent), "postUser(User")

	//clean up folder
	defer os.RemoveAll(testOutputDir)
}

func TestGenerateClientWithVersion(t *testing.T) {
	testOutputDir := getTempDir(t, ".", "testOutput-")
	path := testOutputDir + "/com/yahoo/shopping/parsec_generated/"

	//generate output result
	schema, err := rdl.ParseRDLFile("../../testdata/sampleWithVersion.rdl", false, false, false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	GenerateJavaClient("withVersion", schema, testOutputDir, string(schema.Namespace), "", false)

	//asserts
	clientContent := checkAndGetFileContent(t, path, "SampleClient.java")
	assert.Contains(t, string(clientContent), "CompletableFuture<UserV2>")
	assert.Contains(t, string(clientContent), "postUser(UserV2")

	clientImplContent := checkAndGetFileContent(t, path, "SampleClientImpl.java")
	assert.Contains(t, string(clientImplContent), "CompletableFuture<UserV2>")
	assert.Contains(t, string(clientImplContent), "postUser(UserV2")

	// clean up folder
	defer os.RemoveAll(testOutputDir)
}

func checkAndGetFileContent(t *testing.T, path string, fileName string) []byte {
	//1. check correspanding client file exists
	if _, err := os.Stat(path + fileName); err != nil {
		t.Fatalf("file not exists: %s", path+fileName)
	}
	//2. check file content
	content, err := os.ReadFile(path + fileName)
	if err != nil {
		t.Fatalf("can not read file: %s", path+fileName)
	}
	return content
}

func getTempDir(t *testing.T, dir string, prefix string) string {
	dir, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return dir
}
