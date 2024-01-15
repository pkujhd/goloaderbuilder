
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

build go files for goloader
```
cd examples/builder
./builder -e ../runner/runner -f ../../../goloader/examples/inter/inter.go
../runner/runner -f target/main.goloader
```
