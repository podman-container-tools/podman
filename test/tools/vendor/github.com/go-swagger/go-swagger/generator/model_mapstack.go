// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"github.com/go-openapi/analysis"
	"github.com/go-openapi/spec"
)

type mapStack struct {
	Type     *spec.Schema
	Next     *mapStack
	Previous *mapStack
	ValueRef *schemaGenContext
	Context  *schemaGenContext
	NewObj   *schemaGenContext
}

func newMapStack(context *schemaGenContext) (first, last *mapStack, err error) {
	ms := &mapStack{
		Type:    &context.Schema,
		Context: context,
	}

	l := ms
	for l.HasMore() {
		tpe, err := l.Context.TypeResolver.ResolveSchema(l.Type.AdditionalProperties.Schema, true, true)
		if err != nil {
			return nil, nil, err
		}

		if !tpe.IsMap {
			// reached the end of the rabbit hole
			if tpe.IsComplexObject && tpe.IsAnonymous {
				// found an anonymous object: create the struct from a newly created definition
				nw := l.Context.makeNewStruct(l.Context.makeRefName()+" Anon", *l.Type.AdditionalProperties.Schema)
				sch := spec.RefProperty("#/definitions/" + nw.Name)
				l.NewObj = nw

				l.Type.AdditionalProperties.Schema = sch
				l.ValueRef = l.Context.NewAdditionalProperty(*sch)
			}

			// other cases where to stop are: a $ref or a simple object
			break
		}

		// continue digging for maps
		l.Next = &mapStack{
			Previous: l,
			Type:     l.Type.AdditionalProperties.Schema,
			Context:  l.Context.NewAdditionalProperty(*l.Type.AdditionalProperties.Schema),
		}
		l = l.Next
	}

	// return top and bottom entries of this stack of AdditionalProperties
	return ms, l, nil
}

// Build rewinds the stack of additional properties, building schemas from bottom to top.
//
//nolint:gocognit,gocyclo,cyclop // TODO(fredbi): refactor
func (mt *mapStack) Build() error {
	if mt.NewObj == nil && mt.ValueRef == nil && mt.Next == nil && mt.Previous == nil {
		csch := mt.Type.AdditionalProperties.Schema
		cp := mt.Context.NewAdditionalProperty(*csch)
		d := mt.Context.TypeResolver.Doc

		asch, err := analysis.Schema(analysis.SchemaOpts{
			Root:     d.Spec(),
			BasePath: d.SpecFilePath(),
			Schema:   csch,
		})
		if err != nil {
			return err
		}
		cp.Required = !asch.IsSimpleSchema && !asch.IsMap

		// when the schema is an array or an alias, this may result in inconsistent
		// nullable status between the map element and the array element (resp. the aliased type).
		//
		// Example: when an object has no property and only additionalProperties,
		// which turn out to be arrays of some other object.

		// save the initial override
		hadOverride := cp.GenSchema.IsMapNullOverride
		if err := cp.makeGenSchema(); err != nil {
			return err
		}

		// if we have an override at the top of stack, propagates it down nested arrays
		if hadOverride && cp.GenSchema.IsArray {
			// do it for nested arrays: override is also about map[string][][]... constructs
			it := &cp.GenSchema
			for it.Items != nil && it.IsArray {
				it.Items.IsMapNullOverride = hadOverride
				it = it.Items
			}
		}
		// cover other cases than arrays (aliased types)
		cp.GenSchema.IsMapNullOverride = hadOverride

		mt.Context.MergeResult(cp, false)
		mt.Context.GenSchema.AdditionalProperties = &cp.GenSchema

		// lift validations
		if (csch.Ref.String() != "" || cp.GenSchema.IsAliased) && !cp.GenSchema.IsInterface && !cp.GenSchema.IsStream {
			// - we stopped on a ref, or anything else that require we call its Validate() method
			// - if the alias / ref is on an interface (or stream) type: no validation
			mt.Context.GenSchema.HasValidations = true
			mt.Context.GenSchema.AdditionalProperties.HasValidations = true
		}

		debugLogf("early mapstack exit, nullable: %t for %s", cp.GenSchema.IsNullable, cp.GenSchema.Name)
		return nil
	}
	cur := mt
	for cur != nil {
		if cur.NewObj != nil {
			// a new model has been created during the stack construction (new ref on anonymous object)
			if err := cur.NewObj.makeGenSchema(); err != nil {
				return err
			}
		}

		if cur.ValueRef != nil {
			if err := cur.ValueRef.makeGenSchema(); err != nil {
				return err
			}
		}

		if cur.NewObj != nil {
			// newly created model from anonymous object is declared as extra schema
			cur.Context.MergeResult(cur.NewObj, false)

			// propagates extra schemas
			cur.Context.ExtraSchemas[cur.NewObj.Name] = cur.NewObj.GenSchema
		}

		if cur.ValueRef != nil {
			// this is the genSchema for this new anonymous AdditionalProperty
			if err := cur.Context.makeGenSchema(); err != nil {
				return err
			}

			// if there is a ValueRef, we must have a NewObj (from newMapStack() construction)
			cur.ValueRef.GenSchema.HasValidations = cur.NewObj.GenSchema.HasValidations
			cur.Context.MergeResult(cur.ValueRef, false)
			cur.Context.GenSchema.AdditionalProperties = &cur.ValueRef.GenSchema
		}

		if cur.Previous != nil {
			// we have a parent schema: build a schema for current AdditionalProperties
			if err := cur.Context.makeGenSchema(); err != nil {
				return err
			}
		}
		if cur.Next != nil {
			// we previously made a child schema: lifts things from that one
			// - Required is not lifted (in a cascade of maps, only the last element is actually checked for Required)
			cur.Context.MergeResult(cur.Next.Context, false)
			cur.Context.GenSchema.AdditionalProperties = &cur.Next.Context.GenSchema

			// lift validations
			c := &cur.Next.Context.GenSchema
			if (cur.Next.Context.Schema.Ref.String() != "" || c.IsAliased) && !c.IsInterface && !c.IsStream {
				// - we stopped on a ref, or anything else that require we call its Validate()
				// - if the alias / ref is on an interface (or stream) type: no validation
				cur.Context.GenSchema.HasValidations = true
				cur.Context.GenSchema.AdditionalProperties.HasValidations = true
			}
		}
		if cur.ValueRef != nil {
			cur.Context.MergeResult(cur.ValueRef, false)
			cur.Context.GenSchema.AdditionalProperties = &cur.ValueRef.GenSchema
		}

		if cur.Context.GenSchema.AdditionalProperties != nil {
			// propagate overrides up the resolved schemas, but leaves any ExtraSchema untouched
			cur.Context.GenSchema.AdditionalProperties.IsMapNullOverride = cur.Context.GenSchema.IsMapNullOverride
		}
		cur = cur.Previous
	}

	return nil
}

func (mt *mapStack) HasMore() bool {
	return mt.Type.AdditionalProperties != nil && (mt.Type.AdditionalProperties.Schema != nil || mt.Type.AdditionalProperties.Allows)
}

/* currently unused:
func (mt *mapStack) Dict() map[string]any {
	res := make(map[string]any)
	res["context"] = mt.Context.Schema
	if mt.Next != nil {
		res["next"] = mt.Next.Dict()
	}
	if mt.NewObj != nil {
		res["obj"] = mt.NewObj.Schema
	}
	if mt.ValueRef != nil {
		res["value"] = mt.ValueRef.Schema
	}
	return res
}
*/
