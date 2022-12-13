/*
Copyright 2022 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"fmt"
	"io"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevela/workflow/api/v1alpha1"
	wfContext "github.com/kubevela/workflow/pkg/context"
	"github.com/kubevela/workflow/pkg/cue/model/sets"
	"github.com/kubevela/workflow/pkg/cue/model/value"
	wfTypes "github.com/kubevela/workflow/pkg/types"
)

// WorkflowOperator is operation handler for workflow's suspend/resume/rollback/restart/terminate
type WorkflowOperator interface {
	Suspend(ctx context.Context) error
	Resume(ctx context.Context) error
	Rollback(ctx context.Context) error
	Restart(ctx context.Context, step string) error
	Terminate(ctx context.Context) error
}

type workflowRunOperator struct {
	cli          client.Client
	outputWriter io.Writer
	run          *v1alpha1.WorkflowRun
}

// NewWorkflowRunOperator get an workflow operator with k8sClient, ioWriter(optional, useful for cli) and application
func NewWorkflowRunOperator(cli client.Client, w io.Writer, run *v1alpha1.WorkflowRun) WorkflowOperator {
	return workflowRunOperator{
		cli:          cli,
		outputWriter: w,
		run:          run,
	}
}

// Suspend suspend workflow
func (wo workflowRunOperator) Suspend(ctx context.Context) error {
	run := wo.run
	runKey := client.ObjectKeyFromObject(run)
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := wo.cli.Get(ctx, runKey, run); err != nil {
			return err
		}
		// set the workflow suspend to true
		run.Status.Suspend = true
		return wo.cli.Status().Patch(ctx, run, client.Merge)
	}); err != nil {
		return err
	}

	return wo.writeOutputF("Successfully suspend workflow: %s\n", run.Name)
}

// Resume resume a suspended workflow
func (wo workflowRunOperator) Resume(ctx context.Context) error {
	run := wo.run
	if run.Status.Terminated {
		return fmt.Errorf("can not resume a terminated workflow")
	}

	if run.Status.Suspend {
		if err := ResumeWorkflow(ctx, wo.cli, run); err != nil {
			return err
		}
	}
	return wo.writeOutputF("Successfully resume workflow: %s\n", run.Name)
}

// ResumeWorkflow resume workflow
func ResumeWorkflow(ctx context.Context, cli client.Client, run *v1alpha1.WorkflowRun) error {
	run.Status.Suspend = false
	steps := run.Status.Steps
	for i, step := range steps {
		if step.Type == wfTypes.WorkflowStepTypeSuspend && step.Phase == v1alpha1.WorkflowStepPhaseRunning {
			steps[i].Phase = v1alpha1.WorkflowStepPhaseSucceeded
		}
		for j, sub := range step.SubStepsStatus {
			if sub.Type == wfTypes.WorkflowStepTypeSuspend && sub.Phase == v1alpha1.WorkflowStepPhaseRunning {
				steps[i].SubStepsStatus[j].Phase = v1alpha1.WorkflowStepPhaseSucceeded
			}
		}
	}
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return cli.Status().Patch(ctx, run, client.Merge)
	}); err != nil {
		return err
	}
	return nil
}

// Rollback is not supported for WorkflowRun
func (wo workflowRunOperator) Rollback(ctx context.Context) error {
	return fmt.Errorf("can not rollback a WorkflowRun")
}

// Restart restart workflow
func (wo workflowRunOperator) Restart(ctx context.Context, step string) error {
	run := wo.run
	if err := RestartWorkflow(ctx, wo.cli, run, step); err != nil {
		return err
	}
	return wo.writeOutputF("Successfully restart workflow: %s\n", run.Name)
}

// RestartWorkflow restart workflow
func RestartWorkflow(ctx context.Context, cli client.Client, run *v1alpha1.WorkflowRun, step string) error {
	if step != "" {
		return RestartFromStep(ctx, cli, run, step)
	}
	if run.Status.ContextBackend != nil {
		cm := &corev1.ConfigMap{}
		if err := cli.Get(ctx, client.ObjectKey{Namespace: run.Namespace, Name: run.Status.ContextBackend.Name}, cm); err == nil {
			if err := cli.Delete(ctx, cm); err != nil {
				return err
			}
		} else if !kerrors.IsNotFound(err) {
			return err
		}
	}
	// reset the workflow status to restart the workflow
	run.Status = v1alpha1.WorkflowRunStatus{}

	if err := cli.Status().Update(ctx, run); err != nil {
		return err
	}

	return nil
}

// Terminate terminate workflow
func (wo workflowRunOperator) Terminate(ctx context.Context) error {
	run := wo.run
	if err := TerminateWorkflow(ctx, wo.cli, run); err != nil {
		return err
	}
	return wo.writeOutputF("Successfully terminate workflow: %s\n", run.Name)
}

// TerminateWorkflow terminate workflow
func TerminateWorkflow(ctx context.Context, cli client.Client, run *v1alpha1.WorkflowRun) error {
	// set the workflow terminated to true
	run.Status.Terminated = true
	// set the workflow suspend to false
	run.Status.Suspend = false
	steps := run.Status.Steps
	for i, step := range steps {
		switch step.Phase {
		case v1alpha1.WorkflowStepPhaseFailed:
			if step.Reason != wfTypes.StatusReasonFailedAfterRetries && step.Reason != wfTypes.StatusReasonTimeout {
				steps[i].Reason = wfTypes.StatusReasonTerminate
			}
		case v1alpha1.WorkflowStepPhaseRunning:
			steps[i].Phase = v1alpha1.WorkflowStepPhaseFailed
			steps[i].Reason = wfTypes.StatusReasonTerminate
		default:
		}
		for j, sub := range step.SubStepsStatus {
			switch sub.Phase {
			case v1alpha1.WorkflowStepPhaseFailed:
				if sub.Reason != wfTypes.StatusReasonFailedAfterRetries && sub.Reason != wfTypes.StatusReasonTimeout {
					steps[i].SubStepsStatus[j].Reason = wfTypes.StatusReasonTerminate
				}
			case v1alpha1.WorkflowStepPhaseRunning:
				steps[i].SubStepsStatus[j].Phase = v1alpha1.WorkflowStepPhaseFailed
				steps[i].SubStepsStatus[j].Reason = wfTypes.StatusReasonTerminate
			default:
			}
		}
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return cli.Status().Patch(ctx, run, client.Merge)
	}); err != nil {
		return err
	}
	return nil
}

// RestartFromStep restart workflow from a failed step
func RestartFromStep(ctx context.Context, cli client.Client, run *v1alpha1.WorkflowRun, stepName string) error {
	if stepName == "" {
		return fmt.Errorf("step name can not be empty")
	}
	run.Status.Terminated = false
	run.Status.Suspend = false
	run.Status.Finished = false
	if !run.Status.EndTime.IsZero() {
		run.Status.EndTime = metav1.Time{}
	}
	stepStatus := run.Status.Steps
	mode := run.Status.Mode
	found := false

	var steps []v1alpha1.WorkflowStep
	if run.Spec.WorkflowSpec != nil {
		steps = run.Spec.WorkflowSpec.Steps
	} else {
		workflow := &v1alpha1.Workflow{}
		if err := cli.Get(ctx, client.ObjectKey{Namespace: run.Namespace, Name: run.Spec.WorkflowRef}, workflow); err != nil {
			return err
		}
		steps = workflow.Steps
	}

	dependency := make([]string, 0)
	for i, step := range stepStatus {
		if step.Name == stepName {
			if step.Phase != v1alpha1.WorkflowStepPhaseFailed {
				return fmt.Errorf("can not restart from a non-failed step")
			}
			dependency = getStepDependency(ctx, cli, steps, stepName, mode.Steps == v1alpha1.WorkflowModeDAG)
			run.Status.Steps = deleteStepStatus(dependency, stepStatus, stepName, false)
			found = true
			break
		}
		for _, sub := range step.SubStepsStatus {
			if sub.Name == stepName {
				if sub.Phase != v1alpha1.WorkflowStepPhaseFailed {
					return fmt.Errorf("can not restart from a non-failed step")
				}
				subDependency := getStepDependency(ctx, cli, steps, stepName, mode.SubSteps == v1alpha1.WorkflowModeDAG)
				run.Status.Steps[i].SubStepsStatus = deleteSubStepStatus(subDependency, step.SubStepsStatus, stepName)
				run.Status.Steps[i].Phase = v1alpha1.WorkflowStepPhaseRunning
				run.Status.Steps[i].Reason = ""
				stepDependency := getStepDependency(ctx, cli, steps, step.Name, mode.Steps == v1alpha1.WorkflowModeDAG)
				run.Status.Steps = deleteStepStatus(stepDependency, stepStatus, stepName, true)
				dependency = mergeUniqueStringSlice(subDependency, stepDependency)
				found = true
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("failed step %s not found", stepName)
	}
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return cli.Status().Update(ctx, run)
	}); err != nil {
		return err
	}

	if run.Status.ContextBackend != nil {
		cm := &corev1.ConfigMap{}
		if err := cli.Get(ctx, client.ObjectKey{Namespace: run.Namespace, Name: run.Status.ContextBackend.Name}, cm); err != nil {
			return err
		}
		v, err := value.NewValue(cm.Data[wfContext.ConfigMapKeyVars], nil, "")
		if err != nil {
			return err
		}
		s, err := clearContextVars(steps, v, stepName, dependency)
		if err != nil {
			return err
		}
		cm.Data[wfContext.ConfigMapKeyVars] = s
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return cli.Update(ctx, cm)
		}); err != nil {
			return err
		}
	}
	return nil
}

// nolint:staticcheck
func clearContextVars(steps []v1alpha1.WorkflowStep, v *value.Value, stepName string, dependency []string) (string, error) {
	outputs := make([]string, 0)
	for _, step := range steps {
		if step.Name == stepName || stringsContain(dependency, step.Name) {
			for _, output := range step.Outputs {
				outputs = append(outputs, output.Name)
			}
		}
		for _, sub := range step.SubSteps {
			if sub.Name == stepName || stringsContain(dependency, sub.Name) {
				for _, output := range sub.Outputs {
					outputs = append(outputs, output.Name)
				}
			}
		}
	}
	node := v.CueValue().Syntax(cue.ResolveReferences(true))
	x, ok := node.(*ast.StructLit)
	if !ok {
		return "", fmt.Errorf("value is not a struct lit")
	}
	element := make([]ast.Decl, 0)
	for i := range x.Elts {
		if field, ok := x.Elts[i].(*ast.Field); ok {
			label := strings.Trim(sets.LabelStr(field.Label), `"`)
			if !stringsContain(outputs, label) {
				element = append(element, field)
			}
		}
	}
	x.Elts = element
	b, err := format.Node(x)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func deleteStepStatus(dependency []string, steps []v1alpha1.WorkflowStepStatus, stepName string, group bool) []v1alpha1.WorkflowStepStatus {
	status := make([]v1alpha1.WorkflowStepStatus, 0)
	for _, step := range steps {
		if group && !stringsContain(dependency, step.Name) {
			status = append(status, step)
			continue
		}
		if !group && !stringsContain(dependency, step.Name) && step.Name != stepName {
			status = append(status, step)
		}
	}
	return status
}

func deleteSubStepStatus(dependency []string, subSteps []v1alpha1.StepStatus, stepName string) []v1alpha1.StepStatus {
	status := make([]v1alpha1.StepStatus, 0)
	for _, step := range subSteps {
		if !stringsContain(dependency, step.Name) && step.Name != stepName {
			status = append(status, step)
		}
	}
	return status
}

func stringsContain(items []string, source string) bool {
	for _, item := range items {
		if item == source {
			return true
		}
	}
	return false
}

func getStepDependency(ctx context.Context, cli client.Client, steps []v1alpha1.WorkflowStep, stepName string, dag bool) []string {
	if !dag {
		dependency := make([]string, 0)
		for i, step := range steps {
			if step.Name == stepName {
				for index := i + 1; index < len(steps); index++ {
					dependency = append(dependency, steps[index].Name)
				}
				return dependency
			}
			for j, sub := range step.SubSteps {
				if sub.Name == stepName {
					for index := j + 1; index < len(step.SubSteps); index++ {
						dependency = append(dependency, step.SubSteps[index].Name)
					}
					return dependency
				}
			}
		}
		return dependency
	}
	dependsOn := make(map[string][]string)
	stepOutputs := make(map[string]string)
	for _, step := range steps {
		for _, output := range step.Outputs {
			stepOutputs[output.Name] = step.Name
		}
		dependsOn[step.Name] = step.DependsOn
		for _, sub := range step.SubSteps {
			for _, output := range sub.Outputs {
				stepOutputs[output.Name] = sub.Name
			}
			dependsOn[sub.Name] = sub.DependsOn
		}
	}
	for _, step := range steps {
		for _, input := range step.Inputs {
			if name, ok := stepOutputs[input.From]; ok && !stringsContain(dependsOn[step.Name], name) {
				dependsOn[step.Name] = append(dependsOn[step.Name], name)
			}
		}
		for _, sub := range step.SubSteps {
			for _, input := range sub.Inputs {
				if name, ok := stepOutputs[input.From]; ok && !stringsContain(dependsOn[sub.Name], name) {
					dependsOn[sub.Name] = append(dependsOn[sub.Name], name)
				}
			}
		}
	}
	return findDependency(stepName, dependsOn)
}

func mergeUniqueStringSlice(a, b []string) []string {
	for _, item := range b {
		if !stringsContain(a, item) {
			a = append(a, item)
		}
	}
	return a
}

func findDependency(stepName string, dependsOn map[string][]string) []string {
	dependency := make([]string, 0)
	for step, deps := range dependsOn {
		for _, dep := range deps {
			if dep == stepName {
				dependency = append(dependency, step)
				dependency = append(dependency, findDependency(step, dependsOn)...)
			}
		}
	}
	return dependency
}

func (wo workflowRunOperator) writeOutputF(format string, a ...interface{}) error {
	if wo.outputWriter == nil {
		return nil
	}
	_, err := fmt.Fprintf(wo.outputWriter, format, a...)
	return err
}
