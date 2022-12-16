```yaml
apiVersion: core.oam.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: observability
  namespace: vela-system
spec:
  context:
    readConfig: true
  mode: 
  workflowSpec:
    steps:
      - name: Enable Prism
        type: addon-operation
        properties:
          addonName: vela-prism
      
      - name: Enable o11y
        type: addon-operation
        properties:
          addonName: o11y-definitions
          operation: enable
          args:
          - --override-definitions

      - name: Prepare Prometheus
        type: step-group
        subSteps: 
        - name: get-exist-prometheus
          type: list-config
          properties:
            template: prometheus-server
          outputs:
          - name: prometheus
            valueFrom: "output.configs"

        - name: prometheus-server
          inputs:
          - from: prometheus
            # TODO: Make it is not required
            parameterKey: configs
          if: "!context.readConfig || len(inputs.prometheus) == 0"
          type: addon-operation
          properties:
            addonName: prometheus-server
            operation: enable
            args:
            - memory=4096Mi
            - serviceType=LoadBalancer

      - name: Prepare Loki
        type: addon-operation
        properties:
          addonName: loki
          operation: enable
          args:
            - --version=v0.1.4
            - agent=vector
            - serviceType=LoadBalancer
            
      - name: Prepare Grafana
        type: step-group
        subSteps: 
        
        - name: get-exist-grafana
          type: list-config
          properties:
            template: grafana
          outputs:
          - name: grafana
            valueFrom: "output.configs"
        
        - name: Install Grafana & Init Dashboards
          inputs:
          - from: grafana
            parameterKey: configs
          if: "!context.readConfig || len(inputs.grafana) == 0"
          type: addon-operation
          properties:
            addonName: grafana
            operation: enable
            args:
              - serviceType=LoadBalancer
        
        - name: Init Dashboards
          inputs:
          - from: grafana
            parameterKey: configs
          if: "len(inputs.grafana) != 0"
          type: addon-operation
          properties:
            addonName: grafana
            operation: enable
            args:
              - install=false

      - name: Clean
        type: clean-jobs
  
      - name: print-message
        type: print-message-in-status
        properties:
          message: "All addons have been enabled successfully, you can use 'vela addon list' to check them."
```