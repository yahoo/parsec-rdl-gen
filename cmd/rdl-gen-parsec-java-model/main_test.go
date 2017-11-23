package main

import (
	"strings"
	"testing"

	"github.com/ardielle/ardielle-go/rdl"
	"github.com/stretchr/testify/assert"
)

var (
	schema = &rdl.Schema{
		Types: []*rdl.Type{
			&rdl.Type{
				Variant: rdl.TypeVariantStringTypeDef,
				StringTypeDef: &rdl.StringTypeDef{
					Type:    "Type",
					Name:    "Type",
					Comment: "Comment",
					Annotations: map[rdl.ExtendedAnnotation]string{
						"x_min": "groups=create|update",
					},
				},
			},
		},
	}
	registry = rdl.NewTypeRegistry(schema)
)

func TestGenerateStructFields(t *testing.T) {
	senarios := []struct {
		// input
		fields         []*rdl.StructFieldDef
		name           rdl.TypeName
		comment        string
		cName          string
		annotations    map[rdl.ExtendedAnnotation]string
		genAnnotations bool
		// output
		body    []string
		imports []string
	}{
		{},
	}
	for _, senario := range senarios {
		gen := javaModelGenerator{}
		gen.generateStructFields(senario.fields, senario.name, senario.comment,
			senario.cName, senario.annotations, senario.genAnnotations)
		assert.Equal(t, gen.body, senario.body)
		assert.Equal(t, gen.imports, senario.imports)
	}
}

func TestGenerateStructFieldType(t *testing.T) {
	senarios := []struct {
		// input
		rdlType  rdl.TypeRef
		optional bool
		items    rdl.TypeRef
		keys     rdl.TypeRef
		// output
		body    string
		imports string
	}{
		{"Type", true, "", "", "Type", ""},
		{"array", true, "Type", "", `List<
    @Min(
        groups = {
            ParsecValidationGroups.Create.class,
            ParsecValidationGroups.Update.class
        }
    )
        Type>`, "import javax.validation.constraints.Min;\n"},
		{"map", true, "Type", "Type", `Map<
    @Min(
        groups = {
            ParsecValidationGroups.Create.class,
            ParsecValidationGroups.Update.class
        }
    )
        Type, 
    @Min(
        groups = {
            ParsecValidationGroups.Create.class,
            ParsecValidationGroups.Update.class
        }
    )
        Type>`, "import javax.validation.constraints.Min;\n"},
	}
	for _, senario := range senarios {
		validationGroups = make(map[string]struct{}, 0)
		gen := javaModelGenerator{
			schema:   schema,
			registry: registry,
		}
		gen.generateStructFieldType(senario.rdlType, senario.optional, senario.items, senario.keys)
		assert.Equal(t, strings.Join(gen.body, ""), senario.body)
		assert.Equal(t, strings.Join(gen.imports, ""), senario.imports)
	}
}

func TestGenerateStructFieldParamType(t *testing.T) {
	senarios := []struct {
		// input
		rdlType  rdl.TypeRef
		optional bool
		items    rdl.TypeRef
		keys     rdl.TypeRef
		// output
		body    string
		imports string
	}{
		{"Type", true, "", "", `
    @Min(
        groups = {
            ParsecValidationGroups.Create.class,
            ParsecValidationGroups.Update.class
        }
    )
        Type`, "import javax.validation.constraints.Min;\n"},
		{"array", true, "Type", "", `List<
    @Min(
        groups = {
            ParsecValidationGroups.Create.class,
            ParsecValidationGroups.Update.class
        }
    )
        Type>`, "import javax.validation.constraints.Min;\n"},
		{"map", true, "Type", "Type", `Map<
    @Min(
        groups = {
            ParsecValidationGroups.Create.class,
            ParsecValidationGroups.Update.class
        }
    )
        Type, 
    @Min(
        groups = {
            ParsecValidationGroups.Create.class,
            ParsecValidationGroups.Update.class
        }
    )
        Type>`, "import javax.validation.constraints.Min;\n"},
	}
	for _, senario := range senarios {
		validationGroups = make(map[string]struct{}, 0)
		gen := javaModelGenerator{
			schema:   schema,
			registry: registry,
		}
		gen.generateStructFieldParamType(senario.rdlType, senario.optional, senario.items, senario.keys)
		assert.Equal(t, strings.Join(gen.body, ""), senario.body)
		assert.Equal(t, strings.Join(gen.imports, ""), senario.imports)
	}
}

func TestGenerateValidationGroupAnnotation(t *testing.T) {
	senarios := []struct {
		// input
		key   rdl.ExtendedAnnotation
		value string
		// output
		body    string
		imports string
	}{
		// keys
		{"x_min", "value", "@Min(value)", "import javax.validation.constraints.Min;\n"},
		{"x_max", "value", "@Max(value)", "import javax.validation.constraints.Max;\n"},
		{"x_size", "value", "@Size(value)", "import javax.validation.constraints.Size;\n"},
		{"x_pattern", "value", "@Pattern(value)", "import javax.validation.constraints.Pattern;\n"},
		{"x_must_validate", "value", "@Valid", "import javax.validation.Valid;\n"},
		{"x_name", "value", "@XmlElement(name=\"value\")", "import javax.xml.bind.annotation.XmlElement;\n"},
		{"x_not_null", "value", "@NotNull(value)", "import javax.validation.constraints.NotNull;\n"},
		{"x_null", "value", "@Null(value)", "import javax.validation.constraints.Null;\n"},
		{"x_not_blank", "value", "@NotBlank(value)", "import org.hibernate.validator.constraints.NotBlank;\n"},
		{"x_not_empty", "value", "@NotEmpty(value)", "import org.hibernate.validator.constraints.NotEmpty;\n"},
		{"x_country_code", "value", "@CountryCode", "import com.yahoo.parsec.constraint.validators.CountryCode;\n"},
		{"x_currency", "value", "@ValidCurrency", "import com.yahoo.parsec.constraint.validators.ValidCurrency;\n"},
		{"x_language_tag", "value", "@LanguageTag", "import com.yahoo.parsec.constraint.validators.LanguageTag;\n"},
		{"x_date_time", "value", "@DateTime", "import com.yahoo.parsec.constraint.validators.DateTime;\n"},
		{"x_digits", "value", "@Digits(value)", "import javax.validation.constraints.Digits;\n"},
		{"x_adapter", "value", "@XmlJavaTypeAdapter(value)", "import javax.xml.bind.annotation.adapters.XmlJavaTypeAdapter;\n"},
		// groups
		{"x_min", "groups=create|update", `@Min(
        groups = {
            ParsecValidationGroups.Create.class,
            ParsecValidationGroups.Update.class
        }
    )`, "import javax.validation.constraints.Min;\n"},
	}
	for _, senario := range senarios {
		validationGroups = make(map[string]struct{}, 0)
		gen := javaModelGenerator{}
		gen.generateValidationGroupAnnotation(senario.key, senario.value)
		assert.Equal(t, strings.Join(gen.body, ""), senario.body)
		assert.Equal(t, strings.Join(gen.imports, ""), senario.imports)
	}
}

func TestGenerateStructFieldGetterAnnotations(t *testing.T) {
	annotations := map[rdl.ExtendedAnnotation]string{
		"x_name": "value",
	}
	gen := javaModelGenerator{}
	gen.generateStructFieldGetterAnnotations(annotations)
	assert.Equal(t, strings.Join(gen.body, ""),
		"\n    @XmlElement(name=\"value\")\n")
	assert.Equal(t, strings.Join(gen.imports, ""),
		"import javax.xml.bind.annotation.XmlElement;\n")
}

func TestGenerateStructFieldSetterAnnotations(t *testing.T) {
	annotations := map[rdl.ExtendedAnnotation]string{
		"x_name": "value",
	}
	gen := javaModelGenerator{}
	gen.generateStructFieldSetterAnnotations(annotations)
	assert.Equal(t, strings.Join(gen.body, ""),
		"\n    @XmlElement(name=\"value\")\n")
	assert.Equal(t, strings.Join(gen.imports, ""),
		"import javax.xml.bind.annotation.XmlElement;\n")
}

func TestAppendAnnotation(t *testing.T) {
	gen := javaModelGenerator{}
	gen.appendAnnotation("key", "value")
	assert.Equal(t, strings.Join(gen.body, ""), "key(value)")
}
