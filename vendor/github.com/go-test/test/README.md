test
====

This package contains helper functions for Go testing. The functions are meant to be imported into the default namespace:

```go
import(
	. "github.com/go-test/test"
)
```

IsDeeply
--------

Like `reflect.DeepEqual` but returns where and what the difference is:

```go
if same, diff := IsDeeply(got, expect); !same {
	Dump(got)
	t.Error(diff)
}
```

RootDir
-------

Returns the absolute path of the current repo by looking for `.git/`:

```go
sample := paht.Join(RootDir(), "test", "samples")
```
