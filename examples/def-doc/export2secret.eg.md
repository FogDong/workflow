```yaml
apiVersion: core.oam.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: export-secret
  namespace: default
spec:
  workflowSpec:
    steps:
      - name: export-secret
        type: export2secret
        properties:
          secretName: my-secret
          data:
            testkey: |
              testvalue
              value-line-2
```