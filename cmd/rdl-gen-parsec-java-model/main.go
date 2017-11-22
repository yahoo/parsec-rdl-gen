// Copyright 2016 Yahoo Inc.
// Licensed under the terms of the Apache license. Please see LICENSE.md file distributed with this work for terms.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ardielle/ardielle-go/rdl"
	"github.com/yahoo/parsec-rdl-gen/utils"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	AnnotationPrefix              = "x_"
	JavaxConstraintPackage        = "javax.validation.constraints"
	JavaxValidationPackage        = "javax.validation"
	JavaxXmlBindAnnotationPackage = "javax.xml.bind.annotation"
	HibernateConstraintPackage    = "org.hibernate.validator.constraints"
	ParsecConstraintPackage       = "com.yahoo.parsec.constraint.validators"
	ValidationGroupsKey           = "groups"
	ValidationGroupsClass         = "ParsecValidationGroups"
	ValidationGroupsRegexPattern  = "(^|[ ,])" + ValidationGroupsKey + "\\s?="
)

var (
	ValidationGroupsRegex = regexp.MustCompile(ValidationGroupsRegexPattern)
)

// Version is set when building to contain the build version
var Version string

// BuildDate is set when building to contain the build date
var BuildDate string

var validationGroups map[string]struct{}

type javaModelGenerator struct {
	registry rdl.TypeRegistry
	schema   *rdl.Schema
	name     string
	writer   *bufio.Writer
	err      error
	header   []string
	imports  []string
	body     []string
}

func main() {
	pOutdir := flag.String("o", ".", "Output directory")
	flag.String("s", "", "RDL source file")
	generateAnnotationsString := flag.String("a", "true", "RDL source file")
	namespace := flag.String("ns", "", "Namespace")
	flag.Parse()

	generateAnnotations, err := strconv.ParseBool(*generateAnnotationsString)
	checkErr(err)

	data, err := ioutil.ReadAll(os.Stdin)
	banner := "parsec-rdl-gen (development version)"
	if Version != "" {
		banner = fmt.Sprintf("parsec-rdl-gen %s %s", Version, BuildDate)
	}

	if err == nil {
		var schema rdl.Schema
		err = json.Unmarshal(data, &schema)
		if err == nil {
			GenerateJavaModel(banner, &schema, *pOutdir, generateAnnotations, *namespace)
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

// GenerateJavaModel generates the model code for the types defined in the RDL schema.
func GenerateJavaModel(banner string, schema *rdl.Schema, outdir string, genAnnotations bool, namespace string) error {
	packageDir, err := utils.JavaGenerationDir(outdir, schema, namespace)
	if err != nil {
		return err
	}
	validationGroups = make(map[string]struct{}, 0)
	registry := rdl.NewTypeRegistry(schema)
	for _, t := range schema.Types {
		err := generateJavaType(banner, schema, registry, packageDir, t, genAnnotations, namespace)
		if err != nil {
			return err
		}
	}

	return nil
}

func generateJavaType(banner string, schema *rdl.Schema, registry rdl.TypeRegistry, outdir string, t *rdl.Type, genAnnotations bool, namespace string) error {
	tName, _, _ := rdl.TypeInfo(t)
	bt := registry.BaseType(t)
	switch bt {
	case rdl.BaseTypeStruct:
	//case rdl.BaseTypeUnion:
	//case rdl.BaseTypeArray: //? a list subtype, to avoid generics and erasure?
	case rdl.BaseTypeEnum:
	default:
		fmt.Fprintf(os.Stderr, "[Ignoring type %s]\n", tName)
		return nil
	}
	cName := utils.Capitalize(string(tName))
	out, file, _, err := utils.OutputWriter(outdir, cName, ".java")
	if err != nil {
		return err
	}
	if file != nil {
		defer file.Close()
	}
	gen := &javaModelGenerator{registry, schema, string(tName), out, nil, nil, nil, nil}
	gen.generateHeader(banner, namespace)
	switch bt {
	case rdl.BaseTypeStruct:
		gen.appendToBody("\n")
		gen.generateStruct(t, cName, genAnnotations)
	case rdl.BaseTypeUnion:
		gen.appendToBody("\n")
		gen.generateUnion(t)
	case rdl.BaseTypeArray:
		gen.appendToBody("\n")
		gen.generateArray(t)
	case rdl.BaseTypeEnum:
		gen.appendToBody("\n")
		gen.generateTypeComment(t)
		gen.generateEnum(t)
	}

	for _, header := range gen.header {
		gen.emit(header)
	}

	sort.Strings(gen.imports)
	for _, imports := range gen.imports {
		gen.emit(imports)
	}

	for _, body := range gen.body {
		gen.emit(body)
	}

	out.Flush()
	return gen.err
}

func (gen *javaModelGenerator) emit(s string) {
	if gen.err == nil {
		_, err := gen.writer.WriteString(s)
		if err != nil {
			gen.err = err
		}
	}
}

func (gen *javaModelGenerator) structHasFieldDefault(t *rdl.StructTypeDef) bool {
	if t != nil {
		for _, f := range t.Fields {
			if f.Default != nil {
				switch gen.registry.FindBaseType(f.Type) {
				case rdl.BaseTypeString, rdl.BaseTypeSymbol, rdl.BaseTypeUUID, rdl.BaseTypeTimestamp:
					if f.Default.(string) != "" {
						return true
					}
				case rdl.BaseTypeInt8, rdl.BaseTypeInt16, rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeFloat32, rdl.BaseTypeFloat64:
					if f.Default.(float64) != 0 {
						return true
					}
				case rdl.BaseTypeBool:
					if f.Default.(bool) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (gen *javaModelGenerator) generateHeader(banner string, namespace string) {
	gen.appendToHeader(utils.JavaGenerationHeader(banner))
	gen.appendToHeader("\n\n")
	pack := utils.JavaGenerationPackage(gen.schema, namespace)
	if pack != "" {
		gen.appendToHeader("package " + utils.JavaGenerationPackage(gen.schema, namespace) + ";\n\n")
	}
	gen.imports = append(gen.imports, "import java.util.List;\n")
	gen.imports = append(gen.imports, "import java.util.Map;\n")

	// import hashcode, equals builders, toString builder
	gen.imports = append(gen.imports, "import org.apache.commons.lang3.builder.EqualsBuilder;\n")
	gen.imports = append(gen.imports, "import org.apache.commons.lang3.builder.HashCodeBuilder;\n")
	gen.imports = append(gen.imports, "import org.apache.commons.lang3.builder.ToStringBuilder;\n")
	gen.imports = append(gen.imports, "import org.apache.commons.lang3.builder.ToStringStyle;\n")
	gen.imports = append(gen.imports, "import javax.xml.bind.annotation.XmlAnyElement;\n")
}

func (gen *javaModelGenerator) generateTypeComment(t *rdl.Type) {
	tName, _, tComment := rdl.TypeInfo(t)
	s := string(tName) + " -"
	if tComment != "" {
		s += " " + tComment
	}
	gen.appendToBody(utils.FormatComment(s, 0, 80))
}

func (gen *javaModelGenerator) generateEquals() {
	gen.appendToBody("\n")
	gen.appendToBody("    @Override\n")
	gen.appendToBody("    public boolean equals(Object obj) {\n")
	gen.appendToBody("        return EqualsBuilder.reflectionEquals(this, obj, false);\n")
	gen.appendToBody("    }\n")
}

func (gen *javaModelGenerator) generateHashCode() {
	gen.appendToBody("\n")
	gen.appendToBody("    @Override\n")
	gen.appendToBody("    public int hashCode() {\n")
	gen.appendToBody("        return HashCodeBuilder.reflectionHashCode(this, false);\n")
	gen.appendToBody("    }\n")
}

func (gen *javaModelGenerator) generateToString() {
	gen.appendToBody("\n")
	gen.appendToBody("    @Override\n")
	gen.appendToBody("    public String toString() {\n")
	gen.appendToBody("        return ToStringBuilder.reflectionToString(this, ToStringStyle.SHORT_PREFIX_STYLE);\n")
	gen.appendToBody("    }\n")
}

func (gen *javaModelGenerator) generateUnion(t *rdl.Type) {
	tName, _, _ := rdl.TypeInfo(t)
	ut := t.UnionTypeDef
	uName := utils.Capitalize(string(tName))
	gen.appendToBody(fmt.Sprintf("// %sVariantTag - generated to support %s\n", uName, uName))
	gen.appendToBody(fmt.Sprintf("type %sVariantTag int\n\n", uName))
	gen.appendToBody("// Supporting constants\n")
	gen.appendToBody("const (\n")
	gen.appendToBody(fmt.Sprintf("\t_ %sVariantTag = iota\n", uName))
	for _, v := range ut.Variants {
		uV := utils.Capitalize(string(v))
		gen.appendToBody(fmt.Sprintf("\t%sVariant%s\n", uName, uV))
	}
	gen.appendToBody(")\n\n")

	maxKeyLen := len("Variant")
	for _, v := range ut.Variants {
		if len(v) > maxKeyLen {
			maxKeyLen = len(v)
		}
	}
	gen.generateTypeComment(t)
	gen.appendToBody(fmt.Sprintf("type %s struct {\n", uName))
	s := utils.LeftJustified("Variant", maxKeyLen)
	gen.appendToBody(fmt.Sprintf("\t%s %sVariantTag\n", s, uName))
	for _, v := range ut.Variants {
		uV := utils.Capitalize(string(v))
		vType := utils.JavaType(gen.registry, v, false, "", "")
		s := utils.LeftJustified(uV, maxKeyLen)
		gen.appendToBody(fmt.Sprintf("\t%s *%s\n", s, vType))
	}
	gen.appendToBody("}\n\n")
	gen.appendToBody(fmt.Sprintf("func (u %s) String() string {\n", uName))
	gen.appendToBody("\tswitch u.Variant {\n")
	for _, v := range ut.Variants {
		uV := utils.Capitalize(string(v))
		gen.appendToBody(fmt.Sprintf("\tcase %sVariant%s:\n", uName, uV))
		gen.appendToBody(fmt.Sprintf("\t\treturn fmt.Sprintf(\"%%v\", u.%s)\n", uV))
	}
	gen.appendToBody("\tdefault:\n")
	gen.appendToBody(fmt.Sprintf("\t\treturn \"<%s uninitialized>\"\n", uName))
	gen.appendToBody("\t}\n")
	gen.appendToBody("}\n\n")
	gen.appendToBody(fmt.Sprintf("// MarshalJSON for %s\n", uName))
	gen.appendToBody(fmt.Sprintf("func (u %s) MarshalJSON() ([]byte, error) {\n", uName))
	gen.appendToBody("\tswitch u.Variant {\n")
	for _, v := range ut.Variants {
		uV := utils.Capitalize(string(v))
		gen.appendToBody(fmt.Sprintf("\tcase %sVariant%s:\n", uName, uV))
		gen.appendToBody(fmt.Sprintf("\t\treturn json.Marshal(u.%s)\n", uV))
	}
	gen.appendToBody("\tdefault:\n")
	gen.appendToBody(fmt.Sprintf("\t\treturn nil, fmt.Errorf(\"Cannot marshal uninitialized union type %s\")\n", uName))
	gen.appendToBody("\t}\n")
	gen.appendToBody("}\n\n")

	hasStructs := false
	for _, v := range ut.Variants {
		t := gen.registry.FindType(v)
		if t != nil {
			bt := gen.registry.BaseType(t)
			if bt == rdl.BaseTypeStruct {
				if t.StructTypeDef != nil && t.StructTypeDef.Fields != nil {
					hasStructs = true
					break
				}
			}
		}
	}
	if hasStructs {
		gen.appendToBody(fmt.Sprintf("func check%sStructFields(repr map[string]interface{}, fields ...string) bool {\n", uName))
		gen.appendToBody("\tfor _, s := range fields {\n")
		gen.appendToBody("\t\tif _, ok := repr[s]; !ok {\n")
		gen.appendToBody("\t\t\treturn false\n")
		gen.appendToBody("\t\t}\n")
		gen.appendToBody("\t}\n")
		gen.appendToBody("\treturn true\n")
		gen.appendToBody("}\n\n")
	}

	var structVariants []rdl.TypeRef
	var stringVariants []rdl.TypeRef
	var boolVariants []rdl.TypeRef
	var int8Variants []rdl.TypeRef
	var int16Variants []rdl.TypeRef
	var int32Variants []rdl.TypeRef
	var int64Variants []rdl.TypeRef
	var float32Variants []rdl.TypeRef
	var float64Variants []rdl.TypeRef

	for _, v := range ut.Variants {
		uV := utils.Capitalize(string(v))
		t := gen.registry.FindType(v)
		switch t.Variant {
		case rdl.TypeVariantStructTypeDef:
			structVariants = append(structVariants, v)
			if t.StructTypeDef.Fields != nil {
				names := ""
				for _, f := range t.StructTypeDef.Fields {
					s := fmt.Sprintf("%q", f.Name)
					if names == "" {
						names = s
					} else {
						names = names + ", " + s
					}
				}
				if names != "" {

					gen.appendToBody(fmt.Sprintf("func make%sVariant%s(b []byte, u *%s, fields map[string]interface{}) bool {\n", uName, uV, uName))
					gen.appendToBody(fmt.Sprintf("\tif check%sStructFields(fields, %s) {\n", uName, names))
					gen.appendToBody(fmt.Sprintf("\t\tvar o %s\n", uV))
					gen.appendToBody("\t\tif err := json.Unmarshal(b, &o); err == nil {\n")
					gen.appendToBody(fmt.Sprintf("\t\t\tup := new(%s)\n", uName))
					gen.appendToBody(fmt.Sprintf("\t\t\tup.Variant = %sVariant%s\n", uName, uV))
					gen.appendToBody(fmt.Sprintf("\t\t\tup.%s = &o\n", uV))
					gen.appendToBody("\t\t\t*u = *up\n")
					gen.appendToBody("\t\t\treturn true\n")
					gen.appendToBody("\t\t}\n")
					gen.appendToBody("\t}\n")
					gen.appendToBody("\treturn false\n")
					gen.appendToBody("}\n\n")
				}
			}
		default:
			bt := gen.registry.FindBaseType(v)
			switch bt {
			case rdl.BaseTypeBool:
				boolVariants = append(boolVariants, v)
			case rdl.BaseTypeString:
				stringVariants = append(stringVariants, v)
			case rdl.BaseTypeInt8:
				int8Variants = append(int8Variants, v)
			case rdl.BaseTypeInt16:
				int16Variants = append(int16Variants, v)
			case rdl.BaseTypeInt32:
				int32Variants = append(int32Variants, v)
			case rdl.BaseTypeInt64:
				int64Variants = append(int64Variants, v)
			case rdl.BaseTypeFloat32:
				float32Variants = append(float32Variants, v)
			case rdl.BaseTypeFloat64:
				float64Variants = append(float64Variants, v)
			case rdl.BaseTypeEnum:
			default:
				panic("fix me: " + bt.String())
			}
		}
	}
	gen.appendToBody(fmt.Sprintf("// UnmarshalJSON for %s\n", uName))
	gen.appendToBody(fmt.Sprintf("func (u *%s) UnmarshalJSON(b []byte) error {\n", uName))
	gen.appendToBody("\tvar tmp interface{}\n")
	gen.appendToBody("\tif err := json.Unmarshal(b, &tmp); err != nil {\n")
	gen.appendToBody("\t\treturn err\n")
	gen.appendToBody("\t}\n")
	gen.appendToBody("\tswitch v := tmp.(type) {\n")

	if len(structVariants) > 0 {
		gen.appendToBody("\tcase map[string]interface{}:\n")
		for _, v := range structVariants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\tif make%sVariant%s(b, u, v) {\n", uName, uV))
			gen.appendToBody("\t\t\treturn nil\n")
			gen.appendToBody("\t\t}\n")
		}
	}
	if len(stringVariants) > 0 {
		gen.appendToBody("\tcase string:\n")
		for _, v := range stringVariants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\t*u = %s{%sVariant%s, &v, nil, nil}\n", uName, uName, uV))
			gen.appendToBody("\t\treturn nil\n")
			break //the first one is the one we take
		}
	}
	if len(boolVariants) > 0 {
		gen.appendToBody("\tcase bool:\n")
		for _, v := range boolVariants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\t*u = %s{%sVariant%s, &v, nil, nil, nil, nil, nil}\n", uName, uName, uV))
			gen.appendToBody("\t\treturn nil\n")
			break //the first one is the one we take
		}
	}
	if len(int8Variants) > 0 {
		gen.appendToBody("\tcase int8:\n")
		for _, v := range int8Variants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\t*u = %s{%sVariant%s, &v, nil, nil, nil, nil, nil}\n", uName, uName, uV))
			gen.appendToBody("\t\treturn nil\n")
			break //the first one is the one we take
		}
	}
	if len(int16Variants) > 0 {
		gen.appendToBody("\tcase int16:\n")
		for _, v := range int16Variants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\t*u = %s{%sVariant%s, nil, &v, nil, nil, nil, nil}\n", uName, uName, uV))
			gen.appendToBody("\t\treturn nil\n")
			break //the first one is the one we take
		}
	}
	if len(int32Variants) > 0 {
		gen.appendToBody("\tcase int32:\n")
		for _, v := range int32Variants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\t*u = %s{%sVariant%s, nil, nil, &v, nil, nil, nil}\n", uName, uName, uV))
			gen.appendToBody("\t\treturn nil\n")
			break //the first one is the one we take
		}
	}
	if len(int64Variants) > 0 {
		gen.appendToBody("\tcase int64:\n")
		for _, v := range int64Variants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\t*u = %s{%sVariant%s, nil, nil, nil, &v, nil, nil}\n", uName, uName, uV))
			gen.appendToBody("\t\treturn nil\n")
			break //the first one is the one we take
		}
	}
	if len(float32Variants) > 0 {
		gen.appendToBody("\tcase float32:\n")
		for _, v := range float32Variants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\t*u = %s{%sVariant%s, nil, nil, nil, nil, &v, nil}\n", uName, uName, uV))
			gen.appendToBody("\t\treturn nil\n")
			break //the first one is the one we take
		}
	}
	if len(float64Variants) > 0 {
		gen.appendToBody("\tcase float64:\n")
		for _, v := range float64Variants {
			uV := utils.Capitalize(string(v))
			gen.appendToBody(fmt.Sprintf("\t\t*u = %s{%sVariant%s, nil, nil, nil, nil, nil, &v}\n", uName, uName, uV))
			gen.appendToBody("\t\treturn nil\n")
			break //the first one is the one we take
		}
	}
	gen.appendToBody("\t}\n")
	gen.appendToBody(fmt.Sprintf("\treturn fmt.Errorf(\"Cannot unmarshal JSON to union type %s\")\n", uName))
	gen.appendToBody("}\n")
}

func (gen *javaModelGenerator) literal(lit interface{}) string {
	switch v := lit.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case int16:
		return fmt.Sprintf("%d", v)
	case int8:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case float32:
		return fmt.Sprintf("%g", v)
	default: //bool, enum
		return fmt.Sprintf("%v", lit)
	}
}

func (gen *javaModelGenerator) generateArray(t *rdl.Type) {
	if gen.err == nil {
		switch t.Variant {
		case rdl.TypeVariantArrayTypeDef:
			at := t.ArrayTypeDef
			gen.generateTypeComment(t)
			ftype := utils.JavaType(gen.registry, at.Type, false, at.Items, "")
			gen.appendToBody(fmt.Sprintf("type %s %s\n\n", at.Name, ftype))
		default:
			tName, tType, _ := rdl.TypeInfo(t)
			gtype := utils.JavaType(gen.registry, tType, false, "", "")
			gen.generateTypeComment(t)
			gen.appendToBody(fmt.Sprintf("type %s %s\n\n", tName, gtype))
		}
	}
}

func (gen *javaModelGenerator) generateStruct(t *rdl.Type, cName string, genAnnotations bool) {
	if gen.err == nil {
		switch t.Variant {
		case rdl.TypeVariantStructTypeDef:
			st := t.StructTypeDef
			f := utils.FlattenedFields(gen.registry, t)
			gen.generateTypeComment(t)
			gen.generateStructFields(f, st.Name, st.Comment, cName, st.Annotations, genAnnotations)
			if gen.structHasFieldDefault(st) {
				gen.appendToBody("\n    //\n    // sets up the instance according to its default field values, if any\n    //\n")
				gen.appendToBody(fmt.Sprintf("    private void init() {\n"))
				for _, f := range f {
					if f.Default != nil {
						gen.appendToBody(fmt.Sprintf("        %s = %s;\n", f.Name, gen.literal(f.Default)))
					}
				}
				gen.appendToBody("    }\n")
				gen.appendToBody(fmt.Sprintf("    public %s() { init(); }\n", cName))
			} else {
				gen.appendToBody(fmt.Sprintf("    public %s() {  }\n", cName))
			}

			gen.generateHashCode()
			gen.generateEquals()
			gen.generateToString()
			gen.appendToBody("}\n")
		default:
			panic(fmt.Sprintf("Unreasonable struct typedef: %v", t.Variant))
		}
	}
}

func (gen *javaModelGenerator) generateEnum(t *rdl.Type) {
	if gen.err != nil {
		return
	}
	et := t.EnumTypeDef
	name := utils.Capitalize(string(et.Name))
	gen.appendToBody(fmt.Sprintf("public enum %s {", name))
	for i, elem := range et.Elements {
		sym := elem.Symbol
		if i > 0 {
			gen.appendToBody(",\n")
		} else {
			gen.appendToBody("\n")
		}
		gen.appendToBody(fmt.Sprintf("    %s", sym))
	}
	gen.appendToBody(";\n")
	gen.appendToBody(fmt.Sprintf("\n    public static %s fromString(String v) {\n", name))
	gen.appendToBody(fmt.Sprintf("        for (%s e : values()) {\n", name))
	gen.appendToBody("            if (e.toString().equals(v)) {\n")
	gen.appendToBody("                return e;\n")
	gen.appendToBody("            }\n")
	gen.appendToBody("        }\n")
	gen.appendToBody(fmt.Sprintf("        throw new IllegalArgumentException(\"Invalid string representation for %s: \" + v);\n", name))
	gen.appendToBody("    }\n")
	gen.appendToBody("}\n")

}

func (gen *javaModelGenerator) generateStructFields(fields []*rdl.StructFieldDef, name rdl.TypeName, comment string, cName string, annotations map[rdl.ExtendedAnnotation]string, genAnnotations bool) {
	gen.appendToBody(fmt.Sprintf("public final class %s implements java.io.Serializable {\n", name))
	if fields != nil {
		fnames := make([]string, 0, len(fields))
		ftypes := make([]string, 0, len(fields))
		fannotations := make([]map[rdl.ExtendedAnnotation]string, 0, len(fields))
		for _, f := range fields {
			gen.appendToBody("\n")

			if genAnnotations {
				if len(f.Annotations) == 0 {
					f.Annotations = utils.GetUserDefinedTypeAnnotations(f.Type, gen.schema.Types)
				}
				fannotations = append(fannotations, f.Annotations)
				for extendedKey, value := range f.Annotations {
					gen.appendToBody("    ")
					gen.generateValidationGroupAnnotation(extendedKey, value)
					gen.appendToBody("\n")
				}
			}

			fname := javaFieldName(f.Name)
			fnames = append(fnames, fname)
			optional := f.Optional

			ftype := utils.JavaType(gen.registry, f.Type, optional, f.Items, f.Keys)
			ftypes = append(ftypes, ftype)

			gen.appendToBody("    private ")
			gen.generateStructFieldType(f.Type, optional, f.Items, f.Keys)
			gen.appendToBody(fmt.Sprintf(" %s;\n", fname))

		}

		gen.appendToBody("\n")
		gen.appendToBody("    // This annotated field 'reserved' is used to handle the Moxy unmarshall error\n")
		gen.appendToBody("    // case when user requests some unknown fields which are nullable.\n")
		gen.appendToBody("    @XmlAnyElement(lax=true)\n")
		gen.appendToBody("    private Object parsecReserved;\n")
		for i := range fields {
			fname := fnames[i]
			ftype := ftypes[i]
			if genAnnotations {
				gen.generateStructFieldGetterAnnotations(fannotations[i])
			}
			gen.appendToBody(fmt.Sprintf("    public %s get%s() { return %s; }\n", ftype, upperFirst(fname), fname))
		}
		gen.appendToBody("\n")
		for i := range fields {
			fname := fnames[i]
			ftype := ftypes[i]
			if genAnnotations {
				gen.generateStructFieldSetterAnnotations(fannotations[i])
			}
			gen.appendToBody(fmt.Sprintf("    public %s set%s(%s %s) { this.%s = %s; return this; }\n", cName, upperFirst(fname), ftype, fname, fname, fname))
		}
	}
}

func (gen *javaModelGenerator) generateStructFieldType(rdlType rdl.TypeRef, optional bool, items rdl.TypeRef, keys rdl.TypeRef) {
	t := gen.registry.FindType(rdlType)
	if t == nil || t.Variant == 0 {
		panic("Cannot find type '" + rdlType + "'")
	}
	bt := gen.registry.BaseType(t)
	switch bt {
	case rdl.BaseTypeArray:
		i := rdl.TypeRef("Any")
		switch t.Variant {
		case rdl.TypeVariantArrayTypeDef:
			i = t.ArrayTypeDef.Items
		default:
			if items != "" && items != "Any" {
				i = items
			}
		}
		gen.appendToBody("List<")
		gen.generateStructFieldParamType(i, true, "", "")
		gen.appendToBody(">")
	case rdl.BaseTypeMap:
		k := rdl.TypeRef("Any")
		i := rdl.TypeRef("Any")
		switch t.Variant {
		case rdl.TypeVariantMapTypeDef:
			k = t.MapTypeDef.Keys
			i = t.MapTypeDef.Items
		default:
			if keys != "" && keys != "Any" {
				k = keys
			}
			if items != "" && keys != "Any" {
				i = items
			}
		}
		gen.appendToBody("Map<")
		gen.generateStructFieldParamType(k, true, "", "")
		gen.appendToBody(", ")
		gen.generateStructFieldParamType(i, true, "", "")
		gen.appendToBody(">")
	default:
		gen.appendToBody(utils.JavaType(gen.registry, rdlType, optional, items, keys))
	}
}

func (gen *javaModelGenerator) generateStructFieldParamType(rdlType rdl.TypeRef, optional bool, items rdl.TypeRef, keys rdl.TypeRef) {
	annotations := utils.GetUserDefinedTypeAnnotations(rdlType, gen.schema.Types)
	for extendedKey, value := range annotations {
		gen.appendToBody("\n        ")
		gen.generateValidationGroupAnnotation(extendedKey, value)
		gen.appendToBody(" ")
	}
	gen.generateStructFieldType(rdlType, optional, items, keys)
}

func (gen *javaModelGenerator) generateValidationGroupAnnotation(extendedKey rdl.ExtendedAnnotation, value string) {
	key := strings.TrimLeft(string(extendedKey), AnnotationPrefix)
	match := ValidationGroupsRegex.MatchString(value)

	// fmt.Fprintln(os.Stderr, "key=" + key + ",value=" + value)
	// Checking if there are validation groups
	if match {
		value = gen.getValidationGroupValue(value)
	}

	switch key {
	case "min":
		gen.appendAnnotation("@Min", value)
		gen.appendImportClass(JavaxConstraintPackage + ".Min")
	case "max":
		gen.appendAnnotation("@Max", value)
		gen.appendImportClass(JavaxConstraintPackage + ".Max")
	case "size":
		gen.appendAnnotation("@Size", value)
		gen.appendImportClass(JavaxConstraintPackage + ".Size")
	case "pattern":
		gen.appendAnnotation("@Pattern", value)
		gen.appendImportClass(JavaxConstraintPackage + ".Pattern")
	case "must_validate":
		gen.appendAnnotation("@Valid", "")
		gen.appendImportClass(JavaxValidationPackage + ".Valid")
	case "name":
		gen.appendAnnotation("@XmlElement", fmt.Sprintf("name=\"%s\"", value))
		gen.appendImportClass(JavaxXmlBindAnnotationPackage + ".XmlElement")
	case "not_null":
		gen.appendAnnotation("@NotNull", value)
		gen.appendImportClass(JavaxConstraintPackage + ".NotNull")
	case "null":
		gen.appendAnnotation("@Null", value)
		gen.appendImportClass(JavaxConstraintPackage + ".Null")
	case "not_blank":
		gen.appendAnnotation("@NotBlank", value)
		gen.appendImportClass(HibernateConstraintPackage + ".NotBlank")
	case "not_empty":
		gen.appendAnnotation("@NotEmpty", value)
		gen.appendImportClass(HibernateConstraintPackage + ".NotEmpty")
	case "country_code":
		gen.appendAnnotation("@CountryCode", "")
		gen.appendImportClass(ParsecConstraintPackage + ".CountryCode")
	case "currency":
		gen.appendAnnotation("@ValidCurrency", "")
		gen.appendImportClass(ParsecConstraintPackage + ".ValidCurrency")
	case "language_tag":
		gen.appendAnnotation("@LanguageTag", "")
		gen.appendImportClass(ParsecConstraintPackage + ".LanguageTag")
	case "date_time":
		gen.appendAnnotation("@DateTime", "")
		gen.appendImportClass(ParsecConstraintPackage + ".DateTime")
	case "digits":
		gen.appendAnnotation("@Digits", value)
		gen.appendImportClass(JavaxConstraintPackage + ".Digits")
	case "adapter":
		gen.appendAnnotation("@XmlJavaTypeAdapter", value)
		gen.appendImportClass(JavaxXmlBindAnnotationPackage + ".adapters.XmlJavaTypeAdapter")
	default:
		// unrecognized annotation, do nothing
	}
}

func (gen *javaModelGenerator) generateStructFieldGetterAnnotations(annotations map[rdl.ExtendedAnnotation]string) {
	gen.appendToBody("\n")
	for extendedKey, value := range annotations {
		key := strings.TrimLeft(string(extendedKey), AnnotationPrefix)
		switch key {
		case "name":
			gen.appendAnnotation("    @XmlElement", fmt.Sprintf("name=\"%s\"", value))
			gen.appendToBody("\n")
			gen.appendImportClass(JavaxXmlBindAnnotationPackage + ".XmlElement")
		default:
			// unrecognized annotation, do nothing
		}
	}
}

func (gen *javaModelGenerator) generateStructFieldSetterAnnotations(annotations map[rdl.ExtendedAnnotation]string) {
	gen.appendToBody("\n")
	for extendedKey, value := range annotations {
		key := strings.TrimLeft(string(extendedKey), AnnotationPrefix)
		switch key {
		case "name":
			gen.appendAnnotation("    @XmlElement", fmt.Sprintf("name=\"%s\"", value))
			gen.appendToBody("\n")
			gen.appendImportClass(JavaxXmlBindAnnotationPackage + ".XmlElement")
		default:
			// unrecognized annotation, do nothing
		}
	}
}

func (gen *javaModelGenerator) appendAnnotation(key string, value string) {
	gen.appendToBody(key)
	if value != "" {
		gen.appendToBody(fmt.Sprintf("(%s)", value))
	}
}

func (gen *javaModelGenerator) appendImportClass(importClass string) {
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

func (gen *javaModelGenerator) appendToHeader(header string) {
	gen.header = append(gen.header, header)
}

func (gen *javaModelGenerator) appendToBody(body string) {
	gen.body = append(gen.body, body)
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	r0, n0 := utf8.DecodeRuneInString(s)
	r1, _ := utf8.DecodeRuneInString(s[n0:])
	if r1 != utf8.RuneError && unicode.IsLower(r0) && unicode.IsUpper(r1) {
		return s
	}
	return string(unicode.ToUpper(r0)) + s[n0:]
}

func (gen *javaModelGenerator) getValidationGroupValue(annotationValue string) string {
	var outputBuffer bytes.Buffer
	outputBuffer.WriteString("\n        ")

	annotationValuePieces := utils.Split(annotationValue, ',')
	for _, annotationValuePiece := range annotationValuePieces {
		temp := utils.Split(annotationValuePiece, '=')
		key := strings.Trim(temp[0], " ")

		if key == ValidationGroupsKey {
			if len(temp) == 2 {
				var groupsBuffer bytes.Buffer
				groupsBuffer.WriteString(ValidationGroupsKey)
				groupsBuffer.WriteString(" = {\n            ")
				groups := strings.Split(strings.Replace(temp[1], " ", "", -1), "|")

				for _, group := range groups {
					if group != "" {
						group = utils.Capitalize(group)
						validationGroups[group] = struct{}{}

						groupsBuffer.WriteString(ValidationGroupsClass)
						groupsBuffer.WriteString(".")
						groupsBuffer.WriteString(group)
						groupsBuffer.WriteString(".class,\n            ")
					}
				}

				outputBuffer.WriteString(strings.TrimRight(groupsBuffer.String(), ",\n "))
				outputBuffer.WriteString("\n        }")
			}
		} else {
			if len(temp) == 1 {
				outputBuffer.WriteString("value = ")
			}

			outputBuffer.WriteString(strings.Trim(annotationValuePiece, " "))
			outputBuffer.WriteString(",\n        ")
		}
	}

	return strings.TrimRight(outputBuffer.String(), "\n ") + "\n    "
}

func javaFieldName(n rdl.Identifier) string {
	if n == "default" {
		return "_default"
	}
	return string(n)
}
