//go:build !remote

package utils

import (
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/types"
)

func TestErrorEncoderFuncOmit(t *testing.T) {
	data, err := json.Marshal(struct {
		Err  error   `json:"err,omitempty"`
		Errs []error `json:"errs,omitempty"`
	}{})
	if err != nil {
		t.Fatal(err)
	}

	dataAsMap := make(map[string]any)
	err = json.Unmarshal(data, &dataAsMap)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := dataAsMap["err"]
	if ok {
		t.Errorf("the `err` field should have been omitted")
	}
	_, ok = dataAsMap["errs"]
	if ok {
		t.Errorf("the `errs` field should have been omitted")
	}

	dataAsMap = make(map[string]any)
	data, err = json.Marshal(struct {
		Err  error   `json:"err"`
		Errs []error `json:"errs"`
	}{})
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(data, &dataAsMap)
	if err != nil {
		t.Fatal(err)
	}

	_, ok = dataAsMap["err"]
	if !ok {
		t.Errorf("the `err` field shouldn't have been omitted")
	}
	_, ok = dataAsMap["errs"]
	if !ok {
		t.Errorf("the `errs` field shouldn't have been omitted")
	}
}

func TestWriteJSONNoHTMLEscape(t *testing.T) {
	// Test that WriteJSON does not HTML-escape JSON content
	// This test verifies the fix for issue #17769

	recorder := httptest.NewRecorder()

	// Test data with characters that would be HTML-escaped
	testData := map[string]string{
		"message": "Hello <world> & \"friends\"",
		"script":  "<script>alert('test')</script>",
		"url":     "https://example.com/path?param=value&other=<test>",
	}

	WriteJSON(recorder, 200, testData)

	// Check response headers
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Check that response contains unescaped characters
	body := recorder.Body.String()

	// These characters should NOT be HTML-escaped in JSON responses
	// (but quotes are still properly JSON-escaped)
	expectedUnescaped := []string{
		"<world>",
		"&",
		"\\\"friends\\\"", // JSON-escaped quotes, not HTML-escaped
		"<script>",
		"<test>",
	}

	for _, expected := range expectedUnescaped {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected unescaped string '%s' in response body, got: %s", expected, body)
		}
	}

	// Verify we can parse the JSON back
	var parsed map[string]string
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	// Verify the data matches what we sent
	if !reflect.DeepEqual(parsed, testData) {
		t.Errorf("Parsed message doesn't match original: got %v, want %v", parsed, testData)
	}
}

func TestParseOptionalJSONField(t *testing.T) {
	t.Run("field exists with valid JSON", func(t *testing.T) {
		jsonStr := `["item1", "item2"]`
		queryValues := url.Values{"testfield": []string{jsonStr}}
		var target []string

		err := ParseOptionalJSONField(jsonStr, "testfield", queryValues, &target)

		assert.NoError(t, err)
		assert.Equal(t, []string{"item1", "item2"}, target)
	})

	t.Run("field does not exist", func(t *testing.T) {
		jsonStr := `["item1", "item2"]`
		queryValues := url.Values{"otherfield": []string{jsonStr}}
		var target []string
		originalLen := len(target)

		err := ParseOptionalJSONField(jsonStr, "testfield", queryValues, &target)

		assert.NoError(t, err)
		assert.Len(t, target, originalLen) // Should remain unchanged
	})

	t.Run("field exists with invalid JSON", func(t *testing.T) {
		jsonStr := `{invalid json}`
		queryValues := url.Values{"testfield": []string{jsonStr}}
		var target map[string]string

		err := ParseOptionalJSONField(jsonStr, "testfield", queryValues, &target)

		assert.Error(t, err)
	})

	t.Run("complex object parsing", func(t *testing.T) {
		jsonStr := `{"buildargs": {"ARG1": "value1", "ARG2": "value2"}}`
		queryValues := url.Values{"config": []string{jsonStr}}
		var target map[string]map[string]string

		err := ParseOptionalJSONField(jsonStr, "config", queryValues, &target)

		assert.NoError(t, err)
		expected := map[string]map[string]string{
			"buildargs": {"ARG1": "value1", "ARG2": "value2"},
		}
		assert.Equal(t, expected, target)
	})
}

func TestParseOptionalBool(t *testing.T) {
	t.Run("field exists with true value", func(t *testing.T) {
		queryValues := url.Values{"testfield": []string{"true"}}
		result, found := ParseOptionalBool(true, "testfield", queryValues)

		assert.True(t, found)
		assert.Equal(t, types.NewOptionalBool(true), result)
	})

	t.Run("field exists with false value", func(t *testing.T) {
		queryValues := url.Values{"testfield": []string{"false"}}
		result, found := ParseOptionalBool(false, "testfield", queryValues)

		assert.True(t, found)
		assert.Equal(t, types.NewOptionalBool(false), result)
	})

	t.Run("field does not exist", func(t *testing.T) {
		queryValues := url.Values{"otherfield": []string{"value"}}
		result, found := ParseOptionalBool(true, "testfield", queryValues)

		assert.False(t, found)
		var empty types.OptionalBool
		assert.Equal(t, empty, result)
	})

	t.Run("multiple values for same field", func(t *testing.T) {
		queryValues := url.Values{"testfield": []string{"true", "false"}}
		result, found := ParseOptionalBool(true, "testfield", queryValues)

		assert.True(t, found)
		assert.Equal(t, types.NewOptionalBool(true), result)
	})
}

func TestParseJSONOptionalSlice(t *testing.T) {
	t.Run("parameter exists with valid JSON array", func(t *testing.T) {
		value := `["item1", "item2", "item3"]`
		queryValues := url.Values{"testparam": []string{value}}

		result, err := ParseJSONOptionalSlice(value, queryValues, "testparam")

		assert.NoError(t, err)
		assert.Equal(t, []string{"item1", "item2", "item3"}, result)
	})

	t.Run("parameter does not exist", func(t *testing.T) {
		value := `["item1", "item2"]`
		queryValues := url.Values{"otherparam": []string{value}}

		result, err := ParseJSONOptionalSlice(value, queryValues, "testparam")

		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("parameter exists with invalid JSON", func(t *testing.T) {
		value := `[invalid json]`
		queryValues := url.Values{"testparam": []string{value}}

		result, err := ParseJSONOptionalSlice(value, queryValues, "testparam")

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("parameter exists with empty array", func(t *testing.T) {
		value := `[]`
		queryValues := url.Values{"testparam": []string{value}}

		result, err := ParseJSONOptionalSlice(value, queryValues, "testparam")

		assert.NoError(t, err)
		assert.Equal(t, []string{}, result)
	})

	t.Run("parameter exists with single item", func(t *testing.T) {
		value := `["single"]`
		queryValues := url.Values{"testparam": []string{value}}

		result, err := ParseJSONOptionalSlice(value, queryValues, "testparam")

		assert.NoError(t, err)
		assert.Equal(t, []string{"single"}, result)
	})
}

func TestNewBuildResponseSender(t *testing.T) {
	t.Run("normal operation", func(t *testing.T) {
		w := httptest.NewRecorder()
		sender := NewBuildResponseSender(w)

		assert.NotNil(t, sender)
		assert.NotNil(t, sender.encoder)
		assert.NotNil(t, sender.flusher)
	})
}

func TestResponseSender_Send(t *testing.T) {
	w := httptest.NewRecorder()
	sender := NewBuildResponseSender(w)

	testResponse := map[string]any{
		"stream": "test message",
		"id":     "12345",
	}

	sender.Send(testResponse)

	// Check that the response was written
	assert.NotEmpty(t, w.Body.String())

	// Verify the JSON was properly encoded
	var decoded map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "test message", decoded["stream"])
	assert.Equal(t, "12345", decoded["id"])
}

func TestResponseSender_SendBuildStream(t *testing.T) {
	w := httptest.NewRecorder()
	sender := NewBuildResponseSender(w)

	message := "Building step 1/5"
	sender.SendBuildStream(message)

	// Verify the response structure
	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, message, response["stream"])
}

func TestResponseSender_SendBuildError(t *testing.T) {
	w := httptest.NewRecorder()
	sender := NewBuildResponseSender(w)

	errorMessage := "Build failed: syntax error"
	sender.SendBuildError(errorMessage)

	// Verify the response structure
	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// ErrorMessage field maps to "error" in JSON
	assert.Equal(t, errorMessage, response["error"])
	assert.NotNil(t, response["errorDetail"])

	// Check the nested error structure (errorDetail)
	errorObj := response["errorDetail"].(map[string]any)
	assert.Equal(t, errorMessage, errorObj["message"])
}

func TestResponseSender_SendBuildAux(t *testing.T) {
	w := httptest.NewRecorder()
	sender := NewBuildResponseSender(w)

	auxData := []byte(`{"ID":"sha256:1234567890abcdef"}`)
	sender.SendBuildAux(auxData)

	// Verify the response structure
	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.NotNil(t, response["aux"])

	// The aux field should contain the raw JSON data
	auxBytes, err := json.Marshal(response["aux"])
	assert.NoError(t, err)
	assert.Equal(t, auxData, auxBytes)
}

func TestResponseSender_SendInvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	sender := NewBuildResponseSender(w)

	// Create a value that can't be JSON encoded (contains channels)
	invalidValue := map[string]any{
		"channel": make(chan string),
	}

	// This should not panic, but should log a warning
	sender.Send(invalidValue)

	// The body should be empty since encoding failed
	assert.Empty(t, w.Body.String())
}

// Test integration scenarios
func TestParseOptionalJSONFieldIntegration(t *testing.T) {
	// Simulate a real query parameter scenario
	queryValues := url.Values{
		"buildargs": []string{`{"ARG1":"value1","ARG2":"value2"}`},
		"labels":    []string{`{"app":"myapp","version":"1.0"}`},
	}

	t.Run("parse build args", func(t *testing.T) {
		var buildArgs map[string]string
		err := ParseOptionalJSONField(queryValues.Get("buildargs"), "buildargs", queryValues, &buildArgs)

		require.NoError(t, err)
		expected := map[string]string{"ARG1": "value1", "ARG2": "value2"}
		assert.Equal(t, expected, buildArgs)
	})

	t.Run("parse labels", func(t *testing.T) {
		var labels map[string]string
		err := ParseOptionalJSONField(queryValues.Get("labels"), "labels", queryValues, &labels)

		require.NoError(t, err)
		expected := map[string]string{"app": "myapp", "version": "1.0"}
		assert.Equal(t, expected, labels)
	})

	t.Run("parse non-existent field", func(t *testing.T) {
		var nonExistent map[string]string
		err := ParseOptionalJSONField("", "nonexistent", queryValues, &nonExistent)

		assert.NoError(t, err)
		assert.Nil(t, nonExistent)
	})
}

func TestResponseSenderFlushBehavior(t *testing.T) {
	// Create a custom ResponseWriter that tracks flush calls
	flushCalled := false
	w := &testResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		onFlush: func() {
			flushCalled = true
		},
	}

	sender := NewBuildResponseSender(w)
	sender.Send(map[string]string{"test": "message"})

	assert.True(t, flushCalled, "Flush should have been called")
}

// Helper type for testing flush behavior
type testResponseWriter struct {
	*httptest.ResponseRecorder
	onFlush func()
}

func (t *testResponseWriter) Flush() {
	if t.onFlush != nil {
		t.onFlush()
	}
}
