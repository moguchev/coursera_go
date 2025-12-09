package main

import (
	"fmt"
	"reflect"
)

func setFields(in map[string]interface{}, out interface{}) error {
	reflectOutValue := reflect.ValueOf(out).Elem()

	for fieldName, value := range in {
		field, ok := reflectOutValue.Type().FieldByName(fieldName)
		if !ok {
			continue
		}
		fieldOfValue := reflectOutValue.FieldByName(fieldName)

		sourceType := reflect.ValueOf(value).Type().Kind()
		destinationType := field.Type.Kind()

		var err error
		switch destinationType {
		case reflect.Map, reflect.Slice:
			err = i2s(value, fieldOfValue.Addr().Interface())
		case reflect.Int:
			if sourceType == reflect.Float64 {
				fieldOfValue.Set(reflect.ValueOf(int(value.(float64))))
			} else if sourceType == reflect.Int64 {
				fieldOfValue.Set(reflect.ValueOf(value.(int64)))
			} else {
				err = fmt.Errorf("can`t convert int")
			}
		case reflect.Struct:
			if sourceType == destinationType {
				fieldOfValue.Set(reflect.ValueOf(value))
			} else if sourceType == reflect.Map || sourceType == reflect.Slice {
				err = i2s(value, fieldOfValue.Addr().Interface())
			} else {
				err = fmt.Errorf("different structs` types: %T != %T", sourceType, destinationType)
			}
		case reflect.Bool:
			if sourceType == destinationType {
				fieldOfValue.Set(reflect.ValueOf(value))
			} else {
				err = fmt.Errorf("different  types: %T != %T", sourceType, destinationType)
			}
		case reflect.String:
			if sourceType == destinationType {
				fieldOfValue.Set(reflect.ValueOf(value))
			} else {
				err = fmt.Errorf("different  types: %T != %T", sourceType, destinationType)
			}
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func i2s(data interface{}, out interface{}) error {
	if reflect.ValueOf(out).Kind() != reflect.Ptr {
		return fmt.Errorf("out: %T - is not ptr type", out)
	}

	reflectDataType := reflect.TypeOf(data).Kind()
	reflectOutValue := reflect.ValueOf(out).Elem()

	switch reflectDataType {
	case reflect.Map:
		if reflectOutValue.Kind() != reflect.Struct {
			return fmt.Errorf("out: %T - is not struct", out)
		}
		return setFields(data.(map[string]interface{}), out)
	case reflect.Slice:
		if reflectOutValue.Kind() != reflect.Slice {
			return fmt.Errorf("out: %T - is not slice", out)
		}
		for _, value := range data.([]interface{}) {
			ptr := reflect.New(reflectOutValue.Type().Elem())
			i := ptr.Elem().Addr().Interface()

			err := i2s(value, i)
			if err != nil {
				return err
			}

			s := reflect.Append(reflectOutValue, reflect.ValueOf(i).Elem())
			reflectOutValue.Set(s)
		}
	default:
		return fmt.Errorf("data: %T - is not map or slice", data)
	}

	return nil
}
