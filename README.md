
# Goloader Builder

build go files or go packages for goloader

## Examples
build examples
```
cd exampels/builder
go build
cd examples/runner
go build
```

### build go files for goloader
```
cd examples/builder
./builder -e ../runner/runner -f $GOPATH/src/github.com/pkujhd/goloader/examples/inter/inter.go -p inter
../runner/runner -f target/inter.goloader -r inter.main
```

### build go files for goloader
```
cd examples/builder
./builder -e ../runner/runner -f $GOPATH/src/github.com/pkujhd/goloader/examples/inter 
../runner/runner -f target/main.goloader -r github.com/pkujhd/goloader/examples/inter.main
```
