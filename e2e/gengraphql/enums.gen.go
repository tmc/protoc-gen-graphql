// Code generated by github.com/tmc/protoc-gen-graphql, DO NOT EDIT.

package gengraphql

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/tmc/protoc-gen-graphql/e2e"
	"github.com/vektah/gqlparser/v2/ast"
)

func (ec *executionContext) _TrafficLight(ctx context.Context, sel ast.SelectionSet, v *e2e.TrafficLight) graphql.Marshaler {
	return graphql.MarshalString((*v).String())
}

func (ec *executionContext) unmarshalInputTrafficLight(ctx context.Context, v interface{}) (e2e.TrafficLight, error) {
	switch v := v.(type) {
	case string:
		intValue, ok := e2e.TrafficLight_value[v]
		if !ok {
			return 0, errors.New("unknown value: " + v)
		}
		return e2e.TrafficLight(intValue), nil
	}
	return 0, errors.New("wrong type")
}
