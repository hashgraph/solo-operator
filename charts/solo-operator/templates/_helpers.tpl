{{/*
Expand the name of the chart.
*/}}
{{- define "solo-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "solo-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "solo-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "solo-operator.labels" -}}
helm.sh/chart: {{ include "solo-operator.chart" . }}
{{ include "solo-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "solo-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "solo-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "solo-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "solo-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Allowed namespaces, or "_all_namespaces_placeholder" when empty.
Entries are trimmed and blanks dropped to match the operator's SplitCSV.
*/}}
{{- define "solo-operator.allowedNamespaces" -}}
{{- $clean := list }}
{{- range .Values.allowedNamespaces }}
{{- $ns := trim . }}
{{- if $ns }}
{{- $clean = append $clean $ns }}
{{- end }}
{{- end }}
{{- if $clean }}
{{- $online := lookup "v1" "Namespace" "" "kube-system" }}
{{- range $clean }}
{{- if and $online (not (lookup "v1" "Namespace" "" .)) }}
{{- fail (printf "allowedNamespaces: namespace %q not found; create it before adding it to allowedNamespaces" .) }}
{{- end }}
{{- end }}
{{- join " " (uniq $clean) }}
{{- else -}}
_all_namespaces_placeholder
{{- end }}
{{- end }}

{{/*
Controller manager name
*/}}
{{- define "solo-operator.controllerManagerName" -}}
{{- printf "%s-controller-manager" (include "solo-operator.fullname" .) }}
{{- end }}
