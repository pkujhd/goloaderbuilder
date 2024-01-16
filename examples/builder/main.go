package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkujhd/goloader"
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

	importPkgs := make(map[string]bool)
	for _, importPkg := range pkg.Imports {
		importPkgs[importPkg] = false
	}

	maxDepth := 128
	depth := 1
	for {
		if len(unresolvedSymbols) == 0 || depth > maxDepth {
			break
		}
		err = buildDepPackage(linker, config, unresolvedSymbols, importPkgs)
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
	writer := io.Writer(f)
	err = goloader.Serialize(linker, writer)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func buildDepPackage(linker *goloader.Linker, config *goloaderbuilder.BuildConfig, unresolvedSymbols []string, importPkgs map[string]bool) error {
	for importPkg, dealed := range importPkgs {
		if dealed == false {
			symbolInPkg := false
			for _, symbol := range unresolvedSymbols {
				if strings.Contains(symbol, importPkg) {
					symbolInPkg = true
				}
			}
			if symbolInPkg {
				conf := *config
				conf.PkgPath = importPkg
				conf.BuildPaths = []string{importPkg}
				pkg, err := goloaderbuilder.BuildDepPackage(&conf)
				if err != nil {
					return err
				}
				err = linker.ReadDependPkg(conf.TargetPath, importPkg, unresolvedSymbols)
				if err != nil {
					return err
				}
				for _, importPkg := range pkg.Imports {
					if _, ok := importPkgs[importPkg]; !ok {
						importPkgs[importPkg] = false
					}
				}
			}
		}
	}
	return nil
}
