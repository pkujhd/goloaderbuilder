package goloaderbuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

func GoModDownload(goCmd, workDir string, args ...string) error {
	dlCmd := exec.Command(goCmd, append([]string{"mod", "download"}, args...)...)
	dlCmd.Dir = workDir
	output, err := dlCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to go mod download %s: %s", args, output)
	}

	tidyCmd := exec.Command(goCmd, "mod", "tidy")
	output, err = tidyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to go mod tidy: %s", output)
	}
	return nil
}

func GoGet(goCmd, packagePath, workDir string) error {
	goGetCmd := exec.Command(goCmd, "get", packagePath)
	goGetCmd.Dir = workDir
	output, err := goGetCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to go get %s: %s", packagePath, output)
	}
	return nil
}

func GoListStd(goCmd string) map[string]struct{} {
	stdLibPkgs := map[string]struct{}{}
	cmd := exec.Command(goCmd, "list", "std")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("goloaderbuilder failed to list std packages: %s\n", err)
		return nil
	}
	for _, pkgName := range bytes.Split(output, []byte("\n")) {
		stdLibPkgs[string(pkgName)] = struct{}{}
	}
	return stdLibPkgs
}

func GoList(goCmd, absPath, workDir, targetPath string) (*Package, error) {
	goPath := os.Getenv("GOPATH")
	targetPath = strings.Trim(targetPath, ".a") + ".json"
	if !strings.HasPrefix(absPath, goPath) {
		if _, err := os.Stat(targetPath); err == nil {
			f, err := os.Open(targetPath)
			if err != nil {
				return nil, err
			}
			pkg := Package{}
			err = json.NewDecoder(io.Reader(f)).Decode(&pkg)
			if err != nil {
				return nil, fmt.Errorf("failed to decode response of 'go list -json %s': %w\n", absPath, err)
			}
			if len(pkg.GoFiles)+len(pkg.CgoFiles) == 0 {
				return nil, fmt.Errorf("no Go files found in directory %s", absPath)
			}
			return &pkg, nil
		}
	}

	golistCmd := exec.Command(goCmd, "list", "-json", absPath)
	golistCmd.Dir = workDir
	output, err := golistCmd.StdoutPipe()
	stdErrBuf := &bytes.Buffer{}
	golistCmd.Stderr = io.MultiWriter(os.Stderr, stdErrBuf)
	if err != nil {
		panic(err)
	}
	listDec := json.NewDecoder(output)
	err = golistCmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start 'go list -json %s': %w\nstderr:\n%s", absPath, err, stdErrBuf.String())
	}
	pkg := Package{}
	err = listDec.Decode(&pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response of 'go list -json %s': %w\nstderr:\n%s", absPath, err, stdErrBuf.String())
	}
	err = golistCmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to wait for 'go list -json %s': %w\nstderr:\n%s", absPath, err, stdErrBuf.String())
	}

	if len(pkg.GoFiles)+len(pkg.CgoFiles) == 0 {
		return nil, fmt.Errorf("no Go files found in directory %s", absPath)
	}

	f, err := os.Create(targetPath)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	writer := io.Writer(f)
	encoder := json.NewEncoder(writer)
	err = encoder.Encode(pkg)
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}
