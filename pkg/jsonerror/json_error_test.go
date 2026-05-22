package jsonerror

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errTest         = errors.New("test error")
	errOther        = errors.New("other error")
	errStatic       = errors.New("static")
	errSpecialChars = errors.New(`message with "quotes" and \backslash\`)
)

func TestNew(t *testing.T) {
	t.Parallel()

	jsonErr := New(errTest)
	require.NotNil(t, jsonErr)
	assert.ErrorIs(t, jsonErr.Err(), errTest)
}

func TestNewNil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, New(nil))
}

func TestErr(t *testing.T) {
	t.Parallel()

	jsonErr := New(errTest)
	require.Error(t, jsonErr.Err())
	assert.Equal(t, "test error", jsonErr.Err().Error())
}

func TestErrNilReceiver(t *testing.T) {
	t.Parallel()

	var jsonErr *JSONError
	assert.NoError(t, jsonErr.Err())
}

func TestError(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "test error", New(errTest).Error())
}

func TestErrorNilReceiver(t *testing.T) {
	t.Parallel()

	var jsonErr *JSONError
	assert.Empty(t, jsonErr.Error())
}

func TestErrorNilInnerErr(t *testing.T) {
	t.Parallel()

	jsonErr := &JSONError{err: nil}
	assert.Empty(t, jsonErr.Error())
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(New(errTest))
	require.NoError(t, err)
	assert.Equal(t, `"test error"`, string(data))
}

func TestMarshalJSONNilReceiver(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal((*JSONError)(nil))
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestMarshalJSONNilInnerErr(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(&JSONError{err: nil})
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	jsonErr := &JSONError{}
	err := json.Unmarshal([]byte(`"something went wrong"`), jsonErr)
	require.NoError(t, err)
	require.Error(t, jsonErr.Err())
	assert.Equal(t, "something went wrong", jsonErr.Err().Error())
}

func TestUnmarshalJSONNull(t *testing.T) {
	t.Parallel()

	jsonErr := New(errTest)
	err := json.Unmarshal([]byte("null"), jsonErr)
	require.NoError(t, err)
	assert.NoError(t, jsonErr.Err())
}

func TestUnmarshalJSONInvalid(t *testing.T) {
	t.Parallel()

	jsonErr := &JSONError{}
	assert.Error(t, json.Unmarshal([]byte(`123`), jsonErr))
}

func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := New(errOther)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	restored := &JSONError{}
	err = json.Unmarshal(data, restored)
	require.NoError(t, err)
	assert.Equal(t, original.Error(), restored.Error())
}

func TestJSONRoundTripNull(t *testing.T) {
	t.Parallel()

	var original *JSONError

	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))

	restored := New(errStatic)
	err = json.Unmarshal(data, restored)
	require.NoError(t, err)
	assert.NoError(t, restored.Err())
}

func TestMarshalJSONSpecialChars(t *testing.T) {
	t.Parallel()

	jsonErr := New(errSpecialChars)
	data, err := json.Marshal(jsonErr)
	require.NoError(t, err)

	restored := &JSONError{}
	err = json.Unmarshal(data, restored)
	require.NoError(t, err)
	assert.Equal(t, jsonErr.Error(), restored.Error())
}
