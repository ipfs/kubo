package commands

import "reflect"

const (
  Invalid = reflect.Invalid
  Bool = reflect.Bool
  Int = reflect.Int
  Uint = reflect.Uint
  Float = reflect.Float32
  String = reflect.String
)

// Option is used to specify a field that will be provided by a consumer
type Option struct {
  Names []string      // a list of unique names to
  Type reflect.Kind           // value must be this type
  //Default interface{} // the default value (ignored if `Required` is true)
  //Required bool       // whether or not the option must be provided
}
