{{- if .Values.install }}
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  {{- if or .Values.annotations .Values.keep }}
  annotations:
    {{- with .Values.annotations }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- if .Values.keep }}
    "helm.sh/resource-policy": keep
    {{- end }}
  {{- end }}
  {{- with .Values.additionalLabels }}
  labels:
    {{- toYaml . | nindent 4 }}
  {{- end }}
