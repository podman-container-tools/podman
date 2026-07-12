//go:build linux || freebsd

package integration

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/mattn/go-shellwords"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/podman/v6/version"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

type quadletTestcase struct {
	data        []byte
	serviceName string
	checks      [][]string
}

// Converts "foo@bar.container" to "foo@.container"
func getGenericTemplateFile(fileName string) (bool, string) {
	extension := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, extension)
	parts := strings.SplitN(base, "@", 2)
	if len(parts) == 2 && len(parts[1]) > 0 {
		return true, parts[0] + "@" + extension
	}
	return false, ""
}

func calcServiceName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	service := base[:len(base)-len(ext)]
	switch ext {
	case ".volume":
		service += "-volume"
	case ".network":
		service += "-network"
	case ".image":
		service += "-image"
	case ".build":
		service += "-build"
	case ".artifact":
		service += "-artifact"
	case ".pod":
		service += "-pod"
	}
	return service
}

func loadQuadletTestcase(path string) *quadletTestcase {
	return loadQuadletTestcaseWithServiceName(path, "")
}

func loadQuadletTestcaseWithServiceName(path, serviceName string) *quadletTestcase {
	data, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred())

	var service string
	if len(serviceName) > 0 {
		service = serviceName
	} else {
		service = calcServiceName(path)
	}
	service += ".service"

	checks := make([][]string, 0)

	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, "##") {
			words, err := shellwords.Parse(line[2:])
			Expect(err).ToNot(HaveOccurred())
			checks = append(checks, words)
		}
	}

	return &quadletTestcase{
		data,
		service,
		checks,
	}
}

func matchSublistAt(full []string, pos int, sublist []string) bool {
	if len(sublist) > len(full)-pos {
		return false
	}

	for i := range sublist {
		if sublist[i] != full[pos+i] {
			return false
		}
	}
	return true
}

func matchSublistRegexAt(full []string, pos int, sublist []string) bool {
	if len(sublist) > len(full)-pos {
		return false
	}

	for i := range sublist {
		matched, err := regexp.MatchString(sublist[i], full[pos+i])
		if err != nil || !matched {
			return false
		}
	}
	return true
}

func findSublist(full []string, sublist []string) int {
	if len(sublist) > len(full) {
		return -1
	}
	if len(sublist) == 0 {
		return -1
	}
	for i := 0; i < len(full)-len(sublist)+1; i++ {
		if matchSublistAt(full, i, sublist) {
			return i
		}
	}
	return -1
}

func findSublistRegex(full []string, sublist []string) int {
	if len(sublist) > len(full) {
		return -1
	}
	if len(sublist) == 0 {
		return -1
	}
	for i := 0; i < len(full)-len(sublist)+1; i++ {
		if matchSublistRegexAt(full, i, sublist) {
			return i
		}
	}
	return -1
}

func (t *quadletTestcase) assertStdErrContains(args []string, session *PodmanSessionIntegration) bool {
	return strings.Contains(session.ErrorToString(), args[0])
}

func (t *quadletTestcase) assertKeyIs(args []string, unit *parser.UnitFile) bool {
	Expect(len(args)).To(BeNumerically(">=", 3))
	group := args[0]
	key := args[1]
	values := args[2:]

	realValues := unit.LookupAll(group, key)
	if len(realValues) != len(values) {
		return false
	}

	for i := range realValues {
		if realValues[i] != values[i] {
			return false
		}
	}
	return true
}

func (t *quadletTestcase) assertHasKey(args []string, unit *parser.UnitFile) bool {
	Expect(args).To(HaveLen(3))
	group := args[0]
	key := args[1]
	value := args[2]

	realValues := unit.LookupAll(group, key)
	return slices.Contains(realValues, value)
}

func (t *quadletTestcase) assertKeyIsEmpty(args []string, unit *parser.UnitFile) bool {
	Expect(args).To(HaveLen(2))
	group := args[0]
	key := args[1]

	realValues := unit.LookupAll(group, key)
	return len(realValues) == 0
}

func (t *quadletTestcase) assertKeyIsRegex(args []string, unit *parser.UnitFile) bool {
	Expect(len(args)).To(BeNumerically(">=", 3))
	group := args[0]
	key := args[1]
	values := args[2:]

	realValues := unit.LookupAll(group, key)
	if len(realValues) != len(values) {
		return false
	}

	for i := range realValues {
		matched, _ := regexp.MatchString(values[i], realValues[i])
		if !matched {
			return false
		}
	}
	return true
}

func (t *quadletTestcase) assertKeyContains(args []string, unit *parser.UnitFile) bool {
	Expect(args).To(HaveLen(3))
	group := args[0]
	key := args[1]
	value := args[2]

	realValues := unit.LookupAll(group, key)
	for _, realValue := range realValues {
		if strings.Contains(realValue, value) {
			return true
		}
	}
	return false
}

func (t *quadletTestcase) assertKeyNotContains(args []string, unit *parser.UnitFile) bool {
	return !t.assertKeyContains(args, unit)
}

func (t *quadletTestcase) assertLastKeyIsRegex(args []string, unit *parser.UnitFile) bool {
	Expect(len(args)).To(BeNumerically(">=", 3))
	group := args[0]
	key := args[1]
	regex := args[2]

	value, ok := unit.LookupLast(group, key)
	if !ok {
		return false
	}

	matched, err := regexp.MatchString(regex, value)
	if err != nil || !matched {
		return false
	}
	return true
}

func (t *quadletTestcase) assertLastKeyContains(args []string, unit *parser.UnitFile) bool {
	Expect(args).To(HaveLen(3))
	group := args[0]
	key := args[1]
	value := args[2]

	realValue, ok := unit.LookupLast(group, key)
	return ok && strings.Contains(realValue, value)
}

func (t *quadletTestcase) assertLastKeyNotContains(args []string, unit *parser.UnitFile) bool {
	return !t.assertLastKeyContains(args, unit)
}

func (t *quadletTestcase) assertPodmanArgs(args []string, unit *parser.UnitFile, key string, allowRegex, globalOnly bool) bool {
	podmanArgs, _, _ := unit.LookupLastArgs("Service", key)
	if globalOnly {
		podmanCmdLocation := findSublist(podmanArgs, []string{args[0]})
		if podmanCmdLocation == -1 {
			return false
		}

		podmanArgs = podmanArgs[:podmanCmdLocation]
		args = args[1:]
	}

	var location int
	if allowRegex {
		location = findSublistRegex(podmanArgs, args)
	} else {
		location = findSublist(podmanArgs, args)
	}

	return location != -1
}

func keyValueStringToMap(keyValueString, separator string) (map[string]string, error) {
	keyValMap := make(map[string]string)
	csvReader := csv.NewReader(strings.NewReader(keyValueString))
	csvReader.Comma = []rune(separator)[0]
	keyVarList, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	for _, param := range keyVarList[0] {
		key, val, _ := strings.Cut(param, "=")
		keyValMap[key] = val
	}

	return keyValMap, nil
}

func keyValMapEqualRegex(expectedKeyValMap, actualKeyValMap map[string]string) bool {
	if len(expectedKeyValMap) != len(actualKeyValMap) {
		return false
	}
	for key, expectedValue := range expectedKeyValMap {
		actualValue, ok := actualKeyValMap[key]
		if !ok {
			return false
		}
		matched, err := regexp.MatchString(expectedValue, actualValue)
		if err != nil || !matched {
			return false
		}
	}
	return true
}

func (t *quadletTestcase) assertPodmanArgsKeyVal(args []string, unit *parser.UnitFile, key string, allowRegex, globalOnly bool) bool {
	podmanArgs, _, _ := unit.LookupLastArgs("Service", key)

	if globalOnly {
		podmanCmdLocation := findSublist(podmanArgs, []string{args[0]})
		if podmanCmdLocation == -1 {
			return false
		}

		podmanArgs = podmanArgs[:podmanCmdLocation]
		args = args[1:]
	}

	expectedKeyValMap, err := keyValueStringToMap(args[2], args[1])
	if err != nil {
		return false
	}
	argKeyLocation := 0
	for {
		subListLocation := findSublist(podmanArgs[argKeyLocation:], []string{args[0]})
		if subListLocation == -1 {
			break
		}

		argKeyLocation += subListLocation
		actualKeyValMap, err := keyValueStringToMap(podmanArgs[argKeyLocation+1], args[1])
		if err != nil {
			break
		}
		if allowRegex {
			if keyValMapEqualRegex(expectedKeyValMap, actualKeyValMap) {
				return true
			}
		} else if reflect.DeepEqual(expectedKeyValMap, actualKeyValMap) {
			return true
		}

		argKeyLocation += 2

		if argKeyLocation > len(podmanArgs) {
			break
		}
	}

	return false
}

func (t *quadletTestcase) assertPodmanFinalArgs(args []string, unit *parser.UnitFile, key string) bool {
	podmanArgs, _, _ := unit.LookupLastArgs("Service", key)
	if len(podmanArgs) < len(args) {
		return false
	}
	return matchSublistAt(podmanArgs, len(podmanArgs)-len(args), args)
}

func (t *quadletTestcase) assertPodmanFinalArgsRegex(args []string, unit *parser.UnitFile, key string) bool {
	podmanArgs, _, _ := unit.LookupLastArgs("Service", key)
	if len(podmanArgs) < len(args) {
		return false
	}
	return matchSublistRegexAt(podmanArgs, len(podmanArgs)-len(args), args)
}

func (t *quadletTestcase) assertStartPodmanArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStart", false, false)
}

func (t *quadletTestcase) assertStartPodmanArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStart", true, false)
}

func (t *quadletTestcase) assertStartPodmanGlobalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStart", false, true)
}

func (t *quadletTestcase) assertStartPodmanGlobalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStart", true, true)
}

func (t *quadletTestcase) assertStartPodmanArgsKeyVal(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStart", false, false)
}

func (t *quadletTestcase) assertStartPodmanArgsKeyValRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStart", true, false)
}

func (t *quadletTestcase) assertStartPodmanGlobalArgsKeyVal(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStart", false, true)
}

func (t *quadletTestcase) assertStartPodmanGlobalArgsKeyValRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStart", true, true)
}

func (t *quadletTestcase) assertStartPodmanFinalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgs(args, unit, "ExecStart")
}

func (t *quadletTestcase) assertStartPodmanFinalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgsRegex(args, unit, "ExecStart")
}

func (t *quadletTestcase) assertStartPrePodmanArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStartPre", false, false)
}

func (t *quadletTestcase) assertStartPrePodmanArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStartPre", true, false)
}

func (t *quadletTestcase) assertStartPrePodmanGlobalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStartPre", false, true)
}

func (t *quadletTestcase) assertStartPrePodmanGlobalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStartPre", true, true)
}

func (t *quadletTestcase) assertStartPrePodmanArgsKeyVal(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStartPre", false, false)
}

func (t *quadletTestcase) assertStartPrePodmanArgsKeyValRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStartPre", true, false)
}

func (t *quadletTestcase) assertStartPrePodmanGlobalArgsKeyVal(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStartPre", false, true)
}

func (t *quadletTestcase) assertStartPrePodmanGlobalArgsKeyValRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStartPre", true, true)
}

func (t *quadletTestcase) assertStartPrePodmanFinalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgs(args, unit, "ExecStartPre")
}

func (t *quadletTestcase) assertStartPrePodmanFinalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgsRegex(args, unit, "ExecStartPre")
}

func (t *quadletTestcase) assertStopPodmanArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStop", false, false)
}

func (t *quadletTestcase) assertStopPodmanGlobalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStop", false, true)
}

func (t *quadletTestcase) assertStopPodmanFinalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgs(args, unit, "ExecStop")
}

func (t *quadletTestcase) assertStopPodmanFinalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgsRegex(args, unit, "ExecStop")
}

func (t *quadletTestcase) assertStopPodmanArgsKeyVal(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStop", false, false)
}

func (t *quadletTestcase) assertStopPodmanArgsKeyValRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStop", true, false)
}

func (t *quadletTestcase) assertStopPostPodmanArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStopPost", false, false)
}

func (t *quadletTestcase) assertStopPostPodmanGlobalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecStopPost", false, true)
}

func (t *quadletTestcase) assertStopPostPodmanFinalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgs(args, unit, "ExecStopPost")
}

func (t *quadletTestcase) assertStopPostPodmanFinalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgsRegex(args, unit, "ExecStopPost")
}

func (t *quadletTestcase) assertStopPostPodmanArgsKeyVal(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStopPost", false, false)
}

func (t *quadletTestcase) assertStopPostPodmanArgsKeyValRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecStopPost", true, false)
}

func (t *quadletTestcase) assertReloadPodmanArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecReload", false, false)
}

func (t *quadletTestcase) assertReloadPodmanGlobalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgs(args, unit, "ExecReload", false, true)
}

func (t *quadletTestcase) assertReloadPodmanFinalArgs(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgs(args, unit, "ExecReload")
}

func (t *quadletTestcase) assertReloadPodmanFinalArgsRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanFinalArgsRegex(args, unit, "ExecReload")
}

func (t *quadletTestcase) assertReloadPodmanArgsKeyVal(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecReload", false, false)
}

func (t *quadletTestcase) assertReloadPodmanArgsKeyValRegex(args []string, unit *parser.UnitFile) bool {
	return t.assertPodmanArgsKeyVal(args, unit, "ExecReload", true, false)
}

func (t *quadletTestcase) assertSymlink(args []string, unit *parser.UnitFile) bool {
	Expect(args).To(HaveLen(2))
	symlink := args[0]
	expectedTarget := args[1]

	dir := filepath.Dir(unit.Path)

	target, err := os.Readlink(filepath.Join(dir, symlink))
	Expect(err).ToNot(HaveOccurred())

	return expectedTarget == target
}

func (t *quadletTestcase) doAssert(check []string, unit *parser.UnitFile, session *PodmanSessionIntegration) error {
	Expect(check).ToNot(BeEmpty())
	op := check[0]
	args := make([]string, 0)
	for _, a := range check[1:] {
		// Apply \n and \t as they are used in the testcases
		a = strings.ReplaceAll(a, "\\n", "\n")
		a = strings.ReplaceAll(a, "\\t", "\t")
		args = append(args, a)
	}
	invert := false
	if op[0] == '!' {
		invert = true
		op = op[1:]
	}

	var ok bool
	switch op {
	case "assert-failed":
		ok = true /* Handled separately */
	case "assert-stderr-contains":
		ok = t.assertStdErrContains(args, session)
	case "assert-key-is":
		ok = t.assertKeyIs(args, unit)
	case "assert-has-key":
		ok = t.assertHasKey(args, unit)
	case "assert-key-is-empty":
		ok = t.assertKeyIsEmpty(args, unit)
	case "assert-key-is-regex":
		ok = t.assertKeyIsRegex(args, unit)
	case "assert-key-contains":
		ok = t.assertKeyContains(args, unit)
	case "assert-key-not-contains":
		ok = t.assertKeyNotContains(args, unit)
	case "assert-last-key-contains":
		ok = t.assertLastKeyContains(args, unit)
	case "assert-last-key-not-contains":
		ok = t.assertLastKeyNotContains(args, unit)
	case "assert-last-key-is-regex":
		ok = t.assertLastKeyIsRegex(args, unit)
	case "assert-podman-args":
		ok = t.assertStartPodmanArgs(args, unit)
	case "assert-podman-args-regex":
		ok = t.assertStartPodmanArgsRegex(args, unit)
	case "assert-podman-args-key-val":
		ok = t.assertStartPodmanArgsKeyVal(args, unit)
	case "assert-podman-args-key-val-regex":
		ok = t.assertStartPodmanArgsKeyValRegex(args, unit)
	case "assert-podman-global-args":
		ok = t.assertStartPodmanGlobalArgs(args, unit)
	case "assert-podman-global-args-regex":
		ok = t.assertStartPodmanGlobalArgsRegex(args, unit)
	case "assert-podman-global-args-key-val":
		ok = t.assertStartPodmanGlobalArgsKeyVal(args, unit)
	case "assert-podman-global-args-key-val-regex":
		ok = t.assertStartPodmanGlobalArgsKeyValRegex(args, unit)
	case "assert-podman-final-args":
		ok = t.assertStartPodmanFinalArgs(args, unit)
	case "assert-podman-final-args-regex":
		ok = t.assertStartPodmanFinalArgsRegex(args, unit)
	case "assert-podman-pre-args":
		ok = t.assertStartPrePodmanArgs(args, unit)
	case "assert-podman-pre-args-regex":
		ok = t.assertStartPrePodmanArgsRegex(args, unit)
	case "assert-podman-pre-args-key-val":
		ok = t.assertStartPrePodmanArgsKeyVal(args, unit)
	case "assert-podman-pre-args-key-val-regex":
		ok = t.assertStartPrePodmanArgsKeyValRegex(args, unit)
	case "assert-podman-pre-global-args":
		ok = t.assertStartPrePodmanGlobalArgs(args, unit)
	case "assert-podman-pre-global-args-regex":
		ok = t.assertStartPrePodmanGlobalArgsRegex(args, unit)
	case "assert-podman-pre-global-args-key-val":
		ok = t.assertStartPrePodmanGlobalArgsKeyVal(args, unit)
	case "assert-podman-pre-global-args-key-val-regex":
		ok = t.assertStartPrePodmanGlobalArgsKeyValRegex(args, unit)
	case "assert-podman-pre-final-args":
		ok = t.assertStartPrePodmanFinalArgs(args, unit)
	case "assert-podman-pre-final-args-regex":
		ok = t.assertStartPrePodmanFinalArgsRegex(args, unit)
	case "assert-symlink":
		ok = t.assertSymlink(args, unit)
	case "assert-podman-stop-args":
		ok = t.assertStopPodmanArgs(args, unit)
	case "assert-podman-stop-global-args":
		ok = t.assertStopPodmanGlobalArgs(args, unit)
	case "assert-podman-stop-final-args":
		ok = t.assertStopPodmanFinalArgs(args, unit)
	case "assert-podman-stop-final-args-regex":
		ok = t.assertStopPodmanFinalArgsRegex(args, unit)
	case "assert-podman-stop-args-key-val":
		ok = t.assertStopPodmanArgsKeyVal(args, unit)
	case "assert-podman-stop-args-key-val-regex":
		ok = t.assertStopPodmanArgsKeyValRegex(args, unit)
	case "assert-podman-stop-post-args":
		ok = t.assertStopPostPodmanArgs(args, unit)
	case "assert-podman-stop-post-global-args":
		ok = t.assertStopPostPodmanGlobalArgs(args, unit)
	case "assert-podman-stop-post-final-args":
		ok = t.assertStopPostPodmanFinalArgs(args, unit)
	case "assert-podman-stop-post-final-args-regex":
		ok = t.assertStopPostPodmanFinalArgsRegex(args, unit)
	case "assert-podman-stop-post-args-key-val":
		ok = t.assertStopPostPodmanArgsKeyVal(args, unit)
	case "assert-podman-stop-post-args-key-val-regex":
		ok = t.assertStopPostPodmanArgsKeyValRegex(args, unit)
	case "assert-podman-reload-args":
		ok = t.assertReloadPodmanArgs(args, unit)
	case "assert-podman-reload-global-args":
		ok = t.assertReloadPodmanGlobalArgs(args, unit)
	case "assert-podman-reload-final-args":
		ok = t.assertReloadPodmanFinalArgs(args, unit)
	case "assert-podman-reload-final-args-regex":
		ok = t.assertReloadPodmanFinalArgsRegex(args, unit)
	case "assert-podman-reload-args-key-val":
		ok = t.assertReloadPodmanArgsKeyVal(args, unit)
	case "assert-podman-reload-args-key-val-regex":
		ok = t.assertReloadPodmanArgsKeyValRegex(args, unit)

	default:
		return fmt.Errorf("Unsupported assertion %s", op)
	}
	if invert {
		ok = !ok
	}

	if !ok {
		s := "(nil)"
		if unit != nil {
			s, _ = unit.ToString()
		}
		return fmt.Errorf("Failed assertion for %s: %s\n\n%s", t.serviceName, strings.Join(check, " "), s)
	}
	return nil
}

func (t *quadletTestcase) check(generateDir string, session *PodmanSessionIntegration) {
	expectFail := false
	for _, c := range t.checks {
		if c[0] == "assert-failed" {
			expectFail = true
		}
	}

	file := filepath.Join(generateDir, t.serviceName)
	_, err := os.Stat(file)
	if expectFail {
		Expect(err).To(MatchError(os.ErrNotExist))
	} else {
		Expect(err).ToNot(HaveOccurred())
	}

	var unit *parser.UnitFile
	if !expectFail {
		unit, err = parser.ParseUnitFile(file)
		Expect(err).ToNot(HaveOccurred())
	}

	for _, check := range t.checks {
		err := t.doAssert(check, unit, session)
		Expect(err).ToNot(HaveOccurred())
	}
}

var _ = Describe("quadlet system generator", func() {
	var (
		err          error
		generatedDir string
		quadletDir   string

		runQuadletTestCaseWithServiceName = func(fileName string, exitCode int, errString string, serviceName string) {
			testcase := loadQuadletTestcaseWithServiceName(filepath.Join("quadlet", fileName), serviceName)

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0o644)
			Expect(err).ToNot(HaveOccurred())

			// Also copy any extra snippets
			snippetdirs := []string{fileName + ".d"}
			if ok, genericFileName := getGenericTemplateFile(fileName); ok {
				snippetdirs = append(snippetdirs, genericFileName+".d")
			}
			for _, snippetdir := range snippetdirs {
				dotdDir := filepath.Join("quadlet", snippetdir)
				if s, err := os.Stat(dotdDir); err == nil && s.IsDir() {
					dotdDirDest := filepath.Join(quadletDir, snippetdir)
					err = os.Mkdir(dotdDirDest, os.ModePerm)
					Expect(err).ToNot(HaveOccurred())
					err = CopyDirectory(dotdDir, dotdDirDest)
					Expect(err).ToNot(HaveOccurred())
				}
			}

			// Run quadlet to convert the file
			var args []string
			if isRootless() {
				args = append(args, "--user")
			}
			args = append(args, "--no-kmsg-log", generatedDir)
			session := podmanTest.Quadlet(args, quadletDir)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(exitCode))

			// Print any stderr output
			errs := session.ErrorToString()
			if errs != "" {
				GinkgoWriter.Println("error:", session.ErrorToString())
			}
			Expect(errs).Should(ContainSubstring(errString))

			testcase.check(generatedDir, session)
		}

		runQuadletTestCase = func(fileName string, exitCode int, errString string) {
			runQuadletTestCaseWithServiceName(fileName, exitCode, errString, "")
		}

		runSuccessQuadletTestCase = func(fileName string) {
			runQuadletTestCase(fileName, 0, "")
		}

		runErrorQuadletTestCase = func(fileName, errString string) {
			runQuadletTestCase(fileName, 1, errString)
		}

		runWarningQuadletTestCase = func(fileName, errString string) {
			runQuadletTestCase(fileName, 0, errString)
		}
	)

	BeforeEach(func() {
		generatedDir = filepath.Join(podmanTest.TempDir, "generated")
		err = os.Mkdir(generatedDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		quadletDir = filepath.Join(podmanTest.TempDir, "quadlet")
		err = os.Mkdir(quadletDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("quadlet -version", func() {
		It("Should print correct version", func() {
			session := podmanTest.Quadlet([]string{"-version"}, "/something")
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(version.Version.String()))
		})
	})

	Describe("Running quadlet dryrun tests", func() {
		It("Should exit with an error because of no files are found to parse", func() {
			fileName := "basic.kube"
			testcase := loadQuadletTestcase(filepath.Join("quadlet", fileName))

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0o644)
			Expect(err).ToNot(HaveOccurred())

			session := podmanTest.Quadlet([]string{"-dryrun"}, "/something")
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			current := session.ErrorToStringArray()
			expected := "No files parsed from [/something]"

			found := false
			for _, line := range current {
				if strings.Contains(line, expected) {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})

		It("Should fail on bad quadlet", func() {
			quadletfile := fmt.Sprintf(`[Container]
Image=%s
BOGUS=foo
`, ALPINE)

			quadletfilePath := filepath.Join(podmanTest.TempDir, "bogus.container")
			err = os.WriteFile(quadletfilePath, []byte(quadletfile), 0o644)
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(quadletfilePath)
			session := podmanTest.Quadlet([]string{"-dryrun"}, podmanTest.TempDir)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(1))
			Expect(session.ErrorToString()).To(ContainSubstring("converting \"bogus.container\": unsupported key 'BOGUS' in group 'Container' in " + quadletfilePath))
		})

		It("Should scan and return output for files in subdirectories", func() {
			dirName := "test_subdir"

			err = CopyDirectory(filepath.Join("quadlet", dirName), quadletDir)
			if err != nil {
				GinkgoWriter.Println("error:", err)
			}

			session := podmanTest.Quadlet([]string{"-dryrun", "-user"}, quadletDir)
			session.WaitWithDefaultTimeout()

			current := session.OutputToStringArray()
			expected := []string{
				"---mysleep.service---",
				"---mysleep_1.service---",
				"---mysleep_2.service---",
			}

			Expect(current).To(ContainElements(expected))
		})

		It("Should parse a kube file and print it to stdout", func() {
			fileName := "basic.kube"
			testcase := loadQuadletTestcase(filepath.Join("quadlet", fileName))

			// quadlet uses PODMAN env to get a stable podman path
			podmanPath, found := os.LookupEnv("PODMAN")
			if !found {
				podmanPath = podmanTest.PodmanBinary
			}

			// Write the tested file to the quadlet dir
			err = os.WriteFile(filepath.Join(quadletDir, fileName), testcase.data, 0o644)
			Expect(err).ToNot(HaveOccurred())

			session := podmanTest.Quadlet([]string{"-dryrun"}, quadletDir)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.ErrorToString()).To(ContainSubstring("Loading source unit file "))

			current := session.OutputToStringArray()
			expected := []string{
				"---basic.service---",
				"## assert-podman-args \"kube\"",
				"## assert-podman-args \"play\"",
				"## assert-podman-final-args-regex .*/podman-e2e-.*/subtest-.*/quadlet/deployment.yml",
				"## assert-podman-args \"--replace\"",
				"## assert-podman-args \"--service-container=true\"",
				"## assert-podman-stop-post-args \"kube\"",
				"## assert-podman-stop-post-args \"down\"",
				"## assert-podman-stop-post-final-args-regex .*/podman-e2e-.*/subtest-.*/quadlet/deployment.yml",
				"## assert-key-is \"Unit\" \"RequiresMountsFor\" \"%t/containers\"",
				"## assert-key-is \"Service\" \"KillMode\" \"mixed\"",
				"## assert-key-is \"Service\" \"Type\" \"notify\"",
				"## assert-key-is \"Service\" \"NotifyAccess\" \"all\"",
				"## assert-key-is \"Service\" \"Environment\" \"PODMAN_SYSTEMD_UNIT=%n\"",
				"## assert-key-is \"Service\" \"SyslogIdentifier\" \"%N\"",
				"[X-Kube]",
				"Yaml=deployment.yml",
				"[Unit]",
				"Wants=network-online.target",
				"After=network-online.target",
				fmt.Sprintf("SourcePath=%s/basic.kube", quadletDir),
				"RequiresMountsFor=%t/containers",
				"[Service]",
				"KillMode=mixed",
				"Environment=PODMAN_SYSTEMD_UNIT=%n",
				"Type=notify",
				"NotifyAccess=all",
				"SyslogIdentifier=%N",
				fmt.Sprintf("ExecStart=%s kube play --replace --service-container=true %s/deployment.yml", podmanPath, quadletDir),
				fmt.Sprintf("ExecStopPost=%s kube down %s/deployment.yml", podmanPath, quadletDir),
			}

			Expect(current).To(Equal(expected))
		})
	})

	It("Should not leave temp files after generating service files", func() {
		runSuccessQuadletTestCase("basic.container")

		matches, err := filepath.Glob(filepath.Join(generatedDir, ".tmp-basic.service*"))
		Expect(err).ToNot(HaveOccurred())
		Expect(matches).To(BeEmpty())
	})

	DescribeTable("Running success quadlet test case",
		runSuccessQuadletTestCase,
		Entry("Basic container", "basic.container"),
		Entry("annotation.container", "annotation.container"),
		Entry("apparmor.container", "apparmor.container"),
		Entry("autoupdate.container", "autoupdate.container"),
		Entry("basepodman.container", "basepodman.container"),
		Entry("capabilities.container", "capabilities.container"),
		Entry("capabilities2.container", "capabilities2.container"),
		Entry("comment-with-continuation.container", "comment-with-continuation.container"),
		Entry("devices.container", "devices.container"),
		Entry("disableselinux.container", "disableselinux.container"),
		Entry("dns-options.container", "dns-options.container"),
		Entry("dns-search.container", "dns-search.container"),
		Entry("dns.container", "dns.container"),
		Entry("env-file.container", "env-file.container"),
		Entry("env-host-false.container", "env-host-false.container"),
		Entry("env-host.container", "env-host.container"),
		Entry("env.container", "env.container"),
		Entry("entrypoint.container", "entrypoint.container"),
		Entry("entrypoint-empty.container", "entrypoint-empty.container"),
		Entry("escapes.container", "escapes.container"),
		Entry("exec.container", "exec.container"),
		Entry("group-add.container", "group-add.container"),
		Entry("health.container", "health.container"),
		Entry("health-cmd-shell-quotes.container", "health-cmd-shell-quotes.container"),
		Entry("host.container", "host.container"),
		Entry("httpproxy-false.container", "httpproxy-false.container"),
		Entry("httpproxy-true.container", "httpproxy-true.container"),
		Entry("hostname.container", "hostname.container"),
		Entry("idmapping.container", "idmapping.container"),
		Entry("image.container", "image.container"),
		Entry("image-volume.container", "image-volume.container"),
		Entry("install.container", "install.container"),
		Entry("ip.container", "ip.container"),
		Entry("label.container", "label.container"),
		Entry("line-continuation-whitespace.container", "line-continuation-whitespace.container"),
		Entry("logdriver.container", "logdriver.container"),
		Entry("logopt.container", "logopt.container"),
		Entry("mask.container", "mask.container"),
		Entry("memory.container", "memory.container"),
		Entry("name.container", "name.container"),
		Entry("nestedselinux.container", "nestedselinux.container"),
		Entry("network.container", "network.container"),
		Entry("notify.container", "notify.container"),
		Entry("notify-healthy.container", "notify-healthy.container"),
		Entry("oneshot.container", "oneshot.container"),
		Entry("other-sections.container", "other-sections.container"),
		Entry("podmanargs.container", "podmanargs.container"),
		Entry("ports.container", "ports.container"),
		Entry("ports_ipv6.container", "ports_ipv6.container"),
		Entry("pull.container", "pull.container"),
		Entry("quotes.container", "quotes.container"),
		Entry("readonly.container", "readonly.container"),
		Entry("readonly-tmpfs.container", "readonly-tmpfs.container"),
		Entry("readonly-notmpfs.container", "readonly-notmpfs.container"),
		Entry("readwrite-notmpfs.container", "readwrite-notmpfs.container"),
		Entry("volatiletmp-readwrite.container", "volatiletmp-readwrite.container"),
		Entry("volatiletmp-readonly.container", "volatiletmp-readonly.container"),
		Entry("remap-auto.container", "remap-auto.container"),
		Entry("remap-auto2.container", "remap-auto2.container"),
		Entry("remap-keep-id.container", "remap-keep-id.container"),
		Entry("remap-keep-id2.container", "remap-keep-id2.container"),
		Entry("remap-manual.container", "remap-manual.container"),
		Entry("rootfs.container", "rootfs.container"),
		Entry("seccomp.container", "seccomp.container"),
		Entry("secrets.container", "secrets.container"),
		Entry("selinux.container", "selinux.container"),
		Entry("shmsize.container", "shmsize.container"),
		Entry("stopsigal.container", "stopsignal.container"),
		Entry("stoptimeout.container", "stoptimeout.container"),
		Entry("subidmapping.container", "subidmapping.container"),
		Entry("sysctl.container", "sysctl.container"),
		Entry("timezone.container", "timezone.container"),
		Entry("ulimit.container", "ulimit.container"),
		Entry("unmask.container", "unmask.container"),
		Entry("user.container", "user.container"),
		Entry("userns.container", "userns.container"),
		Entry("workingdir.container", "workingdir.container"),
		Entry("Container - global args", "globalargs.container"),
		Entry("Container - Containers Conf Modules", "containersconfmodule.container"),
		Entry("merged.container", "merged.container"),
		Entry("merged-override.container", "merged-override.container"),
		Entry("template@.container", "template@.container"),
		Entry("template@instance.container", "template@instance.container"),
		Entry("Unit After Override", "unit-after-override.container"),
		Entry("NetworkAlias", "network-alias.container"),
		Entry("CgroupMode", "cgroups-mode.container"),
		Entry("Container - No Default Dependencies", "no_deps.container"),
		Entry("retry.container", "retry.container"),
		Entry("reloadcmd.container", "reloadcmd.container"),
		Entry("reloadsignal.container", "reloadsignal.container"),

		Entry("basic.volume", "basic.volume"),
		Entry("device-copy.volume", "device-copy.volume"),
		Entry("device.volume", "device.volume"),
		Entry("label.volume", "label.volume"),
		Entry("name.volume", "name.volume"),
		Entry("podmanargs.volume", "podmanargs.volume"),
		Entry("uid.volume", "uid.volume"),
		Entry("volume-uid-gid.volume", "volume-uid-gid.volume"),
		Entry("image.volume", "image.volume"),
		Entry("Volume - global args", "globalargs.volume"),
		Entry("Volume - Containers Conf Modules", "containersconfmodule.volume"),
		Entry("Volume - Type=bind", "device-bind.volume"),
		Entry("mount-options.volume", "mount-options.volume"),

		Entry("Absolute Path", "absolute.path.kube"),
		Entry("Basic kube", "basic.kube"),
		Entry("Kube - ConfigMap", "configmap.kube"),
		Entry("Kube - Exit Code Propagation", "exit_code_propagation.kube"),
		Entry("Kube - Logdriver", "logdriver.kube"),
		Entry("Kube - Logopt", "logopt.kube"),
		Entry("Kube - Network", "network.kube"),
		Entry("Kube - PodmanArgs", "podmanargs.kube"),
		Entry("Kube - Publish IPv4 ports", "ports.kube"),
		Entry("Kube - Publish IPv6 ports", "ports_ipv6.kube"),
		Entry("Kube - User Remap Auto with IDs", "remap-auto2.kube"),
		Entry("Kube - User Remap Auto", "remap-auto.kube"),
		Entry("Syslog Identifier", "syslog.identifier.kube"),
		Entry("Kube - Working Directory YAML Absolute Path", "workingdir-yaml-abs.kube"),
		Entry("Kube - Working Directory YAML Relative Path", "workingdir-yaml-rel.kube"),
		Entry("Kube - Working Directory Unit", "workingdir-unit.kube"),
		Entry("Kube - Multiple YAML entries", "multiple-yaml.kube"),
		Entry("Kube - Working Directory already in Service", "workingdir-service.kube"),
		Entry("Kube - global args", "globalargs.kube"),
		Entry("Kube - Containers Conf Modules", "containersconfmodule.kube"),
		Entry("Kube - Service Type=oneshot", "oneshot.kube"),
		Entry("Kube - Down force", "downforce.kube"),

		Entry("Network - Basic", "basic.network"),
		Entry("Network - Disable DNS", "disable-dns.network"),
		Entry("Network - DNS", "dns.network"),
		Entry("Network - Driver", "driver.network"),
		Entry("Network - Gateway", "gateway.network"),
		Entry("Network - IPAM Driver", "ipam-driver.network"),
		Entry("Network - IPv6", "ipv6.network"),
		Entry("Network - InterfaceName network", "interface-name.network"),
		Entry("Network - Internal network", "internal.network"),
		Entry("Network - Label", "label.network"),
		Entry("Network - Multiple Options", "options.multiple.network"),
		Entry("Network - Name", "name.network"),
		Entry("Network - Options", "options.network"),
		Entry("Network - PodmanArgs", "podmanargs.network"),
		Entry("Network - Range", "range.network"),
		Entry("Network - Subnets", "subnets.network"),
		Entry("Network - multiple subnet, gateway and range", "subnet-trio.multiple.network"),
		Entry("Network - subnet, gateway and range", "subnet-trio.network"),
		Entry("Network - global args", "globalargs.network"),
		Entry("Network - Containers Conf Modules", "containersconfmodule.network"),
		Entry("Network - Delete on stop", "delete.network"),

		Entry("Image - Basic", "basic.image"),
		Entry("Image - Architecture", "arch.image"),
		Entry("Image - Auth File", "auth.image"),
		Entry("Image - Certificates", "certs.image"),
		Entry("Image - Credentials", "creds.image"),
		Entry("Image - Decryption Key", "decrypt.image"),
		Entry("Image - OS Key", "os.image"),
		Entry("Image - Policy Key", "policy.image"),
		Entry("Image - Variant Key", "variant.image"),
		Entry("Image - All Tags", "all-tags.image"),
		Entry("Image - TLS Verify", "tls-verify.image"),
		Entry("Image - Arch and OS", "arch-os.image"),
		Entry("Image - global args", "globalargs.image"),
		Entry("Image - Containers Conf Modules", "containersconfmodule.image"),
		Entry("Image - Unit After Override", "unit-after-override.image"),
		Entry("Image - No Default Dependencies", "no_deps.image"),
		Entry("Image - Retry", "retry.image"),

		Entry("Build - Basic", "basic.build"),
		Entry("Build - Annotation Key", "annotation.build"),
		Entry("Build - Arch Key", "arch.build"),
		Entry("Build - AuthFile Key", "authfile.build"),
		Entry("Build - DNS Key", "dns.build"),
		Entry("Build - DNSOptions Key", "dns-options.build"),
		Entry("Build - DNSSearch Key", "dns-search.build"),
		Entry("Build - Environment Key", "env.build"),
		Entry("Build - File Key absolute", "file-abs.build"),
		Entry("Build - File Key relative", "file-rel.build"),
		Entry("Build - File Key HTTP(S) URL", "file-https.build"),
		Entry("Build - ForceRM Key", "force-rm.build"),
		Entry("Build - GlobalArgs", "globalargs.build"),
		Entry("Build - GroupAdd Key", "group-add.build"),
		Entry("Build - IgnoreFile Key", "ignorefile.build"),
		Entry("Build - Containers Conf Modules", "containersconfmodule.build"),
		Entry("Build - Label Key", "label.build"),
		Entry("Build - Multiple Tags", "multiple-tags.build"),
		Entry("Build - Network Key host", "network.build"),
		Entry("Build - PodmanArgs", "podmanargs.build"),
		Entry("Build - BuildArg Key", "buildarg.build"),
		Entry("Build - Pull Key", "pull.build"),
		Entry("Build - Secrets", "secrets.build"),
		Entry("Build - SetWorkingDirectory is absolute path", "setworkingdirectory-is-abs.build"),
		Entry("Build - SetWorkingDirectory is absolute File= path", "setworkingdirectory-is-file-abs.build"),
		Entry("Build - SetWorkingDirectory is relative path", "setworkingdirectory-is-rel.build"),
		Entry("Build - SetWorkingDirectory is relative File= path", "setworkingdirectory-is-file-rel.build"),
		Entry("Build - SetWorkingDirectory is https://.git URL", "setworkingdirectory-is-https-git.build"),
		Entry("Build - SetWorkingDirectory is git:// URL", "setworkingdirectory-is-git.build"),
		Entry("Build - SetWorkingDirectory is github.com URL", "setworkingdirectory-is-github.build"),
		Entry("Build - SetWorkingDirectory is archive URL", "setworkingdirectory-is-archive.build"),
		Entry("Build - Target Key", "target.build"),
		Entry("Build - TLSVerify Key", "tls-verify.build"),
		Entry("Build - Variant Key", "variant.build"),
		Entry("Build - No Default Dependencies", "no_deps.build"),
		Entry("Build - Retry", "retry.build"),
		Entry("Build - No WorkingDirectory with systemd specifier", "no-workingdirectory-systemd-specifier.build"),

		Entry("Artifact - Basic", "basic.artifact"),
		Entry("Artifact - Options", "options.artifact"),

		Entry("Pod - Basic", "basic.pod"),
		Entry("Pod - DNS", "dns.pod"),
		Entry("Pod - DNS Option", "dns-option.pod"),
		Entry("Pod - DNS Search", "dns-search.pod"),
		Entry("Pod - Host", "host.pod"),
		Entry("Pod - HostName", "hostname.pod"),
		Entry("Pod - IP", "ip.pod"),
		Entry("Pod - Label", "label.pod"),
		Entry("Pod - Name", "name.pod"),
		Entry("Pod - Network", "network.pod"),
		Entry("Pod - PodmanArgs", "podmanargs.pod"),
		Entry("Pod - NetworkAlias", "network-alias.pod"),
		Entry("Pod - Remap auto", "remap-auto.pod"),
		Entry("Pod - Remap auto2", "remap-auto2.pod"),
		Entry("Pod - Remap keep-id", "remap-keep-id.pod"),
		Entry("Pod - Remap manual", "remap-manual.pod"),
		Entry("Pod - Shm Size", "shmsize.pod"),
		Entry("Pod - StopTimeout", "stoptimeout.pod"),
		Entry("Pod - Service Environment", "service-environment.pod"),
		Entry("Pod - Restart policy override", "restart.pod"),
	)

	DescribeTable("Running expected warning quadlet test case",
		runWarningQuadletTestCase,
		Entry("label-unsupported-escape.container", "label-unsupported-escape.container", "unsupported escape char"),
		Entry("shortname.container", "shortname.container", "Warning: shortname.container specifies the image \"shortname\" which not a fully qualified image name. This is not ideal for performance and security reasons. See the podman-pull manpage discussion of short-name-aliases.conf for details."),

		Entry("Unsupported Service Key - User", "service-user.container", "Warning: using key User in the Service group is not supported"),
		Entry("Unsupported Service Key - Group", "service-group.container", "Warning: using key Group in the Service group is not supported"),
		Entry("Unsupported Service Key - DynamicUser.build", "service-dynamicuser.build", "Warning: using key DynamicUser in the Service group is not supported"),
		Entry("Unsupported Service Key - DynamicUser.container", "service-dynamicuser.container", "Warning: using key DynamicUser in the Service group is not supported"),
		Entry("Unsupported Service Key - DynamicUser.image", "service-dynamicuser.image", "Warning: using key DynamicUser in the Service group is not supported"),
		Entry("Unsupported Service Key - DynamicUser.kube", "service-dynamicuser.kube", "Warning: using key DynamicUser in the Service group is not supported"),
		Entry("Unsupported Service Key - DynamicUser.network", "service-dynamicuser.network", "Warning: using key DynamicUser in the Service group is not supported"),
		Entry("Unsupported Service Key - DynamicUser.pod", "service-dynamicuser.pod", "Warning: using key DynamicUser in the Service group is not supported"),
		Entry("Unsupported Service Key - DynamicUser.volume", "service-dynamicuser.volume", "Warning: using key DynamicUser in the Service group is not supported"),

		Entry("exec-unsupported-escape.container", "exec-unsupported-escape.container", "unsupported escape char"),
		Entry("reloadcmd-unsupported-escape.container", "reloadcmd-unsupported-escape.container", "unsupported escape char"),
	)

	DescribeTable("Running expected error quadlet test case",
		runErrorQuadletTestCase,
		Entry("idmapping-with-remap.container", "idmapping-with-remap.container", "converting \"idmapping-with-remap.container\": deprecated Remap keys are set along with explicit mapping keys"),
		Entry("noimage.container", "noimage.container", "converting \"noimage.container\": no Image or Rootfs key specified"),
		Entry("pod.non-quadlet.container", "pod.non-quadlet.container", "converting \"pod.non-quadlet.container\": pod test-pod is not Quadlet based"),
		Entry("pod.not-found.container", "pod.not-found.container", "converting \"pod.not-found.container\": quadlet pod unit not-found.pod does not exist"),
		Entry("subidmapping-with-remap.container", "subidmapping-with-remap.container", "converting \"subidmapping-with-remap.container\": deprecated Remap keys are set along with explicit mapping keys"),
		Entry("userns-with-remap.container", "userns-with-remap.container", "converting \"userns-with-remap.container\": deprecated Remap keys are set along with explicit mapping keys"),
		Entry("reloadboth.container", "reloadboth.container", "converting \"reloadboth.container\": ReloadCmd and ReloadSignal are mutually exclusive but both are set"),
		Entry("dependent.error.container", "dependent.error.container", "converting \"dependent.error.container\": unable to translate dependency for basic.container"),

		Entry("image-no-image.volume", "image-no-image.volume", "converting \"image-no-image.volume\": the key Image is mandatory when using the image driver"),
		Entry("Volume - Quadlet image (.build) not found", "build-not-found.quadlet.volume", "converting \"build-not-found.quadlet.volume\": requested Quadlet image not-found.build was not found"),
		Entry("Volume - Quadlet image (.image) not found", "image-not-found.quadlet.volume", "converting \"image-not-found.quadlet.volume\": requested Quadlet image not-found.image was not found"),

		Entry("Kube - User Remap Manual", "remap-manual.kube", "converting \"remap-manual.kube\": RemapUsers=manual is not supported"),
		Entry("Kube - Multiple Yaml and SetWorkingDir=yaml", "multiple-yaml-set-working-dir-yaml.kube", "converting \"multiple-yaml-set-working-dir-yaml.kube\": SetWorkingDirectory=yaml is only supported when a single Yaml key is provided"),

		Entry("Network - Gateway not enough Subnet", "gateway.less-subnet.network", "converting \"gateway.less-subnet.network\": cannot set more gateways than subnets"),
		Entry("Network - Gateway without Subnet", "gateway.no-subnet.network", "converting \"gateway.no-subnet.network\": cannot set gateway or range without subnet"),
		Entry("Network - Range not enough Subnet", "range.less-subnet.network", "converting \"range.less-subnet.network\": cannot set more ranges than subnets"),
		Entry("Network - Range without Subnet", "range.no-subnet.network", "converting \"range.no-subnet.network\": cannot set gateway or range without subnet"),

		Entry("Image - No Image", "no-image.image", "converting \"no-image.image\": no Image key specified"),

		Entry("Build - File Key relative no WD", "file-rel-no-wd.build", "converting \"file-rel-no-wd.build\": relative path in File key requires SetWorkingDirectory key to be set"),
		Entry("Build - Neither WorkingDirectory nor File Key", "neither-workingdirectory-nor-file.build", "converting \"neither-workingdirectory-nor-file.build\": neither SetWorkingDirectory, nor File key specified"),
		Entry("Build - No ImageTag Key", "no-imagetag.build", "converting \"no-imagetag.build\": no ImageTag key specified"),
		Entry("emptyline.container", "emptyline.container", "converting \"emptyline.container\": no Image or Rootfs key specified"),

		Entry("Mount - Missing source=...", "mount-source-missing.container", "converting \"mount-source-missing.container\": source parameter is required for mount type image"),
	)

	DescribeTable("Running success quadlet with ServiceName test case",
		func(fileName, serviceName string) {
			runQuadletTestCaseWithServiceName(fileName, 0, "", serviceName)
		},
		Entry("Build", "service-name.build", "basic"),
		Entry("Container", "service-name.container", "basic"),
		Entry("Image", "service-name.image", "basic"),
		Entry("Kube", "service-name.kube", "basic"),
		Entry("Network", "service-name.network", "basic"),
		Entry("Pod", "service-name.pod", "basic"),
		Entry("Volume", "service-name.volume", "basic"),
	)

	DescribeTable("Running quadlet success test case with dependencies",
		func(fileName string, dependencyFiles []string) {
			// Write additional files this test depends on to the quadlet dir
			for _, dependencyFileName := range dependencyFiles {
				dependencyTestCase := loadQuadletTestcase(filepath.Join("quadlet", dependencyFileName))
				err = os.WriteFile(filepath.Join(quadletDir, dependencyFileName), dependencyTestCase.data, 0o644)
				Expect(err).ToNot(HaveOccurred())
			}

			runSuccessQuadletTestCase(fileName)
		},
		Entry("Container - Mount", "mount.container", []string{"basic.image", "basic.volume", "basic.artifact"}),
		Entry("Container - Quadlet Network", "network.quadlet.container", []string{"basic.network"}),
		Entry("Container - Quadlet Volume", "volume.container", []string{"basic.volume"}),
		Entry("Container - Mount overriding service name", "mount.servicename.container", []string{"service-name.volume"}),
		Entry("Container - Quadlet Network overriding service name", "network.quadlet.servicename.container", []string{"service-name.network"}),
		Entry("Container - Quadlet Volume overriding service name", "volume.servicename.container", []string{"service-name.volume"}),
		Entry("Container - Quadlet build with multiple tags", "build.multiple-tags.container", []string{"multiple-tags.build"}),
		Entry("Container - Artifact Mount", "artifact-mount.container", []string{"basic.artifact"}),
		Entry("Container - Reuse another container's network", "network.reuse.container", []string{"basic.container"}),
		Entry("Container - Reuse another named container's network", "network.reuse.name.container", []string{"name.container"}),
		Entry("Container - Reuse another container's network", "a.network.reuse.container", []string{"basic.container"}),
		Entry("Container - Reuse another named container's network", "a.network.reuse.name.container", []string{"name.container"}),
		Entry(
			"Container - Dependency between quadlet units",
			"dependent.container",
			[]string{
				"basic.artifact",
				"basic.build",
				"basic.container",
				"basic.image",
				"basic.kube",
				"basic.network",
				"basic.pod",
				"basic.volume",
			},
		),
		Entry(
			"Container - Template with Volume Template dependency",
			"template-dependency@.container",
			[]string{
				"template-dependency@.volume",
				"template-dependency@.network",
			},
		),

		Entry("Volume - Quadlet image (.build)", "build.quadlet.volume", []string{"basic.build"}),
		Entry("Volume - Quadlet image (.image)", "image.quadlet.volume", []string{"basic.image"}),
		Entry("Volume - Quadlet image (.build) overriding service name", "build.quadlet.servicename.volume", []string{"service-name.build"}),
		Entry("Volume - Quadlet image (.image) overriding service name", "image.quadlet.servicename.volume", []string{"service-name.image"}),
		Entry(
			"Volume - Dependency between quadlet units",
			"dependent.volume",
			[]string{
				"basic.build",
				"basic.container",
				"basic.image",
				"basic.kube",
				"basic.network",
				"basic.pod",
				"basic.volume",
			},
		),

		Entry("Kube - Quadlet Network", "network.quadlet.kube", []string{"basic.network"}),
		Entry("Kube - Quadlet Network overriding service name", "network.quadlet.servicename.kube", []string{"service-name.network"}),
		Entry(
			"Kube - Dependency between quadlet units",
			"dependent.kube",
			[]string{
				"basic.build",
				"basic.container",
				"basic.image",
				"basic.kube",
				"basic.network",
				"basic.pod",
				"basic.volume",
			},
		),

		Entry("Build - Network Key quadlet", "network.quadlet.build", []string{"basic.network"}),
		Entry("Build - Volume Key", "volume.build", []string{"basic.volume"}),
		Entry("Build - Volume Key quadlet", "volume.quadlet.build", []string{"basic.volume"}),
		Entry("Build - Network Key quadlet overriding service name", "network.quadlet.servicename.build", []string{"service-name.network"}),
		Entry("Build - Volume Key quadlet overriding service name", "volume.quadlet.servicename.build", []string{"service-name.volume"}),
		Entry(
			"Build - Dependency between quadlet units",
			"dependent.build",
			[]string{
				"basic.artifact",
				"basic.build",
				"basic.container",
				"basic.image",
				"basic.kube",
				"basic.network",
				"basic.pod",
				"basic.volume",
			},
		),

		Entry("Pod - Quadlet Network", "network.quadlet.pod", []string{"basic.network"}),
		Entry("Pod - Quadlet Volume", "volume.pod", []string{"basic.volume"}),
		Entry("Pod - Quadlet Network overriding service name", "network.servicename.quadlet.pod", []string{"service-name.network"}),
		Entry("Pod - Quadlet Volume overriding service name", "volume.servicename.pod", []string{"service-name.volume"}),
		Entry(
			"Pod - Do not autostart a container with pod",
			"startwithpod.pod",
			[]string{
				"startwithpod_no.container",
				"startwithpod_yes.container",
				"startwithpod@.container",
				"startwithpod@foo.container",
			},
		),
		Entry(
			"Pod - Dependency between quadlet units",
			"dependent.pod",
			[]string{
				"basic.artifact",
				"basic.build",
				"basic.container",
				"basic.image",
				"basic.kube",
				"basic.network",
				"basic.pod",
				"basic.volume",
			},
		),

		Entry(
			"Image - Dependency between quadlet units",
			"dependent.image",
			[]string{
				"basic.artifact",
				"basic.build",
				"basic.container",
				"basic.image",
				"basic.kube",
				"basic.network",
				"basic.pod",
				"basic.volume",
			},
		),

		Entry(
			"Network - Dependency between quadlet units",
			"dependent.network",
			[]string{
				"basic.artifact",
				"basic.build",
				"basic.container",
				"basic.image",
				"basic.kube",
				"basic.network",
				"basic.pod",
				"basic.volume",
			},
		),
	)
})
