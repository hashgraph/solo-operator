{{- if .Values.install }}
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    {{- with .Values.annotations }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- if .Values.keep }}
    "helm.sh/resource-policy": keep
    {{- end }}
  {{- with .Values.additionalLabels }}
  labels:
    {{- toYaml . | nindent 4 }}
  {{- end }}
