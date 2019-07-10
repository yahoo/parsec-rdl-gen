// Copyright 2016 Yahoo Inc.
// Licensed under the terms of the Apache license. Please see LICENSE.md file distributed with this work for terms.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/ardielle/ardielle-go/rdl"
	"github.com/yahoo/parsec-rdl-gen/utils"
)

const (
	AnnotationPrefix           = "x_"
	JavaxConstraintPackage     = "javax.validation.constraints"
	JavaxValidationPackage     = "javax.validation"
	JavaxInjectPackage         = "javax.inject"
	HibernateConstraintPackage = "org.hibernate.validator.constraints"
	ParsecConstraintPackage    = "com.yahoo.parsec.constraint.validators"
	ValidationGroupsClass      = "ParsecValidationGroups"
)

// Version is set when building to contain the build version
var Version string

// BuildDate is set when building to contain the build date
var BuildDate string

type javaServerGenerator struct {
	registry       rdl.TypeRegistry
	schema         *rdl.Schema
	name           string
	writer         *bufio.Writer
	err            error
	banner         string
	genAnnotations bool
	imports        []string
	genUsingPath   bool
	namespace      string
	isPcSuffix     bool
}

func main() {
	pOutdir := flag.String("o", ".", "Output directory")
	flag.String("s", "", "RDL source file")
	genAnnotationsString := flag.String("a", "true", "Generate annotations")
	genUsingPathString := flag.String("p", "true", "Generate using path")
	genHandlerImplString := flag.String("i", "true", "Generate interface implementations")
	genParsecErrorString := flag.String("e", "true", "Generate Parsec Error classes")
	namespace := flag.String("ns", "", "Namespace")
	pc := flag.String("pc", "false", "add '_Pc' postfix to the generated java class")
	dataFile := flag.String("df", "", "JSON representation of the schema file")
	flag.Parse()

	genAnnotations, err := strconv.ParseBool(*genAnnotationsString)
	checkErr(err)
	genUsingPath, err := strconv.ParseBool(*genUsingPathString)
	checkErr(err)
	genHandlerImpl, err := strconv.ParseBool(*genHandlerImplString)
	checkErr(err)
	genParsecError, err := strconv.ParseBool(*genParsecErrorString)
	checkErr(err)
	isPcSuffix, err := strconv.ParseBool(*pc)
	checkErr(err)

	var data []byte
	if *dataFile != "" {
		data, err = ioutil.ReadFile(*dataFile)
	} else {
		data, err = ioutil.ReadAll(os.Stdin)
	}
	banner := "parsec-rdl-gen (development version)"
	if Version != "" {
		banner = fmt.Sprintf("parsec-rdl-gen %s %s", Version, BuildDate)
	}

	if err == nil {
		var schema rdl.Schema
		err = json.Unmarshal(data, &schema)
		if err == nil {
			GenerateJavaServer(banner, &schema, *pOutdir, genAnnotations, genHandlerImpl, genUsingPath, genParsecError, *namespace, isPcSuffix)
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

// GenerateJavaServer generates the server code for the RDL-defined service
func GenerateJavaServer(banner string, schema *rdl.Schema, outdir string, genAnnotations bool, genHandlerImpl bool, genUsingPath bool, genParsecError bool, namespace string, isPcSuffix bool) error {
	reg := rdl.NewTypeRegistry(schema)
	packageDir, err := utils.JavaGenerationDir(outdir, schema, namespace)
	if err != nil {
		return err
	}
	cName := utils.Capitalize(string(schema.Name))
	ver := *schema.Version
	if ver > 1 { // if rdl version > 1, we append V{version} in class name
	    cName += "V" + strconv.Itoa(int(ver))
	}

	//FooHandler interface
	out, file, _, err := utils.OutputWriter(packageDir, cName, "Handler.java")
	if err != nil {
		return err
	}
	gen := &javaServerGenerator{reg, schema, cName, out, nil, banner, genAnnotations, nil, genUsingPath, namespace, isPcSuffix}
	gen.processTemplate(javaServerHandlerTemplate)
	out.Flush()
	file.Close()

	for _, r := range schema.Resources {
		if r.Async != nil && *r.Async {
			javaServerMakeAsyncResultModel(banner, schema, reg, outdir, r, genAnnotations, genUsingPath, namespace, isPcSuffix, ver)
		} else if len(r.Outputs) > 0 {
			javaServerMakeResultModel(banner, schema, reg, outdir, r, genAnnotations, genUsingPath, namespace, isPcSuffix, ver)
		}
	}

	//FooHandlerImpl class
	if genHandlerImpl {
		// create source directory
		packageSrcDir, err := utils.JavaGenerationSourceDir(schema, namespace)
		if err != nil {
			return err
		}

		// do nothing if file has already existed
		_, filePath := utils.GetOutputPathInfo(packageSrcDir, cName, "HandlerImpl.java")
		if _, err := os.Stat(filePath); err == nil {
			fmt.Fprintln(os.Stderr, "Warning: interface implementation class exists, ignore: ", filePath)
		} else {
			out, file, _, err = utils.OutputWriter(packageSrcDir, cName, "HandlerImpl.java")
			if err != nil {
				return err
			}
			gen = &javaServerGenerator{reg, schema, cName, out, nil, banner, genAnnotations, nil, genUsingPath, namespace, isPcSuffix}
			packageName := utils.JavaGenerationPackage(schema, namespace)

			ver := *schema.Version
			// import user defined struct classes
			for _, t := range schema.Types {
				tName, tType, _ := rdl.TypeInfo(t)
				if strings.ToLower(string(tType)) == "struct" || strings.ToLower(string(tType)) == "enum" {
					importClass := packageName + "." + string(tName)
					if ver > 1 {
						importClass += "V" + strconv.Itoa(int(ver))
					}
					if isPcSuffix {
						importClass += utils.JavaParsecClassSuffix
					}
					gen.appendImportClass(importClass)
				}
			}
			gen.appendImportClass(packageName + ".ResourceContext")
			gen.appendImportClass(packageName + "." + cName + "Handler")
			gen.processTemplate(javaServerHandlerImplTemplate)
			out.Flush()
			file.Close()
		}
	}

	//ResourceContext interface
	s := "ResourceContext"
	out, file, _, err = utils.OutputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	gen = &javaServerGenerator{reg, schema, cName, out, nil, banner, genAnnotations, nil, genUsingPath, namespace, isPcSuffix}
	gen.processTemplate(javaServerContextTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	//FooResources Jax-RS glue
	out, file, _, err = utils.OutputWriter(packageDir, cName, "Resources.java")
	if err != nil {
		return err
	}
	gen = &javaServerGenerator{reg, schema, cName, out, nil, banner, genAnnotations, nil, genUsingPath, namespace, isPcSuffix}
	for _, r := range schema.Resources {
		gen.generateImportClass(r)
	}
	sort.Strings(gen.imports)
	gen.processTemplate(javaServerTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	//Note: to enable jackson's pretty printer:
	//import com.fasterxml.jackson.jaxrs.annotation.JacksonFeatures;
	//import com.fasterxml.jackson.databind.SerializationFeature;
	//for each resource, add this annotation:
	//   @JacksonFeatures(serializationEnable =  { SerializationFeature.INDENT_OUTPUT })

	//FooServer - an optional server wrapper that sets up Jetty9/Jersey2 to run Foo
	out, file, _, err = utils.OutputWriter(packageDir, cName, "Server.java")
	if err != nil {
		return err
	}
	gen = &javaServerGenerator{reg, schema, cName, out, nil, banner, genAnnotations, nil, genUsingPath, namespace, isPcSuffix}
	gen.processTemplate(javaServerInitTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	//ResourceException - the throawable wrapper for alternate return types
	s = "ResourceException"
	out, file, _, err = utils.OutputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	err = utils.JavaGenerateResourceException(schema, out, namespace)
	out.Flush()
	file.Close()
	if err != nil {
		return err
	}

	//ResourceError - the default data object for an error
	s = "ResourceError"
	out, file, _, err = utils.OutputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	err = utils.JavaGenerateResourceError(schema, out, namespace)
	out.Flush()
	file.Close()

	if genParsecError {
		//ParsecResourceError - the default parsec data object for an error
		s = "ParsecResourceError"
		out, file, _, err = utils.OutputWriter(packageDir, s, ".java")
		if err != nil {
			return err
		}
		err = utils.JavaGenerateParsecResourceError(schema, out, namespace)
		out.Flush()
		file.Close()

		//ParsecErrorBody - the default error body of parsec data object for an error
		s = "ParsecErrorBody"
		out, file, _, err = utils.OutputWriter(packageDir, s, ".java")
		if err != nil {
			return err
		}
		err = utils.JavaGenerateParsecErrorBody(schema, out, namespace)
		out.Flush()
		file.Close()

		//ParsecErrorDetail - the default error detail of parsec data object for an error
		s = "ParsecErrorDetail"
		out, file, _, err = utils.OutputWriter(packageDir, s, ".java")
		if err != nil {
			return err
		}
		err = utils.JavaGenerateParsecErrorDetail(schema, out, namespace)
		out.Flush()
		file.Close()
	}

	return err
}

func javaServerMakeAsyncResultModel(banner string, schema *rdl.Schema, reg rdl.TypeRegistry, outdir string, r *rdl.Resource, genAnnotations bool, genUsingPath bool, namespace string, isPcSuffix bool, apiVer int32) error {
	cName := utils.Capitalize(string(r.Type))
	packageDir, err := utils.JavaGenerationDir(outdir, schema, namespace)
	if err != nil {
		return err
	}
	methName, _ := javaMethodName(reg, r, genUsingPath, isPcSuffix, apiVer)
	s := utils.Capitalize(methName) + "Result"
	out, file, _, err := utils.OutputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	gen := &javaServerGenerator{reg, schema, cName, out, nil, banner, genAnnotations, nil, genUsingPath, namespace, isPcSuffix}
	funcMap := template.FuncMap{
		"header":           func() string { return utils.JavaGenerationHeader(gen.banner) },
		"package":          func() string { return utils.JavaGenerationPackage(gen.schema, namespace) },
		"openBrace":        func() string { return "{" },
		"name":             func() string { return utils.Uncapitalize(string(r.Type)) },
		"cName":            func() string { return utils.Capitalize(string(r.Type)) },
		"resultArgs":       func() string { return gen.resultArgs(r) },
		"resultSig":        func() string { return gen.resultSignature(r) },
		"rName":            func() string { return s },
		"pathParamsKey":    func() string { return gen.makePathParamsKey(r) },
		"pathParamsDecls":  func() string { return gen.makePathParamsDecls(r) },
		"pathParamsSig":    func() []string { return gen.makePathParamsSig(r) },
		"pathParamsAssign": func() string { return gen.makePathParamsAssign(r) },
		"headerParamsSig":  func() []string { return gen.makeHeaderParamsSig(r) },
		"headerAssign":     func() string { return gen.makeHeaderAssign(r) },
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(javaServerAsyncResultTemplate))
	err = t.Execute(gen.writer, gen.schema)
	out.Flush()
	file.Close()
	return err
}

func javaServerMakeResultModel(banner string, schema *rdl.Schema, reg rdl.TypeRegistry, outdir string, r *rdl.Resource, genAnnotations bool, genUsingPath bool, namespace string, isPcSuffix bool, apiVer int32) error {
	rType := string(r.Type)
	cName := utils.Capitalize(rType)
	packageDir, err := utils.JavaGenerationDir(outdir, schema, namespace)
	if err != nil {
		return err
	}
	methName, _ := javaMethodName(reg, r, genUsingPath, isPcSuffix, apiVer)
	s := utils.Capitalize(methName) + "Result"
	out, file, _, err := utils.OutputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	gen := &javaServerGenerator{reg, schema, cName, out, nil, banner, genAnnotations, nil, genUsingPath, namespace, isPcSuffix}
	funcMap := template.FuncMap{
		"header":           func() string { return utils.JavaGenerationHeader(gen.banner) },
		"package":          func() string { return utils.JavaGenerationPackage(gen.schema, namespace) },
		"openBrace":        func() string { return "{" },
		"name":             func() string { return utils.Uncapitalize(rType) },
		"cName":            func() string { return utils.Capitalize(rType) },
		"resultArgs":       func() string { return gen.resultArgs(r) },
		"resultSig":        func() string { return gen.resultSignature(r) },
		"rName":            func() string { return utils.Capitalize(strings.ToLower(r.Method)) + rType + "Result" },
		"pathParamsDecls":  func() string { return gen.makePathParamsDecls(r) },
		"pathParamsSig":    func() []string { return gen.makePathParamsSig(r) },
		"pathParamsAssign": func() string { return gen.makePathParamsAssign(r) },
		"headerParamsSig":  func() []string { return gen.makeHeaderParamsSig(r) },
		"headerAssign":     func() string { return gen.makeHeaderAssign(r) },
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(javaServerResultTemplate))
	err = t.Execute(gen.writer, gen.schema)
	out.Flush()
	file.Close()
	return err
}

func (gen *javaServerGenerator) resultSignature(r *rdl.Resource) string {
	s := gen.javaType(gen.registry, r.Type, false, "", "") + " " + utils.Uncapitalize(string(r.Type)) + "Object"
	for _, out := range r.Outputs {
		s += ", " + gen.javaType(gen.registry, out.Type, false, "", "") + " " + javaName(out.Name)
	}
	return s
}

func (gen *javaServerGenerator) resultArgs(r *rdl.Resource) string {
	s := utils.Uncapitalize(string(r.Type)) + "Object"
	//void?
	for _, out := range r.Outputs {
		s += ", " + javaName(out.Name)
	}
	return s

}

func (gen *javaServerGenerator) makePathParamsKey(r *rdl.Resource) string {
	var s bytes.Buffer
	s.WriteString("\"\"")
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				s.WriteString("+" + javaName(in.Name))
			}
		}
	}
	return s.String()
}

func (gen *javaServerGenerator) makePathParamsDecls(r *rdl.Resource) string {
	s := ""
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				jtype := gen.javaType(gen.registry, in.Type, false, "", "")
				s += "\n    private " + jtype + " " + javaName(in.Name) + ";"
			}
		}
	}
	return s
}

func (gen *javaServerGenerator) makePathParamsSig(r *rdl.Resource) []string {
	s := make([]string, 0)
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				jtype := gen.javaType(gen.registry, in.Type, false, "", "")
				s = append(s, jtype+" "+javaName(in.Name))
			}
		}
	}
	return s
}

func (gen *javaServerGenerator) makePathParamsArgs(r *rdl.Resource) []string {
	s := make([]string, 0)
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				s = append(s, javaName(in.Name))
			}
		}
	}
	return s
}

func (gen *javaServerGenerator) makePathParamsAssign(r *rdl.Resource) string {
	s := ""
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				jname := javaName(in.Name)
				s += "\n        this." + jname + " = " + jname + ";"
			}
		}
	}
	return s
}

func (gen *javaServerGenerator) makeHeaderParamsSig(r *rdl.Resource) []string {
	s := make([]string, 0)
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, out := range r.Outputs {
			jtype := gen.javaType(gen.registry, out.Type, false, "", "")
			s = append(s, jtype+" "+javaName(out.Name))
		}
	}
	return s
}
func (gen *javaServerGenerator) makeHeaderAssign(r *rdl.Resource) string {
	s := ""
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, out := range r.Outputs {
			jname := javaName(out.Name)
			//.header("ETag", revision)
			s += fmt.Sprintf("\n            .header(%q, %s)", out.Header, jname)
		}
	}
	return s
}

const javaServerHandlerTemplate = `{{header}}
package {{package}};

import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import javax.ws.rs.container.AsyncResponse;

//
// {{cName}}Handler is the interface that the service implementation must implement
//
public interface {{cName}}Handler {{openBrace}} {{range .Resources}}
    {{methodSig .}};{{end}}
    public ResourceContext newResourceContext(HttpServletRequest request, HttpServletResponse response);
}
`
const javaServerHandlerImplTemplate = `{{origHeader}}
package {{origPackage}};

{{classImports}}
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;

/**
 * {{cName}}HandlerImpl is interface implementation that implement {{cName}}Handler interface.
 */
public class {{cName}}HandlerImpl implements {{cName}}Handler {{openBrace}}{{range .Resources}}

    @Override
    {{methodSig .}} {
        return null;
    }{{end}}

    @Override
    public ResourceContext newResourceContext(HttpServletRequest request, HttpServletResponse response) {
        return new DefaultResourceContext(request, response);
    }
}
`

const javaServerResultTemplate = `{{header}}
package {{package}};

import java.util.Collection;
import java.util.Map;
import java.util.HashMap;
import javax.ws.rs.core.Response;
import javax.ws.rs.WebApplicationException;

public final class {{rName}} {
    private ResourceContext context;{{pathParamsDecls}}
    private int code; //normal result

    {{rName}}(ResourceContext context) {
        this.context = context;
        this.code = 0;
    }

    public boolean isAsync() { return false; }

    public void done(int _code, {{cName}} {{name}}{{range headerParamsSig}}, {{.}}{{end}}) {
        Response _resp = Response.status(_code).entity({{name}}){{headerAssign}}
            .build();
        throw new WebApplicationException(_resp);
    }

    public void done(int code) {
        done(code, new ResourceError().code(code).message(ResourceException.codeToString(code)));
    }

    public void done(int code, Object entity) {
        this.code = code;
        //to do: check if the exception is declared, and that the entity is of the declared type
        WebApplicationException err = new WebApplicationException(Response.status(code).entity(entity).build());
        throw err; //not optimal
    }

}
`
const javaServerAsyncResultTemplate = `{{header}}
package {{package}};

import java.util.Collection;
import java.util.Map;
import java.util.HashMap;
import javax.ws.rs.container.AsyncResponse;
import javax.ws.rs.container.TimeoutHandler;
import javax.ws.rs.core.Response;
import javax.ws.rs.WebApplicationException;
import java.util.concurrent.TimeUnit;

public final class {{rName}} implements TimeoutHandler {
    private AsyncResponse _async;
    private ResourceContext context;{{pathParamsDecls}}
    private int code; //normal result
    private int timeoutCode;

    {{rName}}(ResourceContext context, {{range pathParamsSig}}{{.}}, {{end}}AsyncResponse async) {
        this.context = context;
        this._async = async;{{pathParamsAssign}}
        this.code = 0;
        this.timeoutCode = 0;
    }

    public boolean isAsync() { return _async != null; }

    public void done(int _code, {{cName}} {{name}}{{range headerParamsSig}}, {{.}}{{end}}) {
        Response _resp = Response.status(_code).entity({{name}}){{headerAssign}}
            .build();
        if (_async == null) {
            throw new WebApplicationException(_resp);
        }
        _async.resume(_resp);
    }

    public void done(int code) {
        done(code, new ResourceError().code(code).message(ResourceException.codeToString(code)));
    }

    public void done(int code, Object entity) {
        this.code = code;
        //to do: check if the exception is declared, and that the entity is of the declared type
        WebApplicationException err = new WebApplicationException(Response.status(code).entity(entity).build());
        if (_async == null) {
            throw err; //not optimal
        }
        _async.resume(err);
    }

    private static Map<String,Map<AsyncResponse,{{rName}}>> _waiters = new HashMap<String,Map<AsyncResponse,{{rName}}>>();

    public void wait({{range pathParamsSig}}{{.}}, {{end}}int _timeout, int _normalStatus, int _timeoutStatus) {
        _async.setTimeout(_timeout, TimeUnit.SECONDS);
        this.code = _normalStatus;
        this.timeoutCode = _timeoutStatus;
        synchronized (_waiters) {
            Map<AsyncResponse,{{rName}}> m = _waiters.get({{pathParamsKey}});
            if (m == null) {
                m = new HashMap<AsyncResponse,{{rName}}>();
                _waiters.put({{pathParamsKey}}, m);
            }
            m.put(_async, this);
            _async.setTimeoutHandler(this);
        }
    }

    public void handleTimeout(AsyncResponse ar) {
        //the timeout is per-request.
        {{rName}} result = null;
        synchronized (_waiters) {
            Map<AsyncResponse,{{rName}}> m = _waiters.get({{pathParamsKey}});
            if (m != null) {
                result = m.remove(ar);
            }
        }
        if (result != null) {
            result.done(timeoutCode);
        }
    }

    //this get called to notifyAll of changed state
    public static void notify({{range pathParamsSig}}{{.}}, {{end}}{{resultSig}}) {
        Collection<{{rName}}> _results = null;
        synchronized (_waiters) {
            Map<AsyncResponse,{{rName}}> m = _waiters.remove({{pathParamsKey}});
            if (m != null) {
                _results = m.values();
            }
        }
        if (_results != null) {
            for ({{rName}} _result : _results) {
                _result.done(_result.code, {{resultArgs}});
            }
        }
    }
}
`

const javaServerContextTemplate = `{{header}}
package {{package}};

import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;

//
// ResourceContext
//
public interface ResourceContext {
    public HttpServletRequest request();
    public HttpServletResponse response();
    public void authenticate();
    public void authorize(String action, String resource, String trustedDomain);
}
`

const javaServerInitTemplate = `{{header}}
package {{package}};

import org.eclipse.jetty.server.Server;
import org.eclipse.jetty.servlet.ServletContextHandler;
import org.eclipse.jetty.servlet.ServletHolder;
import org.glassfish.hk2.utilities.binding.AbstractBinder;
import org.glassfish.jersey.server.ResourceConfig;
import org.glassfish.jersey.servlet.ServletContainer;

public class {{cName}}Server {
    {{cName}}Handler handler;

    public {{cName}}Server({{cName}}Handler handler) {
        this.handler = handler;
    }

    public void run(int port) {
        try {
            Server server = new Server(port);
            ServletContextHandler handler = new ServletContextHandler();
            handler.setContextPath("");
            ResourceConfig config = new ResourceConfig({{cName}}Resources.class).register(new Binder());
            handler.addServlet(new ServletHolder(new ServletContainer(config)), "/*");
            server.setHandler(handler);
            server.start();
            server.join();
        } catch (Exception e) {
            System.err.println("*** " + e);
        }
    }

    class Binder extends AbstractBinder {
        @Override
        protected void configure() {
            bind(handler).to({{cName}}Handler.class);
        }
    }
}
`

const javaServerTemplate = `{{header}}
package {{package}};

import javax.ws.rs.*;
import javax.ws.rs.core.*;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import javax.inject.Inject;
import javax.ws.rs.container.AsyncResponse;
import javax.ws.rs.container.Suspended;
import java.io.IOException;
import java.util.Map;
import java.util.Arrays;
import java.util.LinkedHashMap;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import com.yahoo.parsec.logging.LogUtil;
import com.fasterxml.jackson.databind.ObjectMapper;
{{classImports}}

@Path("{{rootPath}}")
public class {{cName}}Resources {
    private static final Logger LOG = LoggerFactory.getLogger({{cName}}Resources.class);
    private static final ObjectMapper OBJECT_MAPPER = new ObjectMapper();
{{range .Resources}}
    @{{uMethod .}}
    @Path("{{methodPath .}}")
    {{handlerSig .}} {{openBrace}}
{{handlerBody .}}    }
{{end}}

    WebApplicationException typedException(int code, ResourceException e, Class<?> eClass) {
        Object data = e.getData();
        Object entity = eClass.isInstance(data) ? data : null;
        int internalServerErrorCode = ResourceException.INTERNAL_SERVER_ERROR;
        if ((code == internalServerErrorCode && LOG.isErrorEnabled()) || LOG.isDebugEnabled()) {
            String msg = object2StringNoThrow(data);
            String className = this.getClass().getSimpleName();
            // only log two tiers of stacks, there is no problem even if the range overflow
            StackTraceElement[] stacks = Arrays.copyOfRange(Thread.currentThread().getStackTrace(), 1, 3);
            Map<String, String> meta = new LinkedHashMap<>();
            meta.put("trace_tag", className);
            meta.put("http_code", String.valueOf(code));
            meta.put("uri", _request == null ? "" : _request.getRequestURL().toString());
            meta.put("trace_string", Arrays.toString(stacks));
            String logInfo = LogUtil.generateLog(className, msg, meta);
            if (code == internalServerErrorCode) {
                LOG.error(logInfo);
            } else {
                LOG.debug(logInfo);
            }
        }
        if (entity != null)
            return new WebApplicationException(Response.status(code).entity(entity).build());
        else
            return new WebApplicationException(code);
    }

    private static String object2StringNoThrow(Object entity) {
        if (entity == null) {
            return null;
        }

        try {
            return OBJECT_MAPPER.writeValueAsString(entity);
        } catch (IOException e) {
            return null;
        }
    }

    @Inject private {{cName}}Handler _delegate;
    @Context private HttpServletRequest _request;
    @Context private HttpServletResponse _response;

}
`

func (gen *javaServerGenerator) makeJavaTypeRef(reg rdl.TypeRegistry, t *rdl.Type) string {
	switch t.Variant {
	case rdl.TypeVariantStringTypeDef:
		typedef := t.StringTypeDef
		return gen.javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantNumberTypeDef:
		typedef := t.NumberTypeDef
		return gen.javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantArrayTypeDef:
		typedef := t.ArrayTypeDef
		return gen.javaType(reg, typedef.Type, false, typedef.Items, "")
	case rdl.TypeVariantMapTypeDef:
		typedef := t.MapTypeDef
		return gen.javaType(reg, typedef.Type, false, typedef.Items, typedef.Keys)
	case rdl.TypeVariantStructTypeDef:
		typedef := t.StructTypeDef
		return gen.javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantEnumTypeDef:
		typedef := t.EnumTypeDef
		return gen.javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantUnionTypeDef:
		return "interface{}" //! FIX
	}
	return "?" //never happens
}

func (gen *javaServerGenerator) javaType(reg rdl.TypeRegistry, rdlType rdl.TypeRef, optional bool, items rdl.TypeRef, keys rdl.TypeRef) string {
	return utils.JavaType(reg, rdlType, optional, items, keys, gen.isPcSuffix, *gen.schema.Version)
}

func (gen *javaServerGenerator) processTemplate(templateSource string) error {
	commentFun := func(s string) string {
		return utils.FormatComment(s, 0, 80)
	}
	basenameFunc := func(s string) string {
		i := strings.LastIndex(s, ".")
		if i >= 0 {
			s = s[i+1:]
		}
		return s
	}
	fieldFun := func(f rdl.StructFieldDef) string {
		optional := f.Optional
		fType := gen.javaType(gen.registry, f.Type, optional, f.Items, f.Keys)
		fName := utils.Capitalize(string(f.Name))
		option := ""
		if optional {
			option = ",omitempty"
		}
		fAnno := "`json:\"" + string(f.Name) + option + "\"`"
		return fmt.Sprintf("%s %s%s", fName, fType, fAnno)
	}

	funcMap := template.FuncMap{
		"header":      func() string { return utils.JavaGenerationHeader(gen.banner) },
		"package":     func() string { return utils.JavaGenerationPackage(gen.schema, gen.namespace) },
		"openBrace":   func() string { return "{" },
		"field":       fieldFun,
		"flattened":   func(t *rdl.Type) []*rdl.StructFieldDef { return utils.FlattenedFields(gen.registry, t) },
		"typeRef":     func(t *rdl.Type) string { return gen.makeJavaTypeRef(gen.registry, t) },
		"basename":    basenameFunc,
		"comment":     commentFun,
		"uMethod":     func(r *rdl.Resource) string { return strings.ToUpper(r.Method) },
		"methodSig":   func(r *rdl.Resource) string { return gen.serverMethodSignature(r) },
		"handlerSig":  func(r *rdl.Resource) string { return gen.handlerSignature(r) },
		"handlerBody": func(r *rdl.Resource) string { return gen.handlerBody(r) },
		"client":      func() string { return gen.name + "Client" },
		"server":      func() string { return gen.name + "Server" },
		"name":        func() string { return gen.name },
		"cName":       func() string { return utils.Capitalize(gen.name) },
		"methodName":  func(r *rdl.Resource) string { return strings.ToLower(r.Method) + string(r.Type) + "Handler" }, //?
		"methodPath":  func(r *rdl.Resource) string { return gen.resourcePath(r) },
		"rootPath":    func() string { return utils.JavaGenerationRootPath(gen.schema) },
		"rName": func(r *rdl.Resource) string {
			return utils.Capitalize(strings.ToLower(r.Method)) + string(r.Type) + "Result"
		},
		"classImports": func() string { return strings.Join(gen.imports, "") },
		"origPackage":  func() string { return utils.JavaGenerationOrigPackage(gen.schema, gen.namespace) },
		"origHeader":   func() string { return utils.JavaGenerationOrigHeader(gen.banner) },
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(templateSource))
	return t.Execute(gen.writer, gen.schema)
}

func (gen *javaServerGenerator) resourcePath(r *rdl.Resource) string {
	path := r.Path
	i := strings.Index(path, "?")
	if i >= 0 {
		path = path[0:i]
	}
	return path
}

func (gen *javaServerGenerator) handlerBody(r *rdl.Resource) string {
	async := r.Async != nil && *r.Async
	resultWrapper := len(r.Outputs) > 0 || async
	returnType := "void"
	if !resultWrapper {
		returnType = gen.javaType(gen.registry, r.Type, false, "", "")
	}
	s := ""
	if resultWrapper {
		s += "        ResourceContext _context = _delegate.newResourceContext(_request, _response);\n"
	} else {
		s += "        try {\n"
		s += "            ResourceContext _context = _delegate.newResourceContext(_request, _response);\n"
	}
	var fargs []string
	bodyName := ""
	if r.Auth != nil {
		if r.Auth.Authenticate {
			s += "            _context.authenticate();\n"
		} else if r.Auth.Action != "" && r.Auth.Resource != "" {
			resource := r.Auth.Resource
			i := strings.Index(resource, "{")
			for i >= 0 {
				j := strings.Index(resource[i:], "}")
				if j < 0 {
					break
				}
				j += i
				resource = resource[0:i] + "\"+" + resource[i+1:j] + "+\"" + resource[j+1:]
				i = strings.Index(resource, "{")
			}
			resource = "\"" + resource + "\""
			s += fmt.Sprintf("            _context.authorize(%q, %s, null);\n", r.Auth.Action, resource)
			//what about the domain variant?
		} else {
			log.Println("*** Badly formed auth spec in resource input:", r)
		}
	}
	for _, in := range r.Inputs {
		name := javaName(in.Name)
		if in.QueryParam != "" {
			//if !(in.Optional || in.Default != nil) {
			//	log.Println("RDL error: queryparam must either be optional or have a default value:", in.Name, "in resource", r)
			//}
			fargs = append(fargs, name)
		} else if in.PathParam {
			fargs = append(fargs, name)
		} else if in.Header != "" {
			fargs = append(fargs, name)
		} else {
			bodyName = name
			fargs = append(fargs, bodyName)
		}
	}
	methName, _ := javaMethodName(gen.registry, r, gen.genUsingPath, gen.isPcSuffix, *gen.schema.Version)
	sargs := ""
	if len(fargs) > 0 {
		sargs = ", " + strings.Join(fargs, ", ")
	}
	if resultWrapper {
		rName := utils.Capitalize(methName) + "Result"
		s += "        " + rName + " result = new " + rName + "(_context"
		if async {
			s += ", " + strings.Join(append(gen.makePathParamsArgs(r), "asyncResp"), ", ")
		}
		s += ");\n"
		sargs += ", result"
		s += "        _delegate." + methName + "(_context" + sargs + ");\n"
	} else {
		noContent := (r.Expected == "NO_CONTENT" && r.Alternatives == nil) || returnType == "Null"
		s += "            "
		if !noContent {
			s += returnType + " e = "
		}
		s += "_delegate." + methName + "(_context" + sargs + ");\n"
		if len(r.Outputs) > 0 {
			for _, o := range r.Outputs {
				s += fmt.Sprintf("            _response.addHeader(%q, e.%s);\n", o.Header, o.Name)
			}
		}
		if noContent {
			s += "            return Response.noContent().build();\n"
		} else {
			s += "            if (null == e) {\n"
			s += "                return Response.noContent().build();\n"
			s += "            }\n"
			s += "            return Response.status(ResourceException." + r.Expected + ").entity(e).build();\n"
		}
		s += "        } catch (ResourceException e) {\n"
		s += "            int _code = e.getCode();\n"
		s += "            switch (_code) {\n"
		if len(r.Alternatives) > 0 {
			for _, alt := range r.Alternatives {
				s += "            case ResourceException." + alt + ":\n"
			}
			s += "                throw typedException(_code, e, " + returnType + ".class);\n"
		}
		if r.Exceptions != nil && len(r.Exceptions) > 0 {
			for ecode, edef := range r.Exceptions {
				etype := edef.Type
				s += "            case ResourceException." + ecode + ":\n"
				s += "                throw typedException(_code, e, " + etype + ".class);\n"
			}
		}
		s += "            default:\n"
		s += "                System.err.println(\"*** Warning: undeclared exception (\"+_code+\") for resource " + methName + "\");\n"
		s += "                throw typedException(_code, e, ResourceError.class);\n" //? really
		s += "            }\n"
		s += "        }\n"
	}
	return s
}

func (gen *javaServerGenerator) paramInit(qname string, pname string, ptype rdl.TypeRef, pdefault *interface{}) string {
	reg := gen.registry
	s := ""
	gtype := gen.javaType(reg, ptype, false, "", "")
	switch ptype {
	case "String":
		if pdefault == nil {
			s += "\t" + pname + " := optionalStringParam(request, \"" + qname + "\", nil)\n"
		} else {
			def := fmt.Sprintf("%v", pdefault)
			s += "\tvar " + pname + "Optional " + gtype + " = " + def + "\n"
			s += "\t" + pname + " := optionalStringParam(request, \"" + qname + "\", " + pname + "Optional)\n"
		}
	case "Int32":
		if pdefault == nil {
			s += "\t" + pname + ", err := optionalInt32Param(request, \"" + qname + "\", nil)\n"
			s += "\tif err != nil {\n\t\tjsonResponse(writer, 400, err)\n\t\treturn\n\t}\n"
		} else {
			def := "0"
			switch v := (*pdefault).(type) {
			case *float64:
				def = fmt.Sprintf("%v", *v)
			default:
				panic("fix me")
			}
			s += "\t" + pname + ", err := requiredInt32Param(request, \"" + qname + "\", " + def + ")\n"
			s += "\tif err != nil {\n\t\tjsonResponse(writer, 400, err)\n\t\treturn\n\t}\n"
		}
	default:
		panic("fix me")
	}
	return s
}

func (gen *javaServerGenerator) handlerSignature(r *rdl.Resource) string {
	//returnType := utils.JavaType(gen.registry, r.Type, false, "", "")
	returnType := "Response"
	reg := gen.registry
	var params []string
	if r.Async != nil && *r.Async {
		params = append(params, "@Suspended AsyncResponse asyncResp")
		returnType = "void"
	} else if len(r.Outputs) > 0 {
		returnType = "void"
	}
	for _, v := range r.Inputs {
		if v.Context != "" { //ignore these ones
			fmt.Fprintln(os.Stderr, "Warning: v1 style context param ignored:", v.Name, v.Context)
			continue
		}
		k := v.Name
		pdecl := ""
		if len(v.Annotations) == 0 {
			v.Annotations = utils.GetUserDefinedTypeAnnotations(v.Type, gen.schema.Types)
		}
		if v.QueryParam != "" {
			pdecl = gen.extendedValueAnnotation(v.Annotations) + fmt.Sprintf("@QueryParam(%q) ", v.QueryParam) + defaultValueAnnotation(v.Default)
		} else if v.PathParam {
			pdecl = gen.extendedValueAnnotation(v.Annotations) + fmt.Sprintf("@PathParam(%q) ", k)
		} else if v.Header != "" {
			pdecl = gen.extendedValueAnnotation(v.Annotations) + fmt.Sprintf("@HeaderParam(%q) ", v.Header)
		} else {
			pdecl = gen.extendedValueAnnotation(v.Annotations)
		}
		ptype := gen.javaType(reg, v.Type, true, "", "")
		params = append(params, "\n        "+pdecl+ptype+" "+javaName(k))
	}
	spec := ""
	if len(r.Produces) > 0 {
		spec += "@Produces({\"" + strings.Join(r.Produces, ", ") + "\"})\n"
	} else {
		spec += "@Produces(\"application/json;charset=utf-8\")\n"
	}
	switch r.Method {
	case "POST", "PUT":
		if len(r.Consumes) > 0 {
			spec += "    @Consumes({\"" + strings.Join(r.Consumes, ", ") + "\"})\n"
		} else {
			spec += "    @Consumes(\"application/json;charset=utf-8\")\n"
		}
	}
	methName, _ := javaMethodName(gen.registry, r, gen.genUsingPath, gen.isPcSuffix, *gen.schema.Version)
	return spec + "    public " + returnType + " " + methName + "(" + strings.Join(params, ", ") + "\n    )"
}

func defaultValueAnnotation(val interface{}) string {
	if val != nil {
		switch v := val.(type) {
		case string:
			return fmt.Sprintf("@DefaultValue(%q) ", v)
		case int8:
			return fmt.Sprintf("@DefaultValue(\"%d\") ", v)
		case int16:
			return fmt.Sprintf("@DefaultValue(\"%d\") ", v)
		case int32:
			return fmt.Sprintf("@DefaultValue(\"%d\") ", v)
		case int64:
			return fmt.Sprintf("@DefaultValue(\"%d\") ", v)
		case float32:
			return fmt.Sprintf("@DefaultValue(\"%g\") ", v)
		case float64:
			return fmt.Sprintf("@DefaultValue(\"%g\") ", v)
		default:
			return fmt.Sprintf("@DefaultValue(\"%v\") ", v)
		}
	}
	return ""
}

func (gen *javaServerGenerator) extendedValueAnnotation(annotations map[rdl.ExtendedAnnotation]string) string {
	var buffer bytes.Buffer
	for extendedKey, value := range annotations {
		key := strings.TrimLeft(string(extendedKey), AnnotationPrefix)
		switch key {
		case "min":
			buffer.WriteString(generateAnnotation("@Min", value))
		case "max":
			buffer.WriteString(generateAnnotation("@Max", value))
		case "size":
			buffer.WriteString(generateAnnotation("@Size", value))
		case "pattern":
			buffer.WriteString(generateAnnotation("@Pattern", value))
		case "must_validate":
			buffer.WriteString(generateAnnotation("@Valid", ""))
			if value != "" {
				var convertGroupValue bytes.Buffer
				convertGroupValue.WriteString("from = Default.class, to = ")
				convertGroupValue.WriteString(ValidationGroupsClass)
				convertGroupValue.WriteString(".")
				convertGroupValue.WriteString(utils.Capitalize(value))
				convertGroupValue.WriteString(".class")

				buffer.WriteString(generateAnnotation("@ConvertGroup", convertGroupValue.String()))
			}
		case "not_null":
			buffer.WriteString(generateAnnotation("@NotNull", value))
		case "null":
			buffer.WriteString(generateAnnotation("@Null", value))
		case "not_blank":
			buffer.WriteString(generateAnnotation("@NotBlank", value))
		case "not_empty":
			buffer.WriteString(generateAnnotation("@NotEmpty", value))
		case "country_code":
			buffer.WriteString(generateAnnotation("@CountryCode", value))
		case "currency":
			buffer.WriteString(generateAnnotation("@ValidCurrency", value))
		case "language_tag":
			buffer.WriteString(generateAnnotation("@LanguageTag", value))
		case "timezone":
			buffer.WriteString(generateAnnotation("@ValidTimeZone", value))
		case "named":
			buffer.WriteString(generateAnnotation("@Named", fmt.Sprintf("\"%s\"", value)))
		case "date_time":
			buffer.WriteString(generateAnnotation("@DateTime", value))
		case "digits":
			buffer.WriteString(generateAnnotation("@Digits", value))
		default:
			// unrecognized annotation, do nothing
		}
	}
	return buffer.String()
}

func (gen *javaServerGenerator) generateImportClass(r *rdl.Resource) {
	for _, v := range r.Inputs {
		if len(v.Annotations) == 0 {
			v.Annotations = utils.GetUserDefinedTypeAnnotations(v.Type, gen.schema.Types)
		}
		for extendedKey, value := range v.Annotations {
			key := strings.TrimLeft(string(extendedKey), AnnotationPrefix)
			switch key {
			case "min":
				gen.appendImportClass(JavaxConstraintPackage + ".Min")
			case "max":
				gen.appendImportClass(JavaxConstraintPackage + ".Max")
			case "size":
				gen.appendImportClass(JavaxConstraintPackage + ".Size")
			case "pattern":
				gen.appendImportClass(JavaxConstraintPackage + ".Pattern")
			case "must_validate":
				gen.appendImportClass(JavaxValidationPackage + ".Valid")
				if value != "" {
					gen.appendImportClass(JavaxValidationPackage + ".groups.Default")
					gen.appendImportClass(JavaxValidationPackage + ".groups.ConvertGroup")
				}
			case "not_null":
				gen.appendImportClass(JavaxConstraintPackage + ".NotNull")
			case "null":
				gen.appendImportClass(JavaxConstraintPackage + ".Null")
			case "not_blank":
				gen.appendImportClass(JavaxConstraintPackage + ".NotBlank")
			case "not_empty":
				gen.appendImportClass(JavaxConstraintPackage + ".NotEmpty")
			case "country_code":
				gen.appendImportClass(ParsecConstraintPackage + ".CountryCode")
			case "currency":
				gen.appendImportClass(ParsecConstraintPackage + ".ValidCurrency")
			case "language_tag":
				gen.appendImportClass(ParsecConstraintPackage + ".LanguageTag")
			case "timezone":
				gen.appendImportClass(ParsecConstraintPackage + ".ValidTimeZone")
			case "named":
				gen.appendImportClass(JavaxInjectPackage + ".Named")
			case "date_time":
				gen.appendImportClass(ParsecConstraintPackage + ".DateTime")
			case "digits":
				gen.appendImportClass(JavaxConstraintPackage + ".Digits")
			default:
				// unrecognized annotation, do nothing
			}
		}
	}
}

func (gen *javaServerGenerator) appendImportClass(importClass string) {
	importString := fmt.Sprintf("import %s;\n", importClass)
	alreadyExists := false
	for _, value := range gen.imports {
		if importString == value {
			alreadyExists = true
		}
	}
	if !alreadyExists {
		gen.imports = append(gen.imports, importString)
	}
}

func generateAnnotation(key string, value string) string {
	var buffer bytes.Buffer
	buffer.WriteString(key)
	if value != "" {
		buffer.WriteString(fmt.Sprintf("(%s)", value))
	}
	buffer.WriteString(" ")
	return buffer.String()
}

func (gen *javaServerGenerator) handlerReturnType(r *rdl.Resource, methName string, returnType string) string {
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		//return utils.Capitalize(methName) + "Result"
		return "void"
	}
	return returnType
}

func (gen *javaServerGenerator) serverMethodSignature(r *rdl.Resource) string {
	reg := gen.registry
	returnType := gen.javaType(reg, r.Type, false, "", "")
	//noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
	//FIX: if nocontent, return nothing, have a void result, and don't "@Produces" anything
	methName, params := javaMethodName(reg, r, gen.genUsingPath, gen.isPcSuffix, *gen.schema.Version)
	sparams := ""
	if len(params) > 0 {
		sparams = ", " + strings.Join(params, ", ")
	}
	returnType = gen.handlerReturnType(r, methName, returnType)
	if returnType == "void" {
		sparams = sparams + ", " + utils.Capitalize(methName) + "Result result"
	} else if (r.Expected == "NO_CONTENT" && r.Alternatives == nil) || returnType == "Null" {
		returnType = "void"
	}
	return "public " + returnType + " " + methName + "(ResourceContext context" + sparams + ")"
}

func javaMethodName(reg rdl.TypeRegistry, r *rdl.Resource, usePath bool, isPcSuffix bool, apiVer int32) (string, []string) {
	var params []string
	bodyType := r.Type
	for _, v := range r.Inputs {
		if v.Context != "" { //ignore these legacy things
			log.Println("Warning: v1 style context param ignored:", v.Name, v.Context)
			continue
		}
		k := v.Name
		if v.QueryParam == "" && !v.PathParam && v.Header == "" {
			bodyType = v.Type
		}
		//rest_core always uses the boxed type
		optional := true
		params = append(params, utils.JavaType(reg, v.Type, optional, "", "", isPcSuffix, apiVer)+" "+javaName(k))
	}
	if r.Name != "" {
		return utils.Uncapitalize(string(r.Name)), params
	} else {
		if method := strings.ToLower(r.Method); usePath {
			var buffer bytes.Buffer
			pieces := strings.Split(r.Path, "/")
			buffer.WriteString(method)
			counter := 0
			for _, piece := range pieces {
				if piece != "" {
					if strings.Contains(piece, "{") && strings.Contains(piece, "}") {
						counter++
						piece = strings.TrimPrefix(piece, "{")
						piece = strings.TrimSuffix(piece, "}")
						if counter == 1 {
							buffer.WriteString("By")
						} else {
							buffer.WriteString("And")
						}
					}
					buffer.WriteString(utils.Capitalize(piece))
				}
			}
			return buffer.String(), params
		} else {
			return method + string(bodyType), params
		}
	}
}

func javaName(name rdl.Identifier) string {
	switch name {
	case "type", "default": //other reserved words
		return "_" + string(name)
	default:
		return string(name)
	}
}
