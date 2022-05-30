// Copyright 2016 Yahoo Inc.
// Licensed under the terms of the Apache license. Please see LICENSE.md file distributed with this work for terms.

package main

import (
	"github.com/ardielle/ardielle-go/rdl"
	"bufio"
	"strings"
	"log"
	"flag"
	"io/ioutil"
	"fmt"
	"encoding/json"
	"os"
	"github.com/yahoo/parsec-rdl-gen/utils"
	"text/template"
	"strconv"
)

type javaClientGenerator struct {
	registry   rdl.TypeRegistry
	schema     *rdl.Schema
	name       string
	writer     *bufio.Writer
	err        error
	banner     string
	ns         string
	base       string
	isPcSuffix bool
}

func main() {
	pOutdir := flag.String("o", ".", "Output directory")
	flag.String("s", "", "RDL source file")
	namespace := flag.String("ns", "", "Namespace")
	pc := flag.String("pc", "false", "add '_Pc' postfix to the generated java class")
	flag.Parse()

	isPcSuffix, err := strconv.ParseBool(*pc)
	checkErr(err)

	data, err := ioutil.ReadAll(os.Stdin)
	banner := "parsec-rdl-gen (development version)"

	if err == nil {
		var schema rdl.Schema
		err = json.Unmarshal(data, &schema)
		if err == nil {
			GenerateJavaClient(banner, &schema, *pOutdir, *namespace, "", isPcSuffix)
			os.Exit(0)
		}
	}
	fmt.Fprintf(os.Stderr, "*** %v\n", err)
	os.Exit(1)
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "*** %v\n", err)
		os.Exit(1)
	}
}

// GenerateJavaClient generates the client code to talk to the server
func GenerateJavaClient(banner string, schema *rdl.Schema, outdir string, ns string, base string, isPcSuffix bool) error {

	reg := rdl.NewTypeRegistry(schema)

	packageDir, err := utils.JavaGenerationDir(outdir, schema, ns)
	if err != nil {
		return err
	}

	cName := utils.Capitalize(string(schema.Name))

	out, file, _, err := utils.OutputWriter(packageDir, cName, "ClientImpl.java")
	if err != nil {
		return err
	}
	gen := &javaClientGenerator{reg, schema, cName, out, nil, banner, ns, base, isPcSuffix}
	gen.processTemplate(javaClientTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	out, file, _, err = utils.OutputWriter(packageDir, cName, "Client.java")
	if err != nil {
		return err
	}
	gen = &javaClientGenerator{reg, schema, cName, out, nil, banner, ns, base, isPcSuffix}
	gen.processTemplate(javaClientInterfaceTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	//ResourceException - the throawable wrapper for alternate return types
	out, file, _, err = utils.OutputWriter(packageDir, "ResourceException", ".java")
	if err != nil {
		return err
	}
	err = utils.JavaGenerateResourceException(schema, out, ns)
	out.Flush()
	file.Close()
	if err != nil {
		return err
	}

	//ResourceError - the default data object for an error
	out, file, _, err = utils.OutputWriter(packageDir, "ResourceError", ".java")
	if err != nil {
		return err
	}
	err = utils.JavaGenerateResourceError(schema, out, ns)
	out.Flush()
	file.Close()
	return err
}

func (gen *javaClientGenerator) processTemplate(templateSource string) error {
	commentFun := func(s string) string {
		return utils.FormatComment(s, 0, 80)
	}
	needExpectFunc := func(r *rdl.Resource) bool {
		if (r.Expected != "OK") || (len(r.Alternatives) > 0) {
			return true
		}
		return false
	}
	needImportHashSetFunc := func(rs []*rdl.Resource) bool {
		for _,r := range rs {
			if needExpectFunc(r) {
				return true
			}
		}
		return false
	}
	needBodyFunc := func(r *rdl.Resource) bool { return gen.needBody(r) }
	needImportJsonProcessingExceptionFunc := func(rs []*rdl.Resource) bool {
		for _,r := range rs {
			if (needBodyFunc(r)) {
				return true
			}
		}
		return false
	}
	funcMap := template.FuncMap{
		"header":      func() string { return utils.JavaGenerationHeader(gen.banner) },
		"package":     func() string { return utils.JavaGenerationPackage(gen.schema, gen.ns) },
		"comment":     commentFun,
		"methodSigWithHeader":
		               func(r *rdl.Resource) string { return "public "+ gen.clientMethodSignature(r, true) },
		"methodSig":   func(r *rdl.Resource) string { return "public "+ gen.clientMethodSignature(r, false) },
		"ContentOfNoHeaderMethod":
		               func(r *rdl.Resource) string { return gen.clientMethodOverloadContent(r) },
		"name":        func() string { return gen.name },
		"cName":       func() string { return utils.Capitalize(gen.name) },
		"lName":       func() string { return utils.Uncapitalize(gen.name) },
		"needBody":    needBodyFunc,
		"bodyObj":     func(r *rdl.Resource) string { return gen.getBodyObj(r) },
		"iMethodWithHeader":
		               func(r *rdl.Resource) string { return gen.clientMethodSignature(r, true) + ";" },
		"iMethod":     func(r *rdl.Resource) string { return gen.clientMethodSignature(r, false) + ";" },
		"builderExt":  func(r *rdl.Resource) string { return gen.builderExt(r) },
		"origPackage": func() string { return utils.JavaGenerationOrigPackage(gen.schema, gen.ns) },
		"origHeader":  func() string { return utils.JavaGenerationOrigHeader(gen.banner) },
		"returnType":  func(r *rdl.Resource) string { return gen.javaType(gen.registry, r.Type, true, "", "")},
		"needExpect":  needExpectFunc,
		"needImportHashSet":  needImportHashSetFunc,
		"needImportJsonProcessingException": needImportJsonProcessingExceptionFunc,
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(templateSource))
	return t.Execute(gen.writer, gen.schema)
}

func (gen* javaClientGenerator) builderExt(r *rdl.Resource) string {
	code := ""
	spacePad := "        "
	for _, input := range r.Inputs {
		iname := javaName(input.Name)
		if input.PathParam {
			code += spacePad + "xUriBuilder.resolveTemplate(\"" + iname + "\", " + iname + ");\n"
		} else if input.QueryParam != "" {
			code += spacePad + "if (" + iname + " != null) {\n"
			code += spacePad + "    xUriBuilder.queryParam(\"" + input.QueryParam + "\", " + iname + ");\n"
			code += spacePad + "}\n"
		}
	}
	return code
}

func (gen* javaClientGenerator) getBodyObj(r *rdl.Resource) string {
	idx, ok := gen.findFirstUserDefType(r.Inputs)
	if ok { return javaName(r.Inputs[idx].Name) }
	return ""
}

func (gen* javaClientGenerator) findFirstUserDefType(resInputs []*rdl.ResourceInput) (int, bool) {
	for idx, input := range resInputs {
		userType := gen.registry.FindBaseType(input.Type)
		// todo: need consider map or array case
		if userType == rdl.BaseTypeStruct {
			return idx, true
		}
	}
	return -1, false
}

func (gen *javaClientGenerator) needBody(r *rdl.Resource) bool {
	// check inputs is user defined type or not
	_, ok := gen.findFirstUserDefType(r.Inputs)
	return ok
}

const javaClientInterfaceTemplate = `{{origHeader}}
package {{origPackage}}.parsec_generated;

import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import {{package}}.ResourceException;
{{range .Types}}{{if .StructTypeDef}}{{if .StructTypeDef.Name}}import {{package}}.{{.StructTypeDef.Name}};
{{end}}{{end}}{{end}}

public interface {{cName}}Client {
{{range .Resources}}
    {{iMethod .}}
    {{iMethodWithHeader .}}{{end}}
}
`
const javaClientTemplate = `{{origHeader}}
package {{origPackage}}.parsec_generated;

import {{package}}.ResourceException;
{{range .Types}}{{if .StructTypeDef}}{{if .StructTypeDef.Name}}import {{package}}.{{.StructTypeDef.Name}};
{{end}}{{end}}{{end}}
import com.ning.http.client.AsyncHandler;
import com.yahoo.parsec.clients.DefaultAsyncCompletionHandler;
import com.yahoo.parsec.clients.ParsecAsyncHttpClient;
import com.yahoo.parsec.clients.ParsecAsyncHttpRequest;
import com.yahoo.parsec.clients.ParsecAsyncHttpRequest.Builder;
{{if needImportJsonProcessingException .Resources}}
import com.fasterxml.jackson.core.JsonProcessingException;{{end}}
import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.ws.rs.core.UriBuilder;
import java.net.URI;
{{if needImportHashSet .Resources}}import java.util.HashSet;
import java.util.Set;{{end}}
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;

public class {{cName}}ClientImpl implements {{cName}}Client {

    /** Logger. */
    private static final Logger LOGGER = LoggerFactory.getLogger({{cName}}ClientImpl.class);

    /** ParsecAsyncHttpClient. */
    private final ParsecAsyncHttpClient parsecAsyncHttpClient;

    /** Object mapper */
    private final ObjectMapper objectMapper;

    /** URL. */
    private String url;

    /** Headers. */
    private final Map<String, List<String>> defaultHeaders;

    /**
     * connection timeout.
     */
    private static final int IDLE_CONNECTION_TIMEOUT_IN_MS = 15000;

    /**
     * total connections.
     */
    private static final int MAXIMUM_CONNECTIONS_TOTAL = 50;

    public {{cName}}ClientImpl(String url) {
        this(url, null);
    }

    public {{cName}}ClientImpl(
        String url,
        Map<String, List<String>> headers
    ) {

        this.parsecAsyncHttpClient = new ParsecAsyncHttpClient.Builder()
                .setAcceptAnyCertificate(true)
                .setAllowPoolingConnections(true)
                .setPooledConnectionIdleTimeout(IDLE_CONNECTION_TIMEOUT_IN_MS)
                .setMaxConnections(MAXIMUM_CONNECTIONS_TOTAL)
                .build();
        this.objectMapper = new ObjectMapper();
        this.url = url;
        this.defaultHeaders = headers;
    }

    public {{cName}}ClientImpl(
            ParsecAsyncHttpClient client,
            ObjectMapper objectMapper,
            String url,
            Map<String, List<String>> headers
    ) {
        this.parsecAsyncHttpClient = client;
        this.objectMapper = objectMapper;
        this.url = url;
        this.defaultHeaders = headers;
    }

    private ParsecAsyncHttpRequest getRequest(
            String method,
            Map<String, List<String>> headers,
            URI uri,
            String body
    ) throws ResourceException {
        Builder builder = new Builder();

        builder.setUri(uri);
        if (headers != null) {
            for (Map.Entry<String, List<String>> entry : headers.entrySet()) {
                String headerKey = entry.getKey();
                for (String headerValue: entry.getValue()) {
                    builder.addHeader(headerKey, headerValue);
                }
            }
        }

        builder.setMethod(method);

        builder.setBody(body).setBodyEncoding("UTF-8");

        ParsecAsyncHttpRequest request = null;
        try {
            request = builder.build();
        } catch (Exception e) {
            LOGGER.error("builder build failed: " + e.getMessage());
            throw new ResourceException(ResourceException.INTERNAL_SERVER_ERROR, e.getMessage());
        }
        return request;
    }

    public Map<String, List<String>> getDefaultHeaders() {
        return defaultHeaders;
    }
{{range .Resources}}
    @Override
    {{methodSig .}} {
        {{ContentOfNoHeaderMethod .}}
    }

    @Override
    {{methodSigWithHeader .}} {
        String xPath = "{{.Path}}";
        String xBody = null;
{{if needBody .}}
        try {
            xBody = objectMapper.writeValueAsString({{bodyObj .}});
        } catch (JsonProcessingException e) {
            LOGGER.error("JsonProcessingException: " + e.getMessage());
            throw new ResourceException(ResourceException.INTERNAL_SERVER_ERROR, e.getMessage());
        }
{{end}}
        UriBuilder xUriBuilder = UriBuilder.fromUri(this.url).path(xPath);
{{builderExt .}}        URI xUri = xUriBuilder.build();
        if (headers == null) {
            headers = getDefaultHeaders();
        }
        ParsecAsyncHttpRequest xRequest = getRequest("{{.Method}}", headers, xUri, xBody);

{{if needExpect .}}
        Set<Integer> xExpectedStatus = new HashSet<>();
        xExpectedStatus.add(ResourceException.{{.Expected}});
        {{if .Alternatives}}{{range .Alternatives}}xExpectedStatus.add(ResourceException.{{.}});
{{end}}{{end}}
        AsyncHandler<{{returnType .}}> xAsyncHandler = new DefaultAsyncCompletionHandler<>({{returnType .}}.class, xExpectedStatus);
{{else}}
        AsyncHandler<{{returnType .}}> xAsyncHandler = new DefaultAsyncCompletionHandler<>({{returnType .}}.class);
{{end}}
        return parsecAsyncHttpClient.criticalExecute(xRequest, xAsyncHandler);
    }
{{end}}
}
`

// todo: copy from go-schema.go
func safeTypeVarName(rtype rdl.TypeRef) rdl.TypeName {
	tokens := strings.Split(string(rtype), ".")
	return rdl.TypeName(utils.Capitalize(strings.Join(tokens, "")))
}

// todo: duplicate with server code, need integrate
func (gen *javaClientGenerator) javaMethodName(reg rdl.TypeRegistry, r *rdl.Resource, needParamWithType bool) (string, []string) {
	var params []string
	bodyType := string(safeTypeVarName(r.Type))
	for _, v := range r.Inputs {
		if v.Context != "" {
			//ignore these legacy things
			log.Println("Warning: v1 style context param ignored:", v.Name, v.Context)
			continue
		}
		k := v.Name
		if v.QueryParam == "" && !v.PathParam && v.Header == "" {
			bodyType = string(safeTypeVarName(v.Type))
		}
		optional := true
		if (needParamWithType) {
			params = append(params, gen.javaType(reg, v.Type, optional, "", "") + " " + javaName(k))
		} else {
			params = append(params, javaName(k))
		}
	}
	if r.Name != "" {
		return utils.Uncapitalize(string(r.Name)), params
	} else {
		return strings.ToLower(string(r.Method)) + string(bodyType), params
	}
}

// todo: duplicate with java-server.go
func javaName(name rdl.Identifier) string {
	switch name {
	case "type", "default": //other reserved words
		return "_" + string(name)
	default:
		return string(name)
	}
}

func (gen *javaClientGenerator) clientMethodSignature(r *rdl.Resource, needHeader bool) string {
	reg := gen.registry
	returnType := gen.javaType(reg, r.Type, true, "", "")
	methName, params := gen.javaMethodName(reg, r, true)
	sparams := ""
	if (needHeader) {
		sparams = "Map<String, List<String>> headers"
	}
	if len(params) > 0 {
		if (sparams != "") {
			sparams = sparams + ", "
		}
		sparams = sparams + strings.Join(params, ", ")
	}
	return "CompletableFuture<" + returnType + "> " + methName + "(" + sparams + ") throws ResourceException"
}

func (gen *javaClientGenerator) clientMethodOverloadContent(r *rdl.Resource) string {
	reg := gen.registry
	methName, params := gen.javaMethodName(reg, r, false)
	paramsWithEmptyMap := "Collections.emptyMap()"
	if len(params) > 0 {
		paramsWithEmptyMap = paramsWithEmptyMap + ", " + strings.Join(params, ", ")
	}
	return "return " + methName + "(" + paramsWithEmptyMap + ");"
}

func (gen *javaClientGenerator) javaType(reg rdl.TypeRegistry, rdlType rdl.TypeRef, optional bool, items rdl.TypeRef, keys rdl.TypeRef) string {
	ver, err := utils.GetSchemaVersionOrDefault(gen.schema, 1)
	checkErr(err)
	return utils.JavaType(reg, rdlType, optional, items, keys, gen.isPcSuffix, ver)
}
