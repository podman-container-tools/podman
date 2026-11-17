// untested sections: 5

package gexec

import (
	"errors"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/onsi/gomega/internal/gutil"
)

var (
	mu     sync.Mutex
	tmpDir string
)

/*
Build uses go build to compile the package at packagePath.  The resulting binary is saved off in a temporary directory.
A path pointing to this binary is returned.

Build uses the $GOPATH set in your environment. If $GOPATH is not set and you are using Go 1.8+,
it will use the default GOPATH instead.  It passes the variadic args on to `go build`.
*/
func Build(packagePath string, args ...string) (compiledPath string, err error) {
	return doBuild(build.Default.GOPATH, packagePath, nil, args...)
}

/*
BuildWithEnvironment is identical to Build but allows you to specify env vars to be set at build time.
*/
func BuildWithEnvironment(packagePath string, env []string, args ...string) (compiledPath string, err error) {
	return doBuild(build.Default.GOPATH, packagePath, env, args...)
}

/*
BuildIn is identical to Build but allows you to specify a custom $GOPATH (the first argument).
*/
func BuildIn(gopath string, packagePath string, args ...string) (compiledPath string, err error) {
	return doBuild(gopath, packagePath, nil, args...)
}

func doBuild(gopath, packagePath string, env []string, args ...string) (compiledPath string, err error) {
	executable, err := newExecutablePath(gopath, packagePath)
	if err != nil {
		return "", err
	}

	cmdArgs := append([]string{"build"}, args...)
	cmdArgs = append(cmdArgs, "-o", executable, packagePath)

	build := exec.Command("go", cmdArgs...)
	build.Env = replaceGoPath(os.Environ(), gopath)
	build.Env = append(build.Env, env...)

	output, err := build.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to build %s:\n\nError:\n%s\n\nOutput:\n%s", packagePath, err, string(output))
	}

	return executable, nil
}

/*
CompileTest uses go test to compile the test package at packagePath.  The resulting binary is saved off in a temporary directory.
A path pointing to this binary is returned.

CompileTest uses the $GOPATH set in your environment. If $GOPATH is not set and you are using Go 1.8+,
it will use the default GOPATH instead.  It passes the variadic args on to `go test`.

Deprecated: CompileTest makes GOPATH assumptions that don't translate well to the go modules world.
*/
func CompileTest(packagePath string, args ...string) (compiledPath string, err error) {
	return doCompileTest(build.Default.GOPATH, packagePath, nil, args...)
}

/*
GetAndCompileTest is identical to CompileTest but `go get` the package before compiling tests.

Deprecated: GetAndCompileTest makes GOPATH assumptions that don't translate well to the go modules world.
*/
func GetAndCompileTest(packagePath string, args ...string) (compiledPath string, err error) {
	if err := getForTest(build.Default.GOPATH, packagePath, []string{"GO111MODULE=off"}); err != nil {
		return "", err
	}

	return doCompileTest(build.Default.GOPATH, packagePath, []string{"GO111MODULE=off"}, args...)
}

/*
CompileTestWithEnvironment is identical to CompileTest but allows you to specify env vars to be set at build time.

Deprecated: CompileTestWithEnvironment makes GOPATH assumptions that don't translate well to the go modules world.
*/
func CompileTestWithEnvironment(packagePath string, env []string, args ...string) (compiledPath string, err error) {
	return doCompileTest(build.Default.GOPATH, packagePath, env, args...)
}

/*
GetAndCompileTestWithEnvironment is identical to GetAndCompileTest but allows you to specify env vars to be set at build time.

Deprecated: GetAndCompileTestWithEnvironment makes GOPATH assumptions that don't translate well to the go modules world.
*/
func GetAndCompileTestWithEnvironment(packagePath string, env []string, args ...string) (compiledPath string, err error) {
	if err := getForTest(build.Default.GOPATH, packagePath, append(env, "GO111MODULE=off")); err != nil {
		return "", err
	}

	return doCompileTest(build.Default.GOPATH, packagePath, append(env, "GO111MODULE=off"), args...)
}

/*
CompileTestIn is identical to CompileTest but allows you to specify a custom $GOPATH (the first argument).

Deprecated: CompileTestIn makes GOPATH assumptions that don't translate well to the go modules world.
*/
func CompileTestIn(gopath string, packagePath string, args ...string) (compiledPath string, err error) {
	return doCompileTest(gopath, packagePath, nil, args...)
}

/*
GetAndCompileTestIn is identical to GetAndCompileTest but allows you to specify a custom $GOPATH (the first argument).
*/
func GetAndCompileTestIn(gopath string, packagePath string, args ...string) (compiledPath string, err error) {
	if err := getForTest(gopath, packagePath, []string{"GO111MODULE=off"}); err != nil {
		return "", err
	}

	return doCompileTest(gopath, packagePath, []string{"GO111MODULE=off"}, args...)
}

func isLocalPackage(packagePath string) bool {
	return strings.HasPrefix(packagePath, ".")
}

func getForTest(gopath, packagePath string, env []string) error {
	if isLocalPackage(packagePath) {
		return nil
	}

	return doGet(gopath, packagePath, env, "-t")
}

func doGet(gopath, packagePath string, env []string, args ...string) error {
	args = append(args, packagePath)
	args = append([]string{"get"}, args...)

	goGet := exec.Command("go", args...)
	goGet.Dir = gopath
	goGet.Env = replaceGoPath(os.Environ(), gopath)
	goGet.Env = append(goGet.Env, env...)

	output, err := goGet.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to get %s:\n\nError:\n%s\n\nOutput:\n%s", packagePath, err, string(output))
	}

	return nil
}

func doCompileTest(gopath, packagePath string, env []string, args ...string) (compiledPath string, err error) {
	executable, err := newExecutablePath(gopath, packagePath, ".test")
	if err != nil {
		return "", err
	}

	cmdArgs := append([]string{"test", "-c"}, args...)
	cmdArgs = append(cmdArgs, "-o", executable, packagePath)

	build := exec.Command("go", cmdArgs...)
	build.Env = replaceGoPath(os.Environ(), gopath)
	build.Env = append(build.Env, env...)

	output, err := build.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to build %s:\n\nError:\n%s\n\nOutput:\n%s", packagePath, err, string(output))
	}

	return executable, nil
}

func replaceGoPath(environ []string, newGoPath string) []string {
	newEnviron := []string{}
	for _, v := range environ {
		if !strings.HasPrefix(v, "GOPATH=") {
			newEnviron = append(newEnviron, v)
		}
	}
	return append(newEnviron, "GOPATH="+newGoPath)
}

func newExecutablePath(gopath, packagePath string, suffixes ...string) (string, error) {
	tmpDir, err := temporaryDirectory()
	if err != nil {
		return "", err
	}

	if len(gopath) == 0 {
		return "", errors.New("$GOPATH not provided when building " + packagePath)
	}

	executable := filepath.Join(tmpDir, path.Base(packagePath))

	if runtime.GOOS == "windows" {
		executable += ".exe"
	}

	return executable, nil
}

/*
You should call CleanupBuildArtifacts before your test ends to clean up any temporary artifacts generated by
gexec. In Ginkgo this is typically done in an AfterSuite callback.
*/
func CleanupBuildArtifacts() {
	mu.Lock()
	defer mu.Unlock()
	if tmpDir != "" {
		os.RemoveAll(tmpDir)
		tmpDir = ""
	}
}

func temporaryDirectory() (string, error) {
	var err error
	mu.Lock()
	defer mu.Unlock()
	if tmpDir == "" {
		tmpDir, err = gutil.MkdirTemp("", "gexec_artifacts")
		if err != nil {
			return "", err
		}
	}

	return gutil.MkdirTemp(tmpDir, "g")
}
