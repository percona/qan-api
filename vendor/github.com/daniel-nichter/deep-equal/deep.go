/*
	The MIT License (MIT)

	Copyright (c) 2015 Daniel Nichter

	Permission is hereby granted, free of charge, to any person obtaining a copy
	of this software and associated documentation files (the "Software"), to deal
	in the Software without restriction, including without limitation the rights
	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
	copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all
	copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
	SOFTWARE.
*/

// Package deep provides function deep.Equal which is like reflect.DeepEqual but
// retunrs a list of differences. This is helpful when comparing complex types
// like structures and maps.
package deep

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	FloatPrecision = 6
	MaxDiff        = 5
	MaxDepth       = 10
)

var (
	ErrMaxRecursion      = errors.New("recursed to MaxDepth")
	ErrTypeMismatch      = errors.New("variables are different reflect.Type")
	ErrKindMismatch      = errors.New("variables are different reflect.Kind")
	ErrNotHandled        = errors.New("cannot compare the reflect.Kind")
	ErrUnexpectedInvalid = errors.New("unexpected reflect.Invalid kind")
)

type cmp struct {
	diff        []string
	buff        []string
	floatFormat string
}

// Equal compares variables a and b, recursing into their structure up to
// MaxDepth levels deep, and returns a list of differences, or nil if there are
// none. Some differences may not be found if an error is also returned.
//
// If a type has an Equal method, like time.Equal, it is called to check for
// equality.
func Equal(a, b interface{}) ([]string, error) {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)
	c := &cmp{
		diff:        []string{},
		buff:        []string{},
		floatFormat: fmt.Sprintf("%%.%df", FloatPrecision),
	}
	err := c.equals(aVal, bVal, 0)
	if len(c.diff) == 0 {
		c.diff = nil
	}
	return c.diff, err
}

func (c *cmp) equals(a, b reflect.Value, level int) error {
	if level > MaxDepth {
		return ErrMaxRecursion
	}

	aType := a.Type()
	bType := b.Type()
	if aType != bType {
		c.saveDiff(aType, bType)
		return ErrTypeMismatch
	}

	aKind := a.Kind()
	bKind := b.Kind()
	if aKind == reflect.Ptr || aKind == reflect.Interface {
		a = a.Elem()
		aKind = a.Kind()
		if a.IsValid() {
			aType = a.Type()
		}
	}
	if bKind == reflect.Ptr || bKind == reflect.Interface {
		b = b.Elem()
		bKind = b.Kind()
		if b.IsValid() {
			bType = b.Type()
		}
	}

	// For example: T{x: *X} and T.x is nil.
	if !a.IsValid() || !b.IsValid() {
		if a.IsValid() && !b.IsValid() {
			c.saveDiff(aType, "<nil pointer>")
		} else if !a.IsValid() && b.IsValid() {
			c.saveDiff("<nil pointer>", bType)
		}
		return nil
	}

	if aKind != bKind { // shouldn't be possible
		c.saveDiff(aKind, bKind)
		return ErrKindMismatch
	}

	// Types with an Equal(), like time.Time.
	eqFunc := a.MethodByName("Equal")
	if eqFunc.IsValid() {
		retVals := eqFunc.Call([]reflect.Value{b})
		if !retVals[0].Bool() {
			c.saveDiff(a, b)
		}
		return nil
	}

	switch aKind {

	/////////////////////////////////////////////////////////////////////
	// Iterable kinds
	/////////////////////////////////////////////////////////////////////

	case reflect.Struct:
		/*
			The variables are structs like:
				type T struct {
					FirstName string
					LastName  string
				}
			Type = <pkg>.T, Kind = reflect.Struct

			Iterate through the fields (FirstName, LastName), recurse into their values.
		*/
		for i := 0; i < a.NumField(); i++ {
			if aType.Field(i).PkgPath != "" {
				continue // skip unexported field, e.g. s in type T struct {s string}
			}

			c.push(aType.Field(i).Name) // push field name to buff

			// Get the Value for each field, e.g. FirstName has Type = string,
			// Kind = reflect.String.
			af := a.Field(i)
			bf := b.Field(i)

			if err := c.equals(af, bf, level+1); err != nil { // recurse to compare the field values
				return err
			}

			c.pop() // pop field name from buff

			if len(c.diff) >= MaxDiff {
				break
			}
		}
	case reflect.Map:
		/*
			The variables are maps like:
				map[string]int{
					"foo": 1,
					"bar": 2,
				}
			Type = map[string]int, Kind = reflect.Map

			Or:
				type T map[string]int{}
			Type = <pkg>.T, Kind = reflect.Map

			Iterate through the map keys (foo, bar), recurse into their values.
		*/

		if a.IsNil() || b.IsNil() {
			if a.IsNil() && !b.IsNil() {
				c.saveDiff("<nil map>", b)
			} else if !a.IsNil() && b.IsNil() {
				c.saveDiff(a, "<nil map>")
			}
			return nil
		}

		if a.Pointer() == b.Pointer() {
			return nil
		}

		aKeys := a.MapKeys()
		bKeys := b.MapKeys()

		sharedKeys := map[string]bool{} // keys in a and b

		for _, key := range aKeys {
			c.push(fmt.Sprintf("map[%s]", key))
			sharedKeys[key.String()] = true

			aVal := a.MapIndex(key)
			bVal := b.MapIndex(key)
			if bVal.IsValid() {
				if err := c.equals(aVal, bVal, level+1); err != nil {
					return err
				}
			} else {
				c.saveDiff(aVal, "<does not have key>")
			}

			c.pop()

			if len(c.diff) >= MaxDiff {
				break
			}
		}

		for _, key := range bKeys {
			if _, ok := sharedKeys[key.String()]; ok {
				continue
			}
			c.push(fmt.Sprintf("map[%s]", key))
			c.saveDiff("<does not have key>", b.MapIndex(key))
			c.pop()
			if len(c.diff) >= MaxDiff {
				break
			}
		}
	case reflect.Slice:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() && !b.IsNil() {
				c.saveDiff("<nil slice>", b)
			} else if !a.IsNil() && b.IsNil() {
				c.saveDiff(a, "<nil slice>")
			}
			return nil
		}

		if a.Pointer() == b.Pointer() {
			return nil
		}

		aLen := a.Len()
		bLen := b.Len()
		n := aLen
		if bLen > aLen {
			n = bLen
		}
		for i := 0; i < n; i++ {
			c.push(fmt.Sprintf("slice[%d]", i))
			if i < aLen && i < bLen {
				if err := c.equals(a.Index(i), b.Index(i), level+1); err != nil {
					return err
				}
			} else if i < aLen {
				c.saveDiff(a.Index(i), "<no value>")
			} else {
				c.saveDiff("<no value>", b.Index(i))
			}
			c.pop()
			if len(c.diff) >= MaxDiff {
				break
			}
		}

	/////////////////////////////////////////////////////////////////////
	// Primitive kinds
	/////////////////////////////////////////////////////////////////////

	case reflect.Float32, reflect.Float64:
		// Avoid 0.04147685731961082 != 0.041476857319611
		// 6 decimal places is close enough
		aval := fmt.Sprintf(c.floatFormat, a.Float())
		bval := fmt.Sprintf(c.floatFormat, b.Float())
		if aval != bval {
			c.saveDiff(aval, bval)
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			c.saveDiff(a.Bool(), b.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			c.saveDiff(a.Int(), b.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if a.Uint() != b.Uint() {
			c.saveDiff(a.Uint(), b.Uint())
		}
	case reflect.String:
		if a.String() != b.String() {
			c.saveDiff(a.String(), b.String())
		}

	default:
		return ErrNotHandled
	}

	return nil
}

func (c *cmp) push(name string) {
	c.buff = append(c.buff, name)
}

func (c *cmp) pop() {
	if len(c.buff) > 0 {
		c.buff = c.buff[0 : len(c.buff)-1]
	}
}

func (c *cmp) saveDiff(aval, bval interface{}) {
	if len(c.buff) > 0 {
		varName := strings.Join(c.buff, ".")
		c.diff = append(c.diff, fmt.Sprintf("%s: %v != %v", varName, aval, bval))
	} else {
		c.diff = append(c.diff, fmt.Sprintf("%v != %v", aval, bval))
	}
}
