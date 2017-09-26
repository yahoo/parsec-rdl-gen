package main

import (
	"github.com/ardielle/ardielle-go/rdl"
	"fmt"
	"regexp"
	"encoding/json"
	"os"
	"flag"
	"io/ioutil"
	"net/url"
	"github.com/yahoo/parsec-rdl-gen/utils"
)

type uriInfo struct {
	Method string
	Path string
	PathRegex string
}

func main() {
	pOutdir := flag.String("o", ".", "Output directory")
	flag.String("s", "", "RDL source file")
	finalName := flag.String("f", "", "FinalName of jar package, will be a part of path in basePath")
	flag.Parse()
	var err error
	var data []byte
	if data, err = ioutil.ReadAll(os.Stdin); err == nil {
		err = genPathInfoFile(*pOutdir, data, *finalName);
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "*** %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func genPathInfoFile(outDir string, rdlJson []byte, finalName string) error {
	var (
		schema rdl.Schema
		pathInfoJson []byte
		err error
	)
	if err = json.Unmarshal(rdlJson, &schema); err != nil {
		return err
	}
	pathInfos := extractPathInfo(&schema, finalName)
	if pathInfoJson, err = json.Marshal(pathInfos); err != nil {
		return err
	}
	if err = os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	fileName := outDir + "/" + string(schema.Name) + ".json"
	if err = ioutil.WriteFile(fileName, pathInfoJson, 0644); err != nil {
		return err
	}
	return nil
}

func extractPathInfo(schema *rdl.Schema, finalName string) []uriInfo {
	rootPath := utils.JavaGenerationRootPath(schema)
	pathInfos := []uriInfo{}
	for _, resource := range schema.Resources {
		pathInfo := uriInfo{}
		pathInfo.Method = resource.Method
		pathInfo.Path = resource.Path
		pathInfo.PathRegex = "^" + finalName + rootPath + genUriRegex(pathInfo.Path, pathInfo.Method)
		pathInfos = append(pathInfos, pathInfo)
	}
	return pathInfos
}

var re = regexp.MustCompile("\\{[^}]+}")

func genUriRegex(path string, method string) string {
	u, _ := url.Parse(path)
	path = u.Path
	pathRegex := re.ReplaceAllString(path, `[^/]+`)
	if method == "GET" {
		pathRegex += `(/?\?|/?$)`
	} else {
		pathRegex += `/?$`
	}
	return pathRegex
}