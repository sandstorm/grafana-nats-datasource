package plugin

import (
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/itchyny/gojq"
	"github.com/sandstormmedia/nats/pkg/plugin/framestruct"
	"reflect"
)

func processViaGojq(v interface{}, jqExpression string) (*data.Frame, error) {
	query, err := gojq.Parse(jqExpression)
	if err != nil {
		return nil, err
	}
	iter := query.Run(v) // or query.RunWithContext

	// filled after 1st row. We use it to check that all rows have the same result format, as everything
	// else is not supported for columnar storage.
	var expectedResultKind reflect.Kind
	// filled after 1st row.
	var isMultiValued bool
	// filled if isMultiValue = true
	var multiValueRow []map[string]interface{}
	//nilValues := make(map[string][]*interface{})
	//typeOfColumn := make(map[string]reflect.Type)

	// filled if isMultiValue = false
	var singleValueField *data.Field

	i := 0
	for {
		i++
		v, ok := iter.Next()
		if !ok {
			// all elements consumed
			break
		}
		if err, ok := v.(error); ok {
			// error from gojq
			return nil, err
		}

		if expectedResultKind == 0 {
			// 1.a) first row -> detect type; and initialize data structures
			expectedResultKind = ensureValue(reflect.ValueOf(v)).Kind()

			isMultiValued = expectedResultKind == reflect.Map
			if !isMultiValued {
				// initialize the correct field type
				singleValueField, err = initializeFieldForResultKind(expectedResultKind)
				if err != nil {
					log.DefaultLogger.Error(fmt.Sprintf("could not initialize Field for result kind: %v: %s", expectedResultKind, err))
					return nil, err
				}
			}

			log.DefaultLogger.Debug(fmt.Sprintf("Detected type from first row: %v", expectedResultKind))
		} else {
			// 1.b) 2nd..last row: check that types match
			resultKindOfRow := ensureValue(reflect.ValueOf(v)).Kind()
			if expectedResultKind != resultKindOfRow {
				return nil, fmt.Errorf(`type mismatch in row %d. Detected from first row: %v. Actual in this row: %v`, i, expectedResultKind, resultKindOfRow)
			}
		}

		// 2) Add current element to data storage
		if isMultiValued {
			vMap := v.(map[string]interface{})
			/*for key, val := range vMap {
				// remove NIL values, as framestruct.ToDataFrame cannot infer the type then (if this occurs in the first row)
				if val == nil {
					delete(vMap, key)
					nilValues[key] = append(nilValues[key], &val)
					//reflect.Zero(reflect.TypeOf(val))
				} else {
					if typeOfColumn[key] == nil {
						typeOfColumn[key] = reflect.TypeOf(v)
					}
					if reflect.TypeOf(v) != typeOfColumn[key] {
						return nil, fmt.Errorf(`type mismatch in row %d and field %s. Detected from first row: %v. Actual in this row: %v`, i, key, typeOfColumn[key], reflect.TypeOf(v))
					}
				}
			}*/
			multiValueRow = append(multiValueRow, vMap)
		} else {
			singleValueField.Append(v)
		}
	}

	/*for key, siNilValues := range nilValues {
		for _, nilValue := range siNilValues {
			if typeOfColumn[key] != nil {
				str := "foo"
				*nilValue = str
				//*nilValue = reflect.Zero(typeOfColumn[key]).Interface()
			} else {
				// no type found AT ALL for the column, so we can pick any type - we take empty string.
				*nilValue = ""
			}
		}
	}*/

	// 3) return result
	if isMultiValued {
		// multi-value fields are auto-converted.
		return framestruct.ToDataFrame("response", multiValueRow)
	} else {
		// single-value fields are already a field at this point.
		frame := data.NewFrame("response")
		frame.Fields = append(frame.Fields, singleValueField)
		// TODO: frame.Meta.Channel ds/<DATASOURCE_UID>/<CUSTOM_PATH>.
		return frame, nil
	}
}

func initializeFieldForResultKind(kind reflect.Kind) (*data.Field, error) {
	if kind == reflect.Bool {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []bool{}), nil
	}
	if kind == reflect.Int {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []int{}), nil
	}
	if kind == reflect.Int8 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []int8{}), nil
	}
	if kind == reflect.Int16 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []int16{}), nil
	}
	if kind == reflect.Int32 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []int32{}), nil
	}
	if kind == reflect.Int64 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []int64{}), nil
	}
	if kind == reflect.Uint {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []uint{}), nil
	}
	if kind == reflect.Uint8 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []uint8{}), nil
	}
	if kind == reflect.Uint16 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []uint16{}), nil
	}
	if kind == reflect.Uint32 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []uint32{}), nil
	}
	if kind == reflect.Uint64 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []uint64{}), nil
	}
	// uintptr not needed -> all value types
	if kind == reflect.Float32 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []float32{}), nil
	}
	if kind == reflect.Float64 {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []float64{}), nil
	}
	// complex64 and complex128 not supported
	// array not supported here
	// Chan not supported here
	// Func not supported here
	// Interface not supported here
	// Map not supported here
	// Pointer not supported here
	// Slice not supported here
	// Slice not supported here
	if kind == reflect.String {
		return data.NewField(data.TimeSeriesValueFieldName, nil, []string{}), nil
	}
	// Struct not supported here
	// UnsafePointer not supported here

	return nil, fmt.Errorf("type %v not supported for single-value fields (initializeFieldForResultKind)", kind)
}

func ensureValue(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v
}
