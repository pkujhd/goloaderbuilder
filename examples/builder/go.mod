module github.com/pkujhd/goloaderbuilder/examples/builder

go 1.11

require (
	github.com/pkujhd/goloader v0.0.0-20240113094056-ff3a1e01ffcb
	github.com/pkujhd/goloaderbuilder v0.0.0-20240116021854-8b753530ada5
	golang.org/x/arch v0.12.0 // indirect
)

replace (
	github.com/pkujhd/goloader => ../../../goloader
	github.com/pkujhd/goloaderbuilder => ../../
)
