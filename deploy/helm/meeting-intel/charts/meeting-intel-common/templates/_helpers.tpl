{{/*
Chart name — every service subchart is named after its own service
(api-gateway, auth-service, ...), so this is just that name, trimmed to
the 63-char DNS label limit Kubernetes object names share.
*/}}
{{- define "meeting-intel-common.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified name used for every object this chart creates. Kept equal
to the chart name (not prefixed with the Helm release name) since each
service is deployed exactly once per cluster — there's no multi-instance
case here the way a generic library chart usually has to guard against.
*/}}
{{- define "meeting-intel-common.fullname" -}}
{{- include "meeting-intel-common.name" . -}}
{{- end -}}

{{/*
Standard app.kubernetes.io/* labels, applied to every object's metadata —
this is what gives every service consistent Prometheus scrape discovery
and Grafana dashboard grouping for free (see
docs/architecture/kubernetes-cicd.md §2).
*/}}
{{- define "meeting-intel-common.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{ include "meeting-intel-common.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | default .Chart.Version | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: meeting-intel
{{- end -}}

{{/*
Labels used both on an object's own metadata.labels and on a
Deployment/Job's spec.selector.matchLabels + pod template labels — these
three must always agree, so every template includes this same helper in
all three places rather than hand-writing matching label blocks.
*/}}
{{- define "meeting-intel-common.selectorLabels" -}}
app.kubernetes.io/name: {{ include "meeting-intel-common.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
ServiceAccount name every service's Deployment/Job runs as.
*/}}
{{- define "meeting-intel-common.serviceAccountName" -}}
{{- include "meeting-intel-common.fullname" . -}}
{{- end -}}
