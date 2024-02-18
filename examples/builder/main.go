package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkujhd/goloader"
	"github.com/pkujhd/goloader/obj"
	"github.com/pkujhd/goloaderbuilder"
)

type stringArrFlags struct {
	Data []string
}

func (i *stringArrFlags) String() string {
	return ``
}

func (i *stringArrFlags) Set(value string) error {
	i.Data = append(i.Data, value)
	return nil
}

func main() {
	var exeFile = flag.String("e", "", "exe file")
	var files stringArrFlags
	flag.Var(&files, "f", "load go object file or go package")
	var buildEnvs stringArrFlags
	flag.Var(&buildEnvs, "env", "load go object file or go package")
	var debug = flag.Bool("d", false, "debug log enable")
	var dynlink = flag.Bool("l", false, "dynlink enable")
	var keepWorkDir = flag.Bool("k", false, "keep work dir enable")
	var workDir = flag.String("w", "./tmp", "build work dir")
	var targetDir = flag.String("t", "./target", "build target dir")
	var pkgPath = flag.String("p", "main", "package path")
	var goBinaryPath = flag.String("g", "go", "go binary path")
	var onlyBuild = flag.Bool("b", false, "only build objfile")

	flag.Parse()

	config := goloaderbuilder.BuildConfig{}
	config.GoBinary = *goBinaryPath
	config.BuildEnv = append(config.BuildEnv, buildEnvs.Data...)
	config.KeepWorkDir = *keepWorkDir
	config.DebugLog = *debug
	config.WorkDir = *workDir
	config.BuildPaths = files.Data
	config.Dynlink = *dynlink
	config.PkgPath = *pkgPath
	config.TargetDir = *targetDir
	err := build(&config, *exeFile, *onlyBuild)
	if err != nil {
		fmt.Printf("build failed! error:%s\n", err)
	}
}

func build(config *goloaderbuilder.BuildConfig, exeFile string, onlyBuild bool) error {
	if len(config.BuildPaths) == 0 {
		return fmt.Errorf("empty buildPath!\n")
	}
	var pkg *goloaderbuilder.Package
	var err error
	if strings.HasSuffix(config.BuildPaths[0], ".go") {
		pkg, err = goloaderbuilder.BuildGoFiles(config)
	} else {
		pkg, err = goloaderbuilder.BuildGoPackage(config)
	}

	if err != nil {
		return err
	}
	if onlyBuild {
		return nil
	}
	symPtr := make(map[string]uintptr)
	err = goloader.RegSymbolWithPath(symPtr, exeFile)
	if err != nil {
		return err
	}
	linker, err := goloader.ReadObj(config.TargetPath, config.PkgPath)
	if err != nil {
		return err
	}
	unresolvedSymbols := goloader.UnresolvedSymbols(linker, symPtr)

	files := make([]string, 0)
	pkgPaths := make([]string, 0)
	if err = buildDepPackage(&files, &pkgPaths, pkg.Imports, config); err != nil {
		return err
	}

	maxDepth := 128
	depth := 1
	for {
		if len(unresolvedSymbols) == 0 || depth > maxDepth {
			break
		}
		err = linker.ReadDependPkgs(files, pkgPaths, unresolvedSymbols, symPtr)
		if err != nil {
			return err
		}
		unresolvedSymbols = goloader.UnresolvedSymbols(linker, symPtr)
		depth = depth + 1
	}

	if len(unresolvedSymbols) > 0 {
		return fmt.Errorf("unresovled symbols:%v", unresolvedSymbols)
	}

	if err = searilzeLinker(config, linker); err != nil {
		return err
	}

	return nil
}

func searilzeLinker(config *goloaderbuilder.BuildConfig, linker *goloader.Linker) error {
	serializeFilePath := config.TargetDir + "/" + config.PkgPath + ".goloader"
	f, err := os.Create(serializeFilePath)
	if err != nil {
		return err
	}
	//clear pkg.Syms, it's too big.
	for _, pkg := range linker.Packages {
		pkg.Syms = make(map[string]*obj.ObjSymbol, 0)
	}
	writer := io.Writer(f)
	err = goloader.Serialize(linker, writer)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func buildDepPackage(files, pkgPaths *[]string, imports []string, config *goloaderbuilder.BuildConfig) error {
	importPkgs := make(map[string]bool)
	importPkgs["unsafe"] = true
	addImport := func(importPkgs map[string]bool, imports []string) {
		for _, importPkg := range imports {
			if importPkg == "C" {
				importPkg = "runtime/cgo"
			}
			if _, ok := importPkgs[importPkg]; !ok {
				importPkgs[importPkg] = false
			}
		}
	}
	addImport(importPkgs, imports)

	fileLen := 0
	for fileLen != len(*files) || fileLen == 0 {
		fileLen = len(*files)
		for importPkg, dealed := range importPkgs {
			if dealed == false {
				conf := *config
				conf.PkgPath = importPkg
				conf.BuildPaths = []string{importPkg}
				pkg, err := goloaderbuilder.BuildDepPackage(&conf)
				if err != nil {
					return err
				}
				*files = append(*files, conf.TargetPath)
				*pkgPaths = append(*pkgPaths, importPkg)
				importPkgs[importPkg] = true
				addImport(importPkgs, pkg.Imports)
			}
		}
	}
	return nil
}
