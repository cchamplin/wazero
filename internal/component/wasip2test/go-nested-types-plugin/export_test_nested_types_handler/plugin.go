package export_test_nested_types_handler

import (
	"github.com/bytecodealliance/wit-bindgen/wit_types"
	"wit_component/test_nested_types_store"
	"wit_component/test_nested_types_types"
)

func Lookup(id uint32) wit_types.Option[test_nested_types_types.Item] {
	return test_nested_types_store.GetItem(id)
}

func Create(name string) wit_types.Result[test_nested_types_types.Item, string] {
	item := test_nested_types_types.Item{
		Id:   1,
		Name: name,
	}
	return wit_types.Ok[test_nested_types_types.Item, string](item)
}
