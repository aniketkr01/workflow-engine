package engine

import (
	"testing"

	"github.com/aniketkr01/workflow-engine/internal/domain"
)

func TestBuildDAG(t *testing.T) {
	tests := []struct {
		name    string
		tasks   []domain.TaskDef
		wantErr string
	}{
		{
			name: "valid DAG",
			tasks: []domain.TaskDef{
				{ID: "a"},
				{ID: "b", Dependencies: []string{"a"}},
				{ID: "c", Dependencies: []string{"b"}},
			},
		},
		{
			name: "duplicate task id",
			tasks: []domain.TaskDef{
				{ID: "a"},
				{ID: "a"},
			},
			wantErr: "duplicate task id: a",
		},
		{
			name: "unknown dependency",
			tasks: []domain.TaskDef{
				{ID: "a", Dependencies: []string{"missing"}},
			},
			wantErr: "task a depends on unknown task missing",
		},
		{
			name: "cyclic dependency",
			tasks: []domain.TaskDef{
				{ID: "a", Dependencies: []string{"b"}},
				{ID: "b", Dependencies: []string{"a"}},
			},
			wantErr: ErrCyclicDependency.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildDAG(tt.tasks)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestReadyTasks(t *testing.T) {
	tests := []struct {
		name      string
		tasks     []domain.TaskDef
		completed map[string]bool
		want      []string
	}{
		{
			name: "ready tasks with no dependencies",
			tasks: []domain.TaskDef{
				{ID: "a"},
				{ID: "b"},
			},
			completed: map[string]bool{},
			want:      []string{"a", "b"},
		},
		{
			name: "single dependency completed",
			tasks: []domain.TaskDef{
				{ID: "a"},
				{ID: "b", Dependencies: []string{"a"}},
				{ID: "c", Dependencies: []string{"b"}},
			},
			completed: map[string]bool{"a": true},
			want:      []string{"b"},
		},
		{
			name: "diamond dependency",
			tasks: []domain.TaskDef{
				{ID: "a"},
				{ID: "b", Dependencies: []string{"a"}},
				{ID: "c", Dependencies: []string{"a"}},
				{ID: "d", Dependencies: []string{"b", "c"}},
			},
			completed: map[string]bool{"a": true},
			want:      []string{"b", "c"},
		},
		{
			name: "completed tasks excluded",
			tasks: []domain.TaskDef{
				{ID: "a"},
				{ID: "b", Dependencies: []string{"a"}},
			},
			completed: map[string]bool{"a": true, "b": true},
			want:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag, err := BuildDAG(tt.tasks)
			if err != nil {
				t.Fatalf("BuildDAG failed: %v", err)
			}
			got := dag.ReadyTasks(tt.completed)
			if !sameElements(got, tt.want) {
				t.Fatalf("ReadyTasks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTopologicalOrder(t *testing.T) {
	tasks := []domain.TaskDef{
		{ID: "a"},
		{ID: "b", Dependencies: []string{"a"}},
		{ID: "c", Dependencies: []string{"a"}},
		{ID: "d", Dependencies: []string{"b", "c"}},
	}

	dag, err := BuildDAG(tasks)
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	order := dag.TopologicalOrder()
	if len(order) != len(tasks) {
		t.Fatalf("expected order length %d, got %d", len(tasks), len(order))
	}

	index := map[string]int{}
	for i, id := range order {
		index[id] = i
	}

	for _, task := range tasks {
		for _, dep := range task.Dependencies {
			if index[dep] >= index[task.ID] {
				t.Fatalf("task %q appears before dependency %q in order %v", task.ID, dep, order)
			}
		}
	}
}

func sameElements(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	counts := make(map[string]int)
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
		if counts[v] < 0 {
			return false
		}
	}
	return true
}
