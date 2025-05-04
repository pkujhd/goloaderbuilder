package goloaderbuilder

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type BuildConfig struct {
	GoBinary        string   // path to go binary, defaults to "go"
	ExtraBuildFlags []string // build flags
	BuildEnv        []string // build env
	BuildPaths      []string // build path
	PkgPath         string   // package path
	TargetDir       string   // target directory path
	TargetPath      string   // output path, output is a library file
	WorkDir         string   // work directory
	KeepWorkDir     bool     // keep work directory
	DebugLog        bool     // output debug build log
	Dynlink         bool     // enable position independent code
}

func mergeBuildFlags(extraBuildFlags []string, dynlink bool) []string {
	gcFlags := []string{}
	if dynlink {
		gcFlags = append(gcFlags, "-dynlink")
	}
	buildFlags := []string{}
	for _, buildflag := range extraBuildFlags {
		if strings.HasPrefix(strings.TrimLeft(buildflag, " "), "-gcflags") {
			flagSet := flag.NewFlagSet("", flag.ContinueOnError)
			f := flagSet.String("gcflags", "", "")
			err := flagSet.Parse([]string{buildflag})
			if err != nil {
				panic(err)
			}
			gcFlags = append(gcFlags, *f)
		} else {
			buildFlags = append(buildFlags, buildflag)
		}
	}

	if len(gcFlags) > 0 {
		buildFlags = append(buildFlags, fmt.Sprintf(`-gcflags=%s`, strings.Join(gcFlags, " ")))
	}
	return buildFlags
}

func execBuild(config *BuildConfig, wg *sync.WaitGroup) {

	var args = []string{"build"}
	args = append(args, mergeBuildFlags(config.ExtraBuildFlags, config.Dynlink)...)
	args = append(args, "-o", config.TargetPath)
	args = append(args, config.BuildPaths...)

	if len(config.BuildPaths) == 1 {
		goPath := os.Getenv("GOPATH")
		if !strings.HasPrefix(config.BuildPaths[0], goPath) {
			if _, err := os.Stat(config.TargetPath); err == nil {
				if wg != nil {
					wg.Done()
				}
				return
			}
		}
	}

	cmd := exec.Command(config.GoBinary, args...)
	cmd.Dir = config.WorkDir
	cmd.Env = append(cmd.Env, config.BuildEnv...)

	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}

	cmd.Stdout = stdoutBuffer
	cmd.Stderr = stderrBuffer

	if err := cmd.Run(); err != nil {
		fmt.Println("could not build with cmd:\n'%s': %w.\nstdout:\n%s\nstderr:\n%s",
			strings.Join(cmd.Args, " "), err, stdoutBuffer, stderrBuffer)
	}

	if config.DebugLog && stdoutBuffer.Len() > 0 {
		fmt.Println(stdoutBuffer)
	}

	if wg != nil {
		wg.Done()
	}

}

func initConfig(config *BuildConfig, absPathEnable bool) error {
	if config.GoBinary == "" {
		config.GoBinary = "go"
	}

	if len(config.BuildPaths) == 0 {
		return fmt.Errorf("source file path is empty")
	}

	if absPathEnable {
		for i := range config.BuildPaths {
			path, err := filepath.Abs(config.BuildPaths[i])
			if err != nil {
				return fmt.Errorf("failed to get absolute path at %s: %w", config.BuildPaths[i], err)
			}
			config.BuildPaths[i] = path
		}
	}

	if config.WorkDir == `` {
		config.WorkDir = "."
	}
	path, err := filepath.Abs(config.WorkDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path at %s: %w", config.WorkDir, err)
	}
	config.WorkDir = path

	err = os.MkdirAll(config.WorkDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create new temp dir at %s: %w", config.WorkDir, err)
	}

	path, err = filepath.Abs(config.TargetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path at %s: %w", config.TargetDir, err)
	}
	config.TargetDir = path

	path = strings.TrimSuffix(config.BuildPaths[0], ".go")
	goPath := os.Getenv("GOPATH")
	if path == config.BuildPaths[0] {
		if strings.HasPrefix(path, goPath) {
			path = strings.TrimPrefix(path, filepath.Join(goPath, "src", ""))
			path = filepath.Join(config.TargetDir, path, "")
		} else {
			path = filepath.Join(config.TargetDir, config.PkgPath)
		}
	} else {
		if strings.HasPrefix(path, goPath) {
			path = strings.TrimPrefix(path, filepath.Join(goPath, "src", ""))
			path = filepath.Join(config.TargetDir, filepath.Dir(path))
		} else {
			path = filepath.Join(config.TargetDir, filepath.Base(path))
		}
	}
	config.TargetPath = path
	path, err = filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path at %s: %w", path, err)
	}
	err = os.MkdirAll(config.TargetPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create new temp dir at %s: %w", config.TargetPath, err)
	}
	config.TargetPath = filepath.Join(config.TargetPath, filepath.Base(config.TargetPath)) + ".a"
	return nil
}

func getPkg(goBinary, absPath, workDir, targetPath string) (*Package, error) {
	pkg, err := GoList(goBinary, absPath, workDir, targetPath)

	if err != nil {
		return nil, err
	}

	if len(pkg.DepsErrors) > 0 {
		err = GoModDownload(goBinary, workDir)
		if err != nil {
			return nil, err
		}
		err = GoGet(goBinary, workDir, workDir)
		if err != nil {
			return nil, err
		}
		pkg, err = GoList(goBinary, absPath, "", targetPath)
		if err != nil {
			return nil, err
		}
		if len(pkg.DepsErrors) > 0 {
			return nil, fmt.Errorf("could not resolve dependency errors after go mod download + go get: %s", pkg.DepsErrors[0].Err)
		}
	}
	return pkg, err
}

func BuildGoFiles(config *BuildConfig) (*Package, error) {
	if !config.KeepWorkDir {
		defer os.RemoveAll(config.WorkDir)
	}

	if err := initConfig(config, true); err != nil {
		return nil, err
	}

	absPath := config.BuildPaths[0]
	workDir := filepath.Dir(absPath)
	config.WorkDir = workDir

	pkg, err := getPkg(config.GoBinary, absPath, workDir, config.TargetPath)
	if err != nil {
		return nil, err
	}

	execBuild(config, nil)
	return pkg, nil
}

func BuildDepPackage(config *BuildConfig, wg *sync.WaitGroup) (*Package, error) {
	if err := initConfig(config, false); err != nil {
		return nil, err
	}
	if len(config.BuildPaths) != 1 {
		return nil, fmt.Errorf("invalid source package path")
	}

	pkg, err := getPkg(config.GoBinary, config.BuildPaths[0], config.WorkDir, config.TargetPath)
	if err != nil {
		return nil, err
	}

	wg.Add(1)
	go execBuild(config, wg)
	return pkg, nil
}

func BuildGoPackage(config *BuildConfig) (*Package, error) {
	if !config.KeepWorkDir {
		defer os.RemoveAll(config.WorkDir)
	}
	if err := initConfig(config, true); err != nil {
		return nil, err
	}
	if len(config.BuildPaths) != 1 {
		return nil, fmt.Errorf("invalid source package path")
	}
	absPath := config.BuildPaths[0]
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("could not stat path at %s: %w", absPath, err)
	}
	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("path at %s is not a directory", absPath)
	}

	pkg, err := getPkg(config.GoBinary, absPath, config.WorkDir, config.TargetPath)
	if err != nil {
		return nil, err
	}

	execBuild(config, nil)
	return pkg, nil
}
