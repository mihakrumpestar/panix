package atomictimeandstate

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/jsonerror"
	"github.com/pkg/errors"
)

var (
	errTest          = errors.New("test error")
	errFirst         = errors.New("first")
	errSecond        = errors.New("second")
	errRoundTrip     = errors.New("round trip error")
	errConcurrent    = errors.New("concurrent error")
	errJSONErrorTest = errors.New("test")
)

func TestNew(t *testing.T) {
	t.Parallel()

	tas := New()
	require.NotNil(t, tas, "New() returned nil")

	loaded := tas.Load()
	require.NotNil(t, loaded, "Load() returned nil after New()")
	assert.False(t, loaded.HasStarted(), "new AtomicTimeAndState should not have started")
	assert.False(t, loaded.IsFinished(), "new AtomicTimeAndState should not be finished")
}

func TestTimeAndState_HasStarted(t *testing.T) {
	t.Parallel()

	tas := &TimeAndState{}
	assert.False(t, tas.HasStarted(), "zero TimeAndState should not have started")

	tas.StartTime = time.Now()
	assert.True(t, tas.HasStarted(), "TimeAndState with StartTime set should have started")
}

func TestTimeAndState_IsFinished(t *testing.T) {
	t.Parallel()

	tas := &TimeAndState{}
	assert.False(t, tas.IsFinished(), "zero TimeAndState should not be finished")

	tas.EndTime = time.Now()
	assert.True(t, tas.IsFinished(), "TimeAndState with EndTime set should be finished")
}

func TestTimeAndState_Duration(t *testing.T) {
	t.Parallel()

	tas := &TimeAndState{}

	_, err := tas.Duration()
	require.Error(t, err, "Duration() on unstarted timer should return error")

	tas.StartTime = time.Now()

	_, err = tas.Duration()
	require.Error(t, err, "Duration() on unfinished timer should return error")

	tas.EndTime = time.Now()
	tas.DurationCache = 5 * time.Second

	dur, err := tas.Duration()
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, dur, "Duration()")
}

func TestStartTimer(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	loaded := tas.Load()
	assert.True(t, loaded.HasStarted(), "StartTimer() should set StartTime")
	assert.True(t, loaded.live, "StartTimer() should set live=true")
}

func TestStartTimerIdempotent(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	firstStart := tas.Load().StartTime

	time.Sleep(1 * time.Millisecond)

	tas.StartTimer()

	secondStart := tas.Load().StartTime

	assert.True(t, firstStart.Equal(secondStart), "StartTimer() should be idempotent, StartTime changed on second call")
}

func TestEndTimerWithError(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	testErr := errTest
	tas.EndTimerWithError(testErr)

	loaded := tas.Load()
	assert.True(t, loaded.IsFinished(), "EndTimerWithError() should set EndTime")
	assert.Positive(t, loaded.DurationCache, "EndTimerWithError() should set DurationCache > 0")
	require.NotNil(t, loaded.EndError, "EndTimerWithError() should set EndError")
	assert.Equal(t, errTest.Error(), loaded.EndError.Error(), "EndError.Error()")
}

func TestEndTimerWithErrorNil(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()
	tas.EndTimerWithError(nil)

	loaded := tas.Load()
	assert.True(t, loaded.IsFinished(), "EndTimerWithError(nil) should still set EndTime")
	assert.Nil(t, loaded.EndError, "EndTimerWithError(nil) should set EndError to nil")
}

func TestEndTimerWithErrorIdempotent(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()
	tas.EndTimerWithError(errFirst)

	firstEnd := tas.Load().EndTime
	firstErr := tas.Load().EndError.Error()

	time.Sleep(1 * time.Millisecond)

	tas.EndTimerWithError(errSecond)

	secondEnd := tas.Load().EndTime
	secondErr := tas.Load().EndError.Error()

	assert.True(t, firstEnd.Equal(secondEnd), "EndTimerWithError() should be idempotent, EndTime changed on second call")
	assert.Equal(t, firstErr, secondErr, "EndError changed on second call")
}

func TestDurationOrElapsedTime(t *testing.T) {
	t.Parallel()

	tas := New()

	_, err := tas.DurationOrElapsedTime()
	require.Error(t, err, "DurationOrElapsedTime() on unstarted timer should return error")

	tas.StartTimer()

	dur, err := tas.DurationOrElapsedTime()
	require.NoError(t, err, "DurationOrElapsedTime() after StartTimer")
	assert.Positive(t, dur, "DurationOrElapsedTime() after StartTimer should be > 0")
}

func TestDurationOrElapsedTimeAfterEnd(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()
	tas.EndTimerWithError(nil)

	dur, err := tas.DurationOrElapsedTime()
	require.NoError(t, err, "DurationOrElapsedTime() after EndTimer")

	loaded := tas.Load()
	assert.Equal(t, loaded.DurationCache, dur, "DurationOrElapsedTime()")
}

func TestDurationOrElapsedTimeNotLive(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	tas.Update(func(t *TimeAndState) {
		t.live = false
		t.DurationCache = 3 * time.Second
	})

	dur, err := tas.DurationOrElapsedTime()
	require.NoError(t, err, "DurationOrElapsedTime() returned error")
	assert.Equal(t, 3*time.Second, dur, "DurationOrElapsedTime() (cached, not live)")
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	data, err := json.Marshal(tas)
	require.NoError(t, err, "MarshalJSON() error")

	var result map[string]any

	err = json.Unmarshal(data, &result)
	require.NoError(t, err, "unmarshal result error")

	assert.Contains(t, result, "start_time", "MarshalJSON() missing start_time field")
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 0, 0, 5, 0, time.UTC)

	data := map[string]any{
		"start_time": startTime.Format(time.RFC3339Nano),
		"end_time":   endTime.Format(time.RFC3339Nano),
		"duration":   5_000_000_000,
	}

	jsonData, err := json.Marshal(data)
	require.NoError(t, err, "marshal test data error")

	tas := New()

	err = json.Unmarshal(jsonData, tas)
	require.NoError(t, err, "UnmarshalJSON() error")

	loaded := tas.Load()
	assert.True(t, loaded.HasStarted(), "UnmarshalJSON() should set StartTime")
	assert.True(t, loaded.IsFinished(), "UnmarshalJSON() should set EndTime")
}

func TestUnmarshalJSONWithEndError(t *testing.T) {
	t.Parallel()

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 0, 0, 5, 0, time.UTC)

	data := map[string]any{
		"start_time": startTime.Format(time.RFC3339Nano),
		"end_time":   endTime.Format(time.RFC3339Nano),
		"duration":   5_000_000_000,
		"end_error":  "something went wrong",
	}

	jsonData, err := json.Marshal(data)
	require.NoError(t, err, "marshal test data error")

	tas := New()

	err = json.Unmarshal(jsonData, tas)
	require.NoError(t, err, "UnmarshalJSON() error")

	loaded := tas.Load()
	require.NotNil(t, loaded.EndError, "UnmarshalJSON() should set EndError")
	assert.Equal(t, "something went wrong", loaded.EndError.Error(), "EndError.Error()")
}

func TestUnmarshalJSONNilReceiver(t *testing.T) {
	t.Parallel()

	var tas *AtomicTimeAndState

	err := json.Unmarshal([]byte(`{"start_time":"2024-01-01T00:00:00Z"}`), tas)
	assert.Error(t, err, "UnmarshalJSON on nil receiver should return error")
}

func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()
	tas.EndTimerWithError(errRoundTrip)

	data, err := json.Marshal(tas)
	require.NoError(t, err, "MarshalJSON() error")

	tas2 := New()

	err = json.Unmarshal(data, tas2)
	require.NoError(t, err, "UnmarshalJSON() error")

	original := tas.Load()
	restored := tas2.Load()

	assert.Equal(t, original.DurationCache, restored.DurationCache, "DurationCache mismatch")

	require.NotNil(t, original.EndError, "original EndError should not be nil")
	require.NotNil(t, restored.EndError, "restored EndError should not be nil")
	assert.Equal(t, original.EndError.Error(), restored.EndError.Error(), "EndError mismatch")
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	tas := New()

	var waitGroup sync.WaitGroup

	numWriters := 10
	numOps := 100

	for writerID := range numWriters {
		waitGroup.Add(1)

		go func(wid int) {
			defer waitGroup.Done()

			for range numOps {
				switch wid % 3 {
				case 0:
					tas.StartTimer()
				case 1:
					tas.EndTimerWithError(errConcurrent)
				default:
					_, _ = tas.DurationOrElapsedTime()
				}
			}
		}(writerID)
	}

	for range numWriters {
		waitGroup.Go(func() {
			for range numOps {
				_ = tas.Load()
			}
		})
	}

	waitGroup.Wait()
}

func TestJSONErrorNew(t *testing.T) {
	t.Parallel()

	jsonErr := jsonerror.New(errJSONErrorTest)
	require.NotNil(t, jsonErr, "jsonerror.New(non-nil) returned nil")
	assert.Equal(t, errJSONErrorTest.Error(), jsonErr.Error(), "Error()")
}

func TestJSONErrorNewNil(t *testing.T) {
	t.Parallel()

	je := jsonerror.New(nil)
	assert.Nil(t, je, "jsonerror.New(nil) should return nil")
}
