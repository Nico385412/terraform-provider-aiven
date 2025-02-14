//nolint:unused
package userconfig

import (
	"fmt"

	"github.com/dave/jennifer/jen"
	"golang.org/x/exp/maps"
)

// handlePrimitiveTypeProperty is a function that converts a primitive type property to a Terraform schema.
func handlePrimitiveTypeProperty(n string, p map[string]interface{}, t string, ireq bool) map[string]*jen.Statement {
	return map[string]*jen.Statement{n: jen.Values(convertPropertyToSchema(n, p, t, true, ireq))}
}

// handleObjectProperty is a function that converts an object type property to a Terraform schema.
func handleObjectProperty(
	n string,
	p map[string]interface{},
	t string,
	req map[string]struct{},
) (map[string]*jen.Statement, error) {
	pa, ok := p["properties"].(map[string]interface{})
	if !ok {
		it, ok := p["items"].(map[string]interface{})
		if ok {
			pa, ok = it["properties"].(map[string]interface{})
		}

		if !ok {
			return nil, fmt.Errorf("unable to get properties field: %#v", p)
		}
	}

	r := convertPropertyToSchema(n, p, t, true, false)

	pc, err := convertPropertiesToSchemaMap(pa, req)
	if err != nil {
		return nil, err
	}

	s := jen.Map(jen.String()).Op("*").Qual(SchemaPackage, "Schema").Values(pc)

	r[jen.Id("Elem")] = jen.Op("&").Qual(SchemaPackage, "Resource").Values(jen.Dict{
		jen.Id("Schema"): s,
	})

	// TODO: Check if we can access the schema via diff suppression function.
	r[jen.Id("DiffSuppressFunc")] = jen.Qual(SchemaUtilPackage, "EmptyObjectDiffSuppressFuncSkipArrays").Call(s)

	r[jen.Id("MaxItems")] = jen.Lit(1)

	return map[string]*jen.Statement{n: jen.Values(r)}, nil
}

// handleArrayOfPrimitiveTypeProperty is a function that converts an array of primitive type property to a Terraform
// schema.
func handleArrayOfPrimitiveTypeProperty(n string, t string) *jen.Statement {
	r := jen.Dict{
		jen.Id("Type"): jen.Qual(SchemaPackage, t),
	}

	if n == "ip_filter" {
		// TODO: Add ip_filter_object to this sanity check when DiffSuppressFunc is implemented for it.
		r[jen.Id("DiffSuppressFunc")] = jen.Qual(SchemaUtilPackage, "IPFilterValueDiffSuppressFunc")
	}

	return jen.Op("&").Qual(SchemaPackage, "Schema").Values(r)
}

// handleArrayOfAggregateTypeProperty is a function that converts an array of aggregate type property to a Terraform
// schema.
func handleArrayOfAggregateTypeProperty(ip map[string]interface{}, req map[string]struct{}) (*jen.Statement, error) {
	pc, err := convertPropertiesToSchemaMap(ip, req)
	if err != nil {
		return nil, err
	}

	return jen.Op("&").Qual(SchemaPackage, "Resource").Values(jen.Dict{
		jen.Id("Schema"): jen.Map(jen.String()).Op("*").Qual(SchemaPackage, "Schema").Values(pc),
	}), nil
}

// handleArrayProperty is a function that converts an array type property to a Terraform schema.
func handleArrayProperty(
	n string,
	p map[string]interface{},
	t string,
) (map[string]*jen.Statement, error) {
	ia, ok := p["items"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("items is not a map[string]interface{}: %#v", p)
	}

	var e *jen.Statement

	var tn, atn []string

	var err error

	oos, iof := ia["one_of"].([]interface{})
	if iof {
		var ct []string

		for _, v := range oos {
			va, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("one_of element is not a map[string]interface{}: %#v", v)
			}

			ct = append(ct, va["type"].(string))
		}

		tn, atn, err = TerraformTypes(ct)
		if err != nil {
			return nil, err
		}
	} else {
		tn, atn, err = TerraformTypes(SlicedString(ia["type"]))
		if err != nil {
			return nil, err
		}
	}

	r := make(map[string]*jen.Statement)

	for k, v := range tn {
		an := n

		if len(tn) > 1 {
			an = fmt.Sprintf("%s_%s", n, atn[k])

			// TODO: Remove with the next major version.
			if an == "ip_filter_string" {
				an = "ip_filter"
			}

			// TODO: Remove with the next major version.
			if an == "namespaces_string" {
				an = "namespaces"
			}
		}

		var ooia map[string]interface{}

		if iof {
			ooia, ok = oos[k].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("unable to convert one_of item to map[string]interface{}: %#v", oos[k])
			}
		}

		if isTerraformTypePrimitive(v) {
			e = handleArrayOfPrimitiveTypeProperty(an, v)
		} else {
			var ipa map[string]interface{}

			if iof {
				ipa, ok = ooia["properties"].(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf(
						"unable to convert one_of item properties to map[string]interface{}: %#v",
						ooia,
					)
				}
			} else {
				ipa, ok = ia["properties"].(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("could not find properties in an array of aggregate type: %#v", p)
				}
			}

			req := map[string]struct{}{}

			if sreq, ok := ia["required"].([]interface{}); ok {
				req = SliceToKeyedMap(sreq)
			}

			e, err = handleArrayOfAggregateTypeProperty(ipa, req)
			if err != nil {
				return nil, err
			}
		}

		s := convertPropertyToSchema(n, p, t, !iof, false)

		if iof {
			ooiat, ok := ooia["type"].(string)
			if !ok {
				return nil, fmt.Errorf("one_of item type is not a string: %#v", ooia)
			}

			_, dpv := descriptionForProperty(p, t)

			dooiid, dooid := descriptionForProperty(ooia, ooiat)

			s[jen.Id("Description")] = jen.Lit(fmt.Sprintf("%s %s", dpv, dooid))

			if dooiid {
				s[jen.Id("Deprecated")] = jen.Lit("Usage of this field is discouraged.")
			}
		}

		s[jen.Id("Elem")] = e

		if an == "ip_filter" {
			// TODO: Add ip_filter_object to this sanity check when DiffSuppressFunc is implemented for it.
			s[jen.Id("DiffSuppressFunc")] = jen.Qual(SchemaUtilPackage, "IPFilterArrayDiffSuppressFunc")
		}

		if mi, ok := p["max_items"].(int); ok {
			s[jen.Id("MaxItems")] = jen.Lit(mi)
		}

		os := jen.Dict{}
		for k, v := range s {
			os[k] = v
		}

		// TODO: Remove with the next major version.
		if an == "ip_filter" || (iof && an == "namespaces") {
			s[jen.Id("Deprecated")] = jen.Lit(
				fmt.Sprintf("This will be removed in v5.0.0 and replaced with %s_string instead.", an),
			)
		}

		r[an] = jen.Values(s)

		if an == "ip_filter" || (iof && an == "namespaces") {
			r[fmt.Sprintf("%s_string", an)] = jen.Values(os)
		}
	}

	return r, nil
}

// handleAggregateTypeProperty is a function that converts an aggregate type property to a Terraform schema.
func handleAggregateTypeProperty(
	n string,
	p map[string]interface{},
	t string,
	at string,
) (map[string]*jen.Statement, error) {
	r := make(map[string]*jen.Statement)

	req := map[string]struct{}{}

	if sreq, ok := p["required"].([]interface{}); ok {
		req = SliceToKeyedMap(sreq)
	}

	switch at {
	case "object":
		v, err := handleObjectProperty(n, p, t, req)
		if err != nil {
			return nil, err
		}

		maps.Copy(r, v)
	case "array":
		v, err := handleArrayProperty(n, p, t)
		if err != nil {
			return nil, err
		}

		maps.Copy(r, v)
	default:
		return nil, fmt.Errorf("unknown aggregate type: %s", at)
	}

	return r, nil
}
