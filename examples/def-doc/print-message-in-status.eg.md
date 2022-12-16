```yaml
apiVersion: core.oam.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: print-message-in-status
  namespace: default
spec:
  workflowSpec:
    steps:
      - name: message
        type: print-message-in-status
        properties:
          message: "hello message"
```