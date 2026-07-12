package utils

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

type podmanSession interface {
	ExitCode() int
	ErrorToString() string
}

type ExitMatcher struct {
	types.GomegaMatcher
	ExpectedExitCode    int
	ExitCode            int
	ExpectedStderr      string
	ExpectedStderrRegex string
	msg                 string
}

// ExitWithError checks both exit code and stderr, fails if either does not match
// Modeled after the gomega Exit() matcher and also operates on sessions.
func ExitWithError(expectExitCode int, expectStderr string) *ExitMatcher {
	return &ExitMatcher{ExpectedExitCode: expectExitCode, ExpectedStderr: expectStderr}
}

// ExitWithErrorRegex checks both exit code and the stderr regex, fails if either does not match
// Modeled after the gomega Exit() matcher and also operates on sessions.
func ExitWithErrorRegex(expectExitCode int, expectStderrRegex string) *ExitMatcher {
	return &ExitMatcher{ExpectedExitCode: expectExitCode, ExpectedStderrRegex: expectStderrRegex}
}

// Match follows gexec.Matcher interface.
func (matcher *ExitMatcher) Match(actual any) (success bool, err error) {
	session, ok := actual.(podmanSession)
	if !ok {
		return false, fmt.Errorf("ExitWithError must be passed a gexec.Exiter (Missing method ExitCode() int) Got:\n#{format.Object(actual, 1)}")
	}

	matcher.ExitCode = session.ExitCode()
	if matcher.ExitCode == -1 {
		matcher.msg = "Expected process to exit. It did not."
		return false, nil
	}

	// Check exit code first. If it's not what we want, there's no point
	// in checking error substrings
	if matcher.ExitCode != matcher.ExpectedExitCode {
		matcher.msg = fmt.Sprintf("Command exited with status %d (expected %d)", matcher.ExitCode, matcher.ExpectedExitCode)
		return false, nil
	}

	switch {
	case matcher.ExpectedStderrRegex != "":
		matched, err := regexp.MatchString(matcher.ExpectedStderrRegex, session.ErrorToString())
		if err != nil {
			matcher.msg = fmt.Sprintf("Invalid regex pattern: %s", err)
			return false, err
		}
		if !matched {
			matcher.msg = fmt.Sprintf("Command exited %d as expected, but stderr did not match regex '%s'", matcher.ExitCode, matcher.ExpectedStderrRegex)
			return false, nil
		}
	case matcher.ExpectedStderr != "":
		if !strings.Contains(session.ErrorToString(), matcher.ExpectedStderr) {
			matcher.msg = fmt.Sprintf("Command exited %d as expected, but did not emit '%s'", matcher.ExitCode, matcher.ExpectedStderr)
			return false, nil
		}
	default:
		if session.ErrorToString() != "" {
			matcher.msg = "Command exited with expected exit status, but emitted unwanted stderr"
			return false, nil
		}
	}

	return true, nil
}

func (matcher *ExitMatcher) FailureMessage(_ any) (message string) {
	return matcher.msg
}

func (matcher *ExitMatcher) NegatedFailureMessage(_ any) (message string) {
	panic("There is no conceivable reason to call Not(ExitWithError) !")
}

func (matcher *ExitMatcher) MatchMayChangeInTheFuture(actual any) bool {
	session, ok := actual.(*gexec.Session)
	if ok {
		return session.ExitCode() == -1
	}
	return true
}

// ExitCleanly asserts that a PodmanSession exits 0 and with no stderr
// Consider using PodmanTestIntegration.PodmanExitCleanly instead of directly using this matcher.
func ExitCleanly() types.GomegaMatcher {
	return &exitCleanlyMatcher{}
}

type exitCleanlyMatcher struct {
	msg string
}

func (matcher *exitCleanlyMatcher) Match(actual any) (success bool, err error) {
	session, ok := actual.(podmanSession)
	if !ok {
		return false, fmt.Errorf("ExitCleanly must be passed a PodmanSession; Got:\n %+v\n%q", actual, format.Object(actual, 1))
	}

	exitcode := session.ExitCode()
	stderr := session.ErrorToString()
	if exitcode != 0 {
		matcher.msg = fmt.Sprintf("Command failed with exit status %d", exitcode)
		if stderr != "" {
			matcher.msg += ". See above for error message."
		}
		return false, nil
	}

	// Exit status is 0. Now check for anything on stderr
	if stderr != "" {
		matcher.msg = fmt.Sprintf("Unexpected warnings seen on stderr: %q", stderr)
		return false, nil
	}

	return true, nil
}

func (matcher *exitCleanlyMatcher) FailureMessage(_ any) (message string) {
	return matcher.msg
}

func (matcher *exitCleanlyMatcher) NegatedFailureMessage(_ any) (message string) {
	// FIXME - I see no situation in which we could ever want this?
	return matcher.msg + " (NOT!)"
}

type ValidJSONMatcher struct {
	types.GomegaMatcher
}

func BeValidJSON() *ValidJSONMatcher {
	return &ValidJSONMatcher{}
}

func (matcher *ValidJSONMatcher) Match(actual any) (success bool, err error) {
	s, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("ValidJSONMatcher expects a string, not %q", actual)
	}

	var i any
	if err := json.Unmarshal([]byte(s), &i); err != nil {
		return false, err
	}
	return true, nil
}

func (matcher *ValidJSONMatcher) FailureMessage(actual any) (message string) {
	return format.Message(actual, "to be valid JSON")
}

func (matcher *ValidJSONMatcher) NegatedFailureMessage(actual any) (message string) {
	return format.Message(actual, "to _not_ be valid JSON")
}
