```yaml
apiVersion: core.oam.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: export2config
  namespace: default
spec:
  workflowSpec:
    steps:
      - name: export-config
        type: export2config
        properties:
          configName: my-configmap
          data:
            testkey: |
              testvalue
              value-line-2
```