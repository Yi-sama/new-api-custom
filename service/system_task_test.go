package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSystemTaskRegistry swaps the package registry for the given handlers for
// the duration of a test and restores the original registry afterward.
func withSystemTaskRegistry(t *testing.T, handlers ...SystemTaskHandler) {
	t.Helper()
	systemTaskHandlersMu.Lock()
	saved := systemTaskHandlers
	systemTaskHandlers = map[string]SystemTaskHandler{}
	for _, h := range handlers {
		systemTaskHandlers[h.Type()] = h
	}
	systemTaskHandlersMu.Unlock()
	t.Cleanup(func() {
		systemTaskHandlersMu.Lock()
		systemTaskHandlers = saved
		systemTaskHandlersMu.Unlock()
	})
}

type stubScheduledHandler struct {
	taskType string
	enabled  bool
	interval time.Duration
	onRun    func(ctx context.Context, task *model.SystemTask, runnerID string)
}

type stubSystemTaskRunResult struct {
	taskID   string
	taskType string
	err      error
}

func (h *stubScheduledHandler) Type() string { return h.taskType }

func (h *stubScheduledHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	if h.onRun != nil {
		h.onRun(ctx, task, runnerID)
	}
}

func (h *stubScheduledHandler) Enabled() bool           { return h.enabled }
func (h *stubScheduledHandler) Interval() time.Duration { return h.interval }
func (h *stubScheduledHandler) NewPayload() any         { return nil }

func countSystemTasks(t *testing.T, taskType string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.SystemTask{}).Where("type = ?", taskType).Count(&count).Error)
	return count
}

func TestSystemTaskSchedulerCreatesWhenDueAndDedups(t *testing.T) {
	truncate(t)

	handler := &stubScheduledHandler{taskType: "test_scheduled", enabled: true, interval: time.Minute}
	withSystemTaskRegistry(t, handler)

	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// An active (pending) row already exists, so a second pass must not create
	// another row.
	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// Finish the run; with a fresh updated_at the next run is not due yet.
	latest, err := model.GetLatestSystemTask(handler.taskType)
	require.NoError(t, err)
	require.NotNil(t, latest)
	_, claimed, err := model.ClaimSystemTask(latest.ID, handler.taskType, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, model.FinishSystemTask(latest.TaskID, "runner-a", model.SystemTaskStatusSucceeded, nil, ""))

	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// Backdate the finished row beyond the interval -> the job becomes due again.
	require.NoError(t, model.DB.Model(&model.SystemTask{}).
		Where("task_id = ?", latest.TaskID).
		Update("updated_at", common.GetTimestamp()-120).Error)

	runSystemTaskScheduler()
	require.Equal(t, int64(2), countSystemTasks(t, handler.taskType))
}

func TestSystemTaskSchedulerSkipsDisabled(t *testing.T) {
	truncate(t)

	handler := &stubScheduledHandler{taskType: "test_disabled", enabled: false, interval: time.Minute}
	withSystemTaskRegistry(t, handler)

	runSystemTaskScheduler()
	assert.Equal(t, int64(0), countSystemTasks(t, handler.taskType))
}

func TestSystemTaskSchedulerHonorsExplicitTypeScope(t *testing.T) {
	truncate(t)
	t.Setenv("SYSTEM_TASK_TYPES", "\r\nvideo_studio_jobs,\tasync_task_poll\r\n")
	withSystemTaskRegistry(t,
		&stubScheduledHandler{taskType: model.SystemTaskTypeVideoStudio, enabled: true, interval: time.Minute},
		&stubScheduledHandler{taskType: model.SystemTaskTypeAsyncTaskPoll, enabled: true, interval: time.Minute},
		&stubScheduledHandler{taskType: model.SystemTaskTypeChannelTest, enabled: true, interval: time.Minute},
	)

	runSystemTaskScheduler()

	assert.Equal(t, int64(1), countSystemTasks(t, model.SystemTaskTypeVideoStudio))
	assert.Equal(t, int64(1), countSystemTasks(t, model.SystemTaskTypeAsyncTaskPoll))
	assert.Equal(t, int64(0), countSystemTasks(t, model.SystemTaskTypeChannelTest))
}

func TestSystemTaskClaimPassLeavesExcludedPendingTaskUntouched(t *testing.T) {
	truncate(t)
	t.Setenv("SYSTEM_TASK_TYPES", model.SystemTaskTypeVideoStudio)
	withSystemTaskRegistry(t, &stubScheduledHandler{taskType: model.SystemTaskTypeChannelTest})
	task, err := model.CreateSystemTask(model.SystemTaskTypeChannelTest, nil, nil)
	require.NoError(t, err)

	runSystemTaskClaimPass("scoped-runner")

	current, err := model.GetLatestSystemTask(model.SystemTaskTypeChannelTest)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, task.TaskID, current.TaskID)
	assert.Equal(t, model.SystemTaskStatusPending, current.Status)
	assert.Empty(t, current.LockedBy)
}

func TestSystemTaskSchedulerRejectsUnrecognizedScope(t *testing.T) {
	for _, scope := range []string{", ,", "unknown_task"} {
		t.Run(scope, func(t *testing.T) {
			truncate(t)
			t.Setenv("SYSTEM_TASK_TYPES", scope)
			withSystemTaskRegistry(t, &stubScheduledHandler{
				taskType: model.SystemTaskTypeChannelTest,
				enabled:  true,
				interval: time.Minute,
			})

			runSystemTaskScheduler()

			assert.Equal(t, int64(0), countSystemTasks(t, model.SystemTaskTypeChannelTest))
		})
	}
}

func TestSystemTaskStaleLockCleanupHonorsExplicitTypeScope(t *testing.T) {
	for _, tc := range []struct {
		name            string
		scope           string
		expectedVideo   model.SystemTaskStatus
		expectedChannel model.SystemTaskStatus
	}{
		{"default", "", model.SystemTaskStatusFailed, model.SystemTaskStatusFailed},
		{"video_only", model.SystemTaskTypeVideoStudio, model.SystemTaskStatusFailed, model.SystemTaskStatusRunning},
		{"unknown", "unknown_task", model.SystemTaskStatusRunning, model.SystemTaskStatusRunning},
		{"empty_list", ", ,", model.SystemTaskStatusRunning, model.SystemTaskStatusRunning},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			t.Setenv("SYSTEM_TASK_TYPES", tc.scope)
			withSystemTaskRegistry(t,
				&stubScheduledHandler{taskType: model.SystemTaskTypeVideoStudio},
				&stubScheduledHandler{taskType: model.SystemTaskTypeChannelTest},
			)
			now := common.GetTimestamp()
			for _, taskType := range []string{model.SystemTaskTypeVideoStudio, model.SystemTaskTypeChannelTest} {
				task, err := model.CreateSystemTask(taskType, nil, nil)
				require.NoError(t, err)
				_, claimed, err := model.ClaimSystemTask(task.ID, taskType, "expired-runner", now+60)
				require.NoError(t, err)
				require.True(t, claimed)
			}
			require.NoError(t, model.DB.Model(&model.SystemTaskLock{}).
				Where("locked_by = ?", "expired-runner").Update("locked_until", now-1).Error)

			require.NoError(t, runSystemTaskStaleLockCleanup())

			for taskType, expected := range map[string]model.SystemTaskStatus{
				model.SystemTaskTypeVideoStudio: tc.expectedVideo,
				model.SystemTaskTypeChannelTest: tc.expectedChannel,
			} {
				task, err := model.GetLatestSystemTask(taskType)
				require.NoError(t, err)
				require.NotNil(t, task)
				assert.Equal(t, expected, task.Status)
				var lockCount int64
				require.NoError(t, model.DB.Model(&model.SystemTaskLock{}).
					Where("type = ?", taskType).Count(&lockCount).Error)
				if expected == model.SystemTaskStatusRunning {
					assert.Equal(t, int64(1), lockCount)
				} else {
					assert.Zero(t, lockCount)
				}
			}
		})
	}
}

func TestSystemTaskClaimPassDispatchesByType(t *testing.T) {
	truncate(t)

	ran := make(chan stubSystemTaskRunResult, 1)
	handler := &stubScheduledHandler{
		taskType: "test_dispatch",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskType: task.Type,
				err:      model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	withSystemTaskRegistry(t, handler)

	_, err := model.CreateSystemTask(handler.taskType, nil, nil)
	require.NoError(t, err)

	runSystemTaskClaimPass("runner-dispatch")

	select {
	case got := <-ran:
		require.NoError(t, got.err)
		assert.Equal(t, handler.taskType, got.taskType)
	case <-time.After(2 * time.Second):
		t.Fatal("claimed task was not dispatched to its handler")
	}

	require.Eventually(t, func() bool {
		latest, err := model.GetLatestSystemTask(handler.taskType)
		return err == nil && latest != nil && latest.Status == model.SystemTaskStatusSucceeded
	}, 2*time.Second, 20*time.Millisecond)
}

func TestSystemTaskClaimPassDispatchesEarliestPendingByType(t *testing.T) {
	truncate(t)

	ran := make(chan stubSystemTaskRunResult, 2)
	handlerA := &stubScheduledHandler{
		taskType: "test_dispatch_a",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskID: task.TaskID,
				err:    model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	handlerB := &stubScheduledHandler{
		taskType: "test_dispatch_b",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskID: task.TaskID,
				err:    model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	withSystemTaskRegistry(t, handlerA, handlerB)

	firstA, err := model.CreateSystemTask(handlerA.taskType, nil, nil)
	require.NoError(t, err)
	secondTaskID, err := model.GenerateSystemTaskID()
	require.NoError(t, err)
	secondA := &model.SystemTask{
		TaskID: secondTaskID,
		Type:   handlerA.taskType,
		Status: model.SystemTaskStatusPending,
	}
	require.NoError(t, model.DB.Create(secondA).Error)
	firstB, err := model.CreateSystemTask(handlerB.taskType, nil, nil)
	require.NoError(t, err)

	runSystemTaskClaimPass("runner-dispatch")

	got := map[string]bool{}
	for range 2 {
		select {
		case result := <-ran:
			require.NoError(t, result.err)
			got[result.taskID] = true
		case <-time.After(2 * time.Second):
			t.Fatal("claimed tasks were not dispatched to their handlers")
		}
	}

	assert.True(t, got[firstA.TaskID])
	assert.True(t, got[firstB.TaskID])
	assert.False(t, got[secondA.TaskID])

	require.Eventually(t, func() bool {
		reloaded, err := model.GetSystemTaskByTaskID(secondA.TaskID)
		return err == nil && reloaded != nil && reloaded.Status == model.SystemTaskStatusPending
	}, 2*time.Second, 20*time.Millisecond)
}

func TestEnqueueSystemTaskReportsCreatedAndExistingActive(t *testing.T) {
	truncate(t)

	first, created, err := EnqueueSystemTask("test_enqueue", map[string]bool{"manual": true})
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, first)

	existing, created, err := EnqueueSystemTask("test_enqueue", nil)
	require.NoError(t, err)
	require.False(t, created)
	require.NotNil(t, existing)
	assert.Equal(t, first.TaskID, existing.TaskID)

	_, claimed, err := model.ClaimSystemTask(first.ID, first.Type, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, model.FinishSystemTask(first.TaskID, "runner-a", model.SystemTaskStatusSucceeded, nil, ""))

	second, created, err := EnqueueSystemTask("test_enqueue", nil)
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, second)
	assert.NotEqual(t, first.TaskID, second.TaskID)
}
