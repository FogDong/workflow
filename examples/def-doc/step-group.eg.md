```yaml
apiVersion: core.oam.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: example-group
  namespace: default
spec:
  workflowSpec:
    steps:
      - name: step
        type: step-group
        subSteps:
          - name: apply-sub-step1
            type: apply-deployment
            properties:
              image: nginx
          - name: apply-sub-step2
            type: apply-deployment
            properties:
              image: nginx
```
