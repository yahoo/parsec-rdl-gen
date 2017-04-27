package main

import (
	"github.com/ardielle/ardielle-go/rdl"
	"testing"
	"io/ioutil"
	"os"
	"encoding/json"
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
	pathInfos := extractPathInfo(&schema)
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
