package main

import (
	"os"
	"testing"

	"github.com/ardielle/ardielle-go/rdl"
	"github.com/stretchr/testify/assert"
)

func TestGenerateServerWithoutVersion(t *testing.T) {
	testOutputDir := "."
	path := testOutputDir + "/com/yahoo/shopping/parsec_generated/"
	srcPath := testOutputDir + "/src/main/java/com/yahoo/shopping/"
	// delete the output folder before test
	os.RemoveAll(testOutputDir + "/com")
	os.RemoveAll(testOutputDir + "/src")

	//generate output result
	schema, err := rdl.ParseRDLFile("../../testdata/sampleWithoutVersion.rdl", false, false, false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	GenerateJavaServer("withoutVersion", schema, testOutputDir, true, true, true, true, string(schema.Namespace), false)

	//asserts
	resourcesContent := checkAndGetFileContent(t, path, "SampleResources.java")
	assert.Contains(t, string(resourcesContent), "class SampleResources")
	assert.Contains(t, string(resourcesContent), "@Path(\"/sample\")")
	assert.Contains(t, string(resourcesContent), "User user")

	handlerContent := checkAndGetFileContent(t, path, "SampleHandler.java")
	assert.Contains(t, string(handlerContent), "interface SampleHandler")
	assert.Contains(t, string(handlerContent), "public User postUsers(ResourceContext context, User user)")
	assert.Contains(t, string(handlerContent), "public User getUsersById(ResourceContext context, Integer id)")

	serverContent := checkAndGetFileContent(t, path, "SampleServer.java")
	assert.Contains(t, string(serverContent), "class SampleServer")
	assert.Contains(t, string(serverContent), "bind(handler).to(SampleHandler.class)")

	hImplContent := checkAndGetFileContent(t, srcPath, "SampleHandlerImpl.java")
	assert.Contains(t, string(hImplContent), "class SampleHandlerImpl")
	assert.Contains(t, string(hImplContent), "implements SampleHandler")
	assert.Contains(t, string(hImplContent), "public User postUsers(ResourceContext context, User user)")
	assert.Contains(t, string(hImplContent), "public User getUsersById(ResourceContext context, Integer id)")

	// clean up folder
	os.RemoveAll(testOutputDir + "/com")
	os.RemoveAll(testOutputDir + "/src")
}

func TestGenerateServerWithVersion(t *testing.T) {
	testOutputDir := "."
	path := testOutputDir + "/com/yahoo/shopping/parsec_generated/"
	srcPath := testOutputDir + "/src/main/java/com/yahoo/shopping/"
	// delete the output folder before test
	os.RemoveAll(testOutputDir + "/com")
	os.RemoveAll(testOutputDir + "/src")

	//generate output result
	schema, err := rdl.ParseRDLFile("../../testdata/sampleWithVersion.rdl", false, false, false)
	if err != nil {
		t.Fatalf("%v", err)
	}

	GenerateJavaServer("withVersion", schema, testOutputDir, true, true, true, true, string(schema.Namespace), false)

	//asserts
	resourcesContent := checkAndGetFileContent(t, path, "SampleV2Resources.java")
	assert.Contains(t, string(resourcesContent), "class SampleV2Resources")
	assert.Contains(t, string(resourcesContent), "@Path(\"/sample/v2\")")
	assert.Contains(t, string(resourcesContent), "UserV2 user")

	handlerContent := checkAndGetFileContent(t, path, "SampleV2Handler.java")
	assert.Contains(t, string(handlerContent), "interface SampleV2Handler")
	assert.Contains(t, string(handlerContent), "public UserV2 postUsers(ResourceContext context, UserV2 user)")
	assert.Contains(t, string(handlerContent), "public UserV2 getUsersById(ResourceContext context, Integer id)")

	serverContent := checkAndGetFileContent(t, path, "SampleV2Server.java")
	assert.Contains(t, string(serverContent), "class SampleV2Server")
	assert.Contains(t, string(serverContent), "bind(handler).to(SampleV2Handler.class)")

	hImplContent := checkAndGetFileContent(t, srcPath, "SampleV2HandlerImpl.java")
	assert.Contains(t, string(hImplContent), "class SampleV2HandlerImpl")
	assert.Contains(t, string(hImplContent), "implements SampleV2Handler")
	assert.Contains(t, string(hImplContent), "public UserV2 postUsers(ResourceContext context, UserV2 user)")
	assert.Contains(t, string(hImplContent), "public UserV2 getUsersById(ResourceContext context, Integer id)")

	// clean up folder
	os.RemoveAll(testOutputDir + "/com")
	os.RemoveAll(testOutputDir + "/src")
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
