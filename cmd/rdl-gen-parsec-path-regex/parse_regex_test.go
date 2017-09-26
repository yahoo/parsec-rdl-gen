package main

import (
	"github.com/ardielle/ardielle-go/rdl"
	"testing"
	"io/ioutil"
	"os"
	"encoding/json"
	"github.com/yahoo/parsec-rdl-gen/utils"
)

func TestParseRegex(test *testing.T) {
	data, err := ioutil.ReadFile("../../testdata/rdl.json")
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
	pathInfos := extractPathInfo(&schema, "/api")
	pathInfoJson, err := json.Marshal(pathInfos)
	if err != nil {
		test.Errorf("marshal json error: %v", err)
		os.Exit(1)
	}
	expectedPathInfoJson, err := ioutil.ReadFile("../../testdata/expectedRdlPathInfo.json")
	if err != nil {
		test.Error("read expected data fail")
		os.Exit(1)
	}
	if string(pathInfoJson) != string(expectedPathInfoJson) {
		test.Error("result not as expected")
		os.Exit(1)
	}
}

func TestPathRegexGenerator(test *testing.T) {
	type pathInfos struct {
		method string
		path string
	}
	uriPaths := []pathInfos {
		{"POST", "/passcodes"},
		{"GET", "/passcodes"},
		{"GET", "/passcodes/{id}"},
		{"GET", "/passcodes/{id}/bbb/{id2}"},
		{"GET", "/transactions?offset={offset}&count={count}"},
	}
	expectedPathRegex := []string {
		`/passcodes/?$`,
		`/passcodes(/?\?|/?$)`,
		`/passcodes/[^/]+(/?\?|/?$)`,
		`/passcodes/[^/]+/bbb/[^/]+(/?\?|/?$)`,
		`/transactions(/?\?|/?$)`,
	}
	for idx, pathInfo := range uriPaths {
		if pathRegex := genUriRegex(pathInfo.path, pathInfo.method);pathRegex != expectedPathRegex[idx] {
			test.Errorf("pathRegex generated not as expected, path: %v, actulPathRegex: %v",
			pathInfo.path, pathRegex)
		}
	}
}

func TestBasePath(test *testing.T) {
	data, err := ioutil.ReadFile("../../testdata/rdl.json")
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
	expectedRootPath := "/mobilePayment/v1"
	rootPath := utils.JavaGenerationRootPath(&schema)
	if rootPath != expectedRootPath {
		test.Error("JavaGenerationRootPath not gen as expected, expected: %v, actual: %v",
			expectedRootPath, rootPath)
	}
}