deep-equal
==========

This package provides a single function: `deep.Equal`. It's like [reflect.DeepEqual](http://golang.org/pkg/reflect/#DeepEqual), but it retunrs a list of differences, which is helpful when testing the equality of complex types like structures and maps.

```go
import (
	"fmt"
	"github.com/daniel-nichter/deep-equal"
)

func main() {
	type T struct {
		Name    string
		Numbers []float64
	}
	t1 := T{
		Name:    "John",
		Numbers: []float64{1.1, 2.2, 3.0},
	}
	t2 := T{
		Name:    "Johnny",
		Numbers: []float64{1.1, 2.2, 3.3},
	}

	diff, err := deep.Equal(t1, t2)
	if err != nil {
		fmt.Println("ERROR:", err)
	}
	for _, d := range diff {
		fmt.Println(d)
	}
}
```

```
Name: John != Johnny
Numbers.slice[2]: 3.000000 != 3.300000
```

The code is alpha quality and does not handle all [reflect.Kind](http://golang.org/pkg/reflect/#Kind).
