
```yaml
apiVersion: core.oam.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: suspend-example
  namespace: default
spec:
  workflowSpec:
    steps:
      - name: slack-message
        type: notification
        properties:
          slack:
            url:
              value: <your-slack-url>
            # the Slack webhook address, please refer to: https://api.slack.com/messaging/webhooks
            message:
              text: Ready to apply the application, ask the administrator to approve and resume the workflow.
      - name: manual-approval
        type: suspend
        # properties:
        #   duration: "30s"
      - name: nginx-server
        type: apply-deployment
        properties:
          image: nginx
```