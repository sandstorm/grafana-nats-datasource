package tamarin

import (
	"fmt"
	"github.com/cloudcmds/tamarin/object"
	"reflect"
)

// Make sure structs implement required interfaces
var (
	_ object.TypeConverter = (*bytesConverter)(nil)
)

type bytesConverter struct {
}

var (
	byteArrType = reflect.TypeOf([]byte(""))
)

// To converts a Tamarin object to a Go object.
func (b bytesConverter) To(obj object.Object) (interface{}, error) {
	s, ok := obj.(*object.String)
	if !ok {
		return nil, fmt.Errorf("type error: expected a string (got %v)", obj.Type())
	}
	return []byte(s.Value()), nil
}

// From converts a Go object to a Tamarin object.
func (b bytesConverter) From(obj interface{}) (object.Object, error) {
	return object.NewString(string(obj.([]byte))), nil
}

// Type that this TypeConverter is responsible for.
func (b bytesConverter) Type() reflect.Type {
	return byteArrType
}
