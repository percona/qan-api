package test

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

var Debug = false

func IsDeeply(got, expect interface{}) (result bool, error string) {
	gotVal := reflect.ValueOf(got)
	expectVal := reflect.ValueOf(expect)
	loc = []string{}
	equal, err := deepEquals(gotVal, expectVal, 0)
	if !equal {
		return false, fmt.Sprintf("%s:\n%s", strings.Join(loc, "."), err)
	}
	return true, ""
}

// --------------------------------------------------------------------------

// Location of first inequality, if any
var loc []string

func push(name string) {
	loc = append(loc, name)
}

func pop() {
	if len(loc) > 0 {
		loc = loc[0 : len(loc)-1]
	}
}

func deepEquals(got reflect.Value, expect reflect.Value, level uint) (bool, string) {
	if level > 10 {
		return false, "deepEquals() recursion too deep"
	}

	if got.Kind() != expect.Kind() {
		return false, fmt.Sprintf("Got a %s, expected a %s", got.Kind(), expect.Kind())
	}

	if expect.Kind() == reflect.Ptr {
		expect = expect.Elem()
	}
	if got.Kind() == reflect.Ptr {
		got = got.Elem()
	}

	// Compare the expected and got values based on their type,
	// return immediate when a difference is found.
	switch expect.Kind() {
	case reflect.Struct:
		if Debug {
			fmt.Println("reflect.Struct")
		}

		// For each field (key) in the expected struct...
		for i := 0; i < expect.NumField(); i++ {
			if got.Type().Field(i).PkgPath != "" {
				continue // skip unexported field, e.g. s in &T{s string}
			}
			if Debug {
				fmt.Printf("%+v\n", got.Type().Field(i))
			}

			push(got.Type().Field(i).Name)

			gotVal := got.Field(i)
			expectVal := expect.Field(i)

			fieldType := got.Type().Field(i).Type.String()
			if fieldType == "time.Time" {
				gotT := &time.Time{}
				gotTptr := reflect.ValueOf(gotT).Elem()
				gotTptr.Set(gotVal)

				expectT := &time.Time{}
				expectTptr := reflect.ValueOf(expectT).Elem()
				expectTptr.Set(expectVal)

				if !gotT.Equal(*expectT) {
					return false, fmt.Sprintf("   got: %s\nexpect: %s", gotT, expectT)
				}
			}

			if equal, err := deepEquals(gotVal, expectVal, level+1); !equal {
				return false, err
			}
			pop()
		}
	case reflect.Map:
		if Debug {
			fmt.Println("reflect.Map")
		}

		// Get all keys in the expected map.  If there aren't any, check that
		// there also aren't any in the got map.
		keys := expect.MapKeys()
		if len(keys) == 0 {
			gotKeys := got.MapKeys()
			if len(gotKeys) != 0 {
				err := fmt.Sprintf("     got: %s values\nexpected: no %s values\n", gotKeys[0], gotKeys[0])
				return false, err
			}
			return true, "" // no keys or values in either map
		}

		// For key in the map, compare the got and expected values...
		for _, key := range keys {
			gotVal := got.MapIndex(key)
			expectVal := expect.MapIndex(key)
			push(fmt.Sprintf("map[%s]", key))
			if equals, err := deepEquals(gotVal, expectVal, level+1); !equals {
				return false, err
			}
			pop()
		}
	case reflect.Slice:
		if Debug {
			fmt.Println("reflect.Size")
		}

		if got.IsNil() != expect.IsNil() {
			return false, "One slice is not nil"
		}
		if got.Len() != expect.Len() {
			return false, "Slices have different lengths"
		}
		if got.Pointer() == expect.Pointer() {
			return true, ""
		}
		for i := 0; i < got.Len(); i++ {
			push(fmt.Sprintf("slice[%d]", i))
			if equals, err := deepEquals(got.Index(i), expect.Index(i), level+1); !equals {
				return false, err
			}
			pop()
		}
	default:
		if Debug {
			fmt.Println("primitive")
		}

		if equal, err := checkPrimitives(got, expect); !equal {
			return false, err
		}
	}

	// No differences; all the events are identical (or there's a bug in this func).
	return true, ""
}

func checkPrimitives(got reflect.Value, expect reflect.Value) (bool, string) {
	/*
	 * Check got.IsValid() first: this returns true if the value is defined.
	 * We know the expect value is defined because it's in the map we're iterating,
	 * but the got value may not be defined (i.e. is not "valid"--IsValid() is
	 * poorly named; IsDefined() would be better imho).  This avoids a panic like
	 * "called got.Float() on zero Value".
	 */
	switch expect.Kind() {
	case reflect.Float32, reflect.Float64:
		if got.IsValid() {
			// Avoid 0.04147685731961082 != 0.041476857319611
			// 6 decimal places is close enough
			g := fmt.Sprintf("%.6f", got.Float())
			e := fmt.Sprintf("%.6f", expect.Float())
			if g != e {
				err := fmt.Sprintf("     got: %s\nexpected: %s\n", g, e)
				return false, err
			}
		} else {
			err := fmt.Sprintf("     got: undef\nexpected: %f\n",
				expect.Float())
			return false, err
		}
	case reflect.Bool:
		if got.IsValid() {
			if got.Bool() != expect.Bool() {
				err := fmt.Sprintf("     got: %t\nexpected: %t\n",
					got.Bool(), expect.Bool())
				return false, err
			}
		} else {
			err := fmt.Sprintf("     got: undef\nexpected: %t\n",
				expect.Bool())
			return false, err
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if got.IsValid() {
			if got.Int() != expect.Int() {
				err := fmt.Sprintf("     got: %d\nexpected: %d\n",
					got.Int(), expect.Int())
				return false, err
			}
		} else {
			err := fmt.Sprintf("     got: undef\nexpected: %d\n",
				expect.Int())
			return false, err
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if got.IsValid() {
			if got.Uint() != expect.Uint() {
				err := fmt.Sprintf("     got: %d\nexpected: %d\n",
					got.Uint(), expect.Uint())
				return false, err
			}
		} else {
			err := fmt.Sprintf("     got: undef\nexpected: %d\n",
				expect.Uint())
			return false, err
		}
	case reflect.String:
		if got.IsValid() {
			if got.String() != expect.String() {
				err := fmt.Sprintf("     got: %s\nexpected: %s\n",
					got.String(), expect.String())
				return false, err
			}
		} else {
			err := fmt.Sprintf("     got: undef\nexpected: %s\n",
				expect.String())
			return false, err
		}
	case reflect.Invalid: // nil pointer
		// For example: T{x: *X} and T.x is nil.
		if got.IsValid() && !expect.IsValid() {
			err := fmt.Sprintf("     got: pointer\nexpected: nil\n")
			return false, err
		} else if !got.IsValid() && expect.IsValid() {
			err := fmt.Sprintf("     got: nil\nexpected: pointer\n")
			return false, err
		}
	case reflect.Interface:
		if got.IsValid() && !expect.IsValid() {
			err := fmt.Sprintf("     got: %s\nexpected: nil\n", got.String())
			return false, err
		} else if !got.IsValid() && expect.IsValid() {
			err := fmt.Sprintf("     got: nil\nexpected: %s\n", expect.String())
			return false, err
		}
	default:
		return false, fmt.Sprintf("checkPrimitives() cannot handle %s", expect.Kind())
	}
	return true, ""
}
