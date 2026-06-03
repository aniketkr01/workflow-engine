package engine

import (
	"errors"
	"fmt"

	"github.com/aniketkr01/workflow-engine/internal/domain"
)

var ErrCyclicDependency = errors.New("workflow contains a cyclic dependency")

// DAG represents the directed acyclic graph of task definitions.
type DAG struct {
	nodes map[string]*domain.TaskDef
	edges map[string][]string // nodeID -> list of successor nodeIDs
	inDeg map[string]int      // in-degree of each node
}

// BuildDAG validates and constructs the execution graph.
func BuildDAG(tasks []domain.TaskDef) (*DAG, error) {
	dag := &DAG{
		nodes: make(map[string]*domain.TaskDef, len(tasks)),
		edges: make(map[string][]string, len(tasks)),
		inDeg: make(map[string]int, len(tasks)),
	}

	for i := range tasks {
		t := &tasks[i]
		if _, exists := dag.nodes[t.ID]; exists {
			return nil, fmt.Errorf("duplicate task id: %s", t.ID)
		}
		dag.nodes[t.ID] = t
		dag.inDeg[t.ID] = 0
	}

	for i := range tasks {
		t := &tasks[i]
		for _, dep := range t.Dependencies {
			if _, exists := dag.nodes[dep]; !exists {
				return nil, fmt.Errorf("task %s depends on unknown task %s", t.ID, dep)
			}
			dag.edges[dep] = append(dag.edges[dep], t.ID)
			dag.inDeg[t.ID]++
		}
	}

	if err := dag.validateAcyclic(); err != nil {
		return nil, err
	}
	return dag, nil
}

// validateAcyclic performs Kahn's topological sort to detect cycles.
func (d *DAG) validateAcyclic() error {
	inDeg := make(map[string]int, len(d.inDeg))
	for k, v := range d.inDeg {
		inDeg[k] = v
	}

	queue := make([]string, 0)
	for id, deg := range inDeg {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	processed := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		processed++
		for _, succ := range d.edges[node] {
			inDeg[succ]--
			if inDeg[succ] == 0 {
				queue = append(queue, succ)
			}
		}
	}

	if processed != len(d.nodes) {
		return ErrCyclicDependency
	}
	return nil
}

// ReadyTasks returns task IDs that have all dependencies completed.
func (d *DAG) ReadyTasks(completed map[string]bool) []string {
	var ready []string
	for id, task := range d.nodes {
		if completed[id] {
			continue
		}
		allDone := true
		for _, dep := range task.Dependencies {
			if !completed[dep] {
				allDone = false
				break
			}
		}
		if allDone {
			ready = append(ready, id)
		}
	}
	return ready
}

// GetTask returns a task definition by ID.
func (d *DAG) GetTask(id string) (*domain.TaskDef, bool) {
	t, ok := d.nodes[id]
	return t, ok
}

// TopologicalOrder returns tasks in execution order.
func (d *DAG) TopologicalOrder() []string {
	inDeg := make(map[string]int, len(d.inDeg))
	for k, v := range d.inDeg {
		inDeg[k] = v
	}

	queue := make([]string, 0)
	for id, deg := range inDeg {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		for _, succ := range d.edges[node] {
			inDeg[succ]--
			if inDeg[succ] == 0 {
				queue = append(queue, succ)
			}
		}
	}
	return order
}
