```yaml
apiVersion: core.oam.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: apply-deployment
  namespace: default
spec:
  workflowSpec:
    steps:
    - name: apply
      type: apply-deployment
      properties:
        image: nginx
```