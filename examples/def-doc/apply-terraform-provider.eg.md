```yaml
apiVersion: core.oam.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: apply-terraform-provider
  namespace: default
spec:
  workflowSpec:
    steps:
    - name: provider
      type: apply-terraform-provider
      properties:
        type: alibaba
        name: my-alibaba-provider
        accessKey: <accessKey>
        secretKey: <secretKey>
        region: cn-hangzhou
```