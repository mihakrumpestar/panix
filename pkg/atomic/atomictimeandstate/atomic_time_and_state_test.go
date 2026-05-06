package atomictimeandstate

import (
	"encoding/json"
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
	if tas == nil {
		t.Fatal("New() returned nil")
	}

	loaded := tas.Load()
	if loaded == nil {
		t.Fatal("Load() returned nil after New()")
	}

	if loaded.HasStarted() {
		t.Error("new AtomicTimeAndState should not have started")
	}

	if loaded.IsFinished() {
		t.Error("new AtomicTimeAndState should not be finished")
	}
}

func TestTimeAndState_HasStarted(t *testing.T) {
	t.Parallel()

	tas := &TimeAndState{}
	if tas.HasStarted() {
		t.Error("zero TimeAndState should not have started")
	}

	tas.StartTime = time.Now()
	if !tas.HasStarted() {
		t.Error("TimeAndState with StartTime set should have started")
	}
}

func TestTimeAndState_IsFinished(t *testing.T) {
	t.Parallel()

	tas := &TimeAndState{}
	if tas.IsFinished() {
		t.Error("zero TimeAndState should not be finished")
	}

	tas.EndTime = time.Now()
	if !tas.IsFinished() {
		t.Error("TimeAndState with EndTime set should be finished")
	}
}

func TestTimeAndState_Duration(t *testing.T) {
	t.Parallel()

	tas := &TimeAndState{}

	_, err := tas.Duration()
	if err == nil {
		t.Error("Duration() on unstarted timer should return error")
	}

	tas.StartTime = time.Now()

	_, err = tas.Duration()
	if err == nil {
		t.Error("Duration() on unfinished timer should return error")
	}

	tas.EndTime = time.Now()
	tas.DurationCache = 5 * time.Second

	dur, err := tas.Duration()
	if err != nil {
		t.Errorf("Duration() returned error: %v", err)
	}

	if dur != 5*time.Second {
		t.Errorf("Duration() = %v, want 5s", dur)
	}
}

func TestStartTimer(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	loaded := tas.Load()
	if !loaded.HasStarted() {
		t.Error("StartTimer() should set StartTime")
	}

	if !loaded.live {
		t.Error("StartTimer() should set live=true")
	}
}

func TestStartTimerIdempotent(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	firstStart := tas.Load().StartTime

	time.Sleep(1 * time.Millisecond)

	tas.StartTimer()

	secondStart := tas.Load().StartTime

	if !firstStart.Equal(secondStart) {
		t.Error("StartTimer() should be idempotent, StartTime changed on second call")
	}
}

func TestEndTimerWithError(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	testErr := errTest
	tas.EndTimerWithError(testErr)

	loaded := tas.Load()
	if !loaded.IsFinished() {
		t.Error("EndTimerWithError() should set EndTime")
	}

	if loaded.DurationCache <= 0 {
		t.Error("EndTimerWithError() should set DurationCache > 0")
	}

	if loaded.EndError == nil {
		t.Error("EndTimerWithError() should set EndError")
	}

	if loaded.EndError.Error() != errTest.Error() {
		t.Errorf("EndError.Error() = %q, want %q", loaded.EndError.Error(), errTest.Error())
	}
}

func TestEndTimerWithErrorNil(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()
	tas.EndTimerWithError(nil)

	loaded := tas.Load()
	if !loaded.IsFinished() {
		t.Error("EndTimerWithError(nil) should still set EndTime")
	}

	if loaded.EndError != nil {
		t.Error("EndTimerWithError(nil) should set EndError to nil")
	}
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

	if !firstEnd.Equal(secondEnd) {
		t.Error("EndTimerWithError() should be idempotent, EndTime changed on second call")
	}

	if firstErr != secondErr {
		t.Errorf("EndError changed from %q to %q on second call", firstErr, secondErr)
	}
}

func TestDurationOrElapsedTime(t *testing.T) {
	t.Parallel()

	tas := New()

	_, err := tas.DurationOrElapsedTime()
	if err == nil {
		t.Error("DurationOrElapsedTime() on unstarted timer should return error")
	}

	tas.StartTimer()

	dur, err := tas.DurationOrElapsedTime()
	if err != nil {
		t.Errorf("DurationOrElapsedTime() after StartTimer returned error: %v", err)
	}

	if dur <= 0 {
		t.Error("DurationOrElapsedTime() after StartTimer should be > 0")
	}
}

func TestDurationOrElapsedTimeAfterEnd(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()
	tas.EndTimerWithError(nil)

	dur, err := tas.DurationOrElapsedTime()
	if err != nil {
		t.Errorf("DurationOrElapsedTime() after EndTimer returned error: %v", err)
	}

	loaded := tas.Load()
	if dur != loaded.DurationCache {
		t.Errorf("DurationOrElapsedTime() = %v, want DurationCache %v", dur, loaded.DurationCache)
	}
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
	if err != nil {
		t.Errorf("DurationOrElapsedTime() returned error: %v", err)
	}

	if dur != 3*time.Second {
		t.Errorf("DurationOrElapsedTime() = %v, want 3s (cached, not live)", dur)
	}
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()

	data, err := json.Marshal(tas)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("unmarshal result error: %v", err)
	}

	if _, ok := result["start_time"]; !ok {
		t.Error("MarshalJSON() missing start_time field")
	}
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
	if err != nil {
		t.Fatalf("marshal test data error: %v", err)
	}

	tas := New()

	err = json.Unmarshal(jsonData, tas)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	loaded := tas.Load()
	if !loaded.HasStarted() {
		t.Error("UnmarshalJSON() should set StartTime")
	}

	if !loaded.IsFinished() {
		t.Error("UnmarshalJSON() should set EndTime")
	}
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
	if err != nil {
		t.Fatalf("marshal test data error: %v", err)
	}

	tas := New()

	err = json.Unmarshal(jsonData, tas)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	loaded := tas.Load()
	if loaded.EndError == nil {
		t.Error("UnmarshalJSON() should set EndError")
	}

	if loaded.EndError.Error() != "something went wrong" {
		t.Errorf("EndError.Error() = %q, want %q", loaded.EndError.Error(), "something went wrong")
	}
}

func TestUnmarshalJSONNilReceiver(t *testing.T) {
	t.Parallel()

	var tas *AtomicTimeAndState

	err := json.Unmarshal([]byte(`{"start_time":"2024-01-01T00:00:00Z"}`), tas)
	if err == nil {
		t.Error("UnmarshalJSON on nil receiver should return error")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	tas := New()
	tas.StartTimer()
	tas.EndTimerWithError(errRoundTrip)

	data, err := json.Marshal(tas)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	tas2 := New()

	err = json.Unmarshal(data, tas2)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	original := tas.Load()
	restored := tas2.Load()

	if original.DurationCache != restored.DurationCache {
		t.Errorf("DurationCache mismatch: original=%v, restored=%v", original.DurationCache, restored.DurationCache)
	}

	if original.EndError.Error() != restored.EndError.Error() {
		t.Errorf("EndError mismatch: original=%q, restored=%q", original.EndError.Error(), restored.EndError.Error())
	}
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
	if jsonErr == nil {
		t.Fatal("jsonerror.New(non-nil) returned nil")
	}

	if jsonErr.Error() != errJSONErrorTest.Error() {
		t.Errorf("Error() = %q, want %q", jsonErr.Error(), errJSONErrorTest.Error())
	}
}

func TestJSONErrorNewNil(t *testing.T) {
	t.Parallel()

	je := jsonerror.New(nil)
	if je != nil {
		t.Error("jsonerror.New(nil) should return nil")
	}
}
