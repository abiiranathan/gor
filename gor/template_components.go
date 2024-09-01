package gor

var components = `
{{- block "input" . }}
  {{- $ID := .id }}
  {{- if not $ID }}
    {{- $ID = .name }}
  {{- end }}
  {{- $disabled := IsTrue .disabled }}
  {{- $readonly := IsTrue .readonly }}
  {{- $required := IsTrue .required }}
  {{- $type := .type }}
  {{- if not $type }}
    {{- $type = "text" }}
  {{- end }}
  {{- $placeholder := .placeholder }}
  {{- $value := .value }}
  {{- $min := .min }}
  {{- $max := .max }}
  {{- $step := .step }}
  {{- $pattern := .pattern }}
  {{- $autocomplete := .autocomplete }}
  {{- $autofocus := IsTrue .autofocus }}
  {{- $class := .class }}
  {{ if not $class }}
  {{ $class = "py-2 px-3 mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50" }}
  {{ end}}

  <div class="mb-4">
    <label for="{{ $ID }}">{{ .label }}</label>
    <input
      type="{{ $type }}"
      id="{{ $ID }}"
      name="{{ .name }}"
      {{- if $disabled }} disabled{{ end }}
      {{- if $readonly }} readonly{{ end }}
      {{- if $required }} required{{ end }}
      {{- if $placeholder }} placeholder="{{ $placeholder }}"{{ end }}
      {{- if $value }} value="{{ $value }}"{{ end }}
      {{- if $min }} min="{{ $min }}"{{ end }}
      {{- if $max }} max="{{ $max }}"{{ end }}
      {{- if $step }} step="{{ $step }}"{{ end }}
      {{- if $pattern }} pattern="{{ $pattern }}"{{ end }}
      {{- if $autocomplete }} autocomplete="{{ $autocomplete }}"{{ end }}
      {{- if $autofocus }} autofocus{{ end }}
      {{- if $class }} class="{{ $class }}"{{ end }}
    >
  </div>
{{ end }}

{{- block "textarea" . }}
{{- $ID := .id }}
{{- if not $ID }}
{{- $ID = .name }}
{{- end }}

{{- $disabled := IsTrue .disabled }}
{{- $readonly := IsTrue .readonly }}
{{- $required := IsTrue .required }}

<div class="mb-4">
    <label for="{{ $ID }}" class="block text-base font-medium text-gray-800 mb-1">{{.label}}</label>
    <textarea id="{{ $ID }}" 
              name="{{ .name }}"
              placeholder="{{ .placeholder }}"
              class="py-2 px-3 mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
              {{ if $required}}required{{ end }} {{ if $readonly}}readonly{{ end }} {{ if $disabled}}disabled{{ end }}>{{- .value -}}</textarea>
</div>
{{ end }}

{{- block "select" . }}
{{- $ID := .id }}
{{- if not $ID }}
{{- $ID = .name }}
{{- end }}

{{- $disabled := IsTrue .disabled }}
{{- $readonly := IsTrue .readonly }}
{{- $required := IsTrue .required }}

<div class="mb-4">
    <label for="{{ $ID }}" class="block text-base font-medium text-gray-800 mb-1">{{.label}}</label>
    <select id="{{ $ID }}" 
            name="{{ .name }}"
			value={{ $.Value }}
            class="py-2 px-3 mt-1 block w-full rounded-md border bg-white border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
            {{ if $required}}required{{ end }} {{ if $readonly}}readonly{{ end }} {{ if $disabled}}disabled{{ end }}>
		{{ if .Placeholder }}
        	<option value="">{{.Placeholder}}</option>
		{{ end }}
        {{ range .options }}
        <option value="{{ . }}" {{ if eq . $.value }}selected{{ end }}>{{.}}</option>
        {{ end }}
    </select>
</div>
{{ end }}

{{- block "checkbox" . }}
{{- $ID := .id }}
{{- if not $ID }}
{{- $ID = .name }}
{{- end }}

{{- $disabled := IsTrue .disabled }}
{{- $readonly := IsTrue .readonly }}
{{- $required := IsTrue .required }}
{{- $checked := IsTrue .checked }}

<div class="mb-4">
    <label for="{{ $ID }}" class="inline-flex items-center gap-2">
        <input type="checkbox" class="py-2 px-3 rounded border border-gray-300 text-indigo-600 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
               id="{{ $ID }}" name="{{ .name }}" value="{{ .value }}"
               {{ if $checked}}checked{{ end }}
			   {{ if $required}}required{{ end }} 
			   {{ if $readonly}}readonly{{ end }} 
			   {{ if $disabled}}disabled{{ end }}  
			   >
        <span class="ml-2 text-base text-gray-800">{{ .label }}</span>
    </label>
</div>
{{ end }}

{{- block "radio" . }}
{{- $ID := .id }}
{{- if not $ID }}
{{- $ID = .name }}
{{- end }}

{{- $disabled := IsTrue .disabled }}
{{- $readonly := IsTrue .readonly }}
{{- $required := IsTrue .required }}
{{- $checked := IsTrue .checked }}

<div class="mb-4">
    <span class="block text-base font-medium text-gray-800 mb-1">{{ .label }}</span>
    <div class="mt-1 space-y-2">
        {{ range .options }}
        <label class="inline-flex items-center">
            <input type="radio" 
			class="border border-gray-300 text-indigo-600 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
					id="{{ $ID }}_{{.name}}" name="{{ .name }}" value="{{ .value }}"
	               {{ if $checked}}checked{{ end }}
				   {{ if $required}}required{{ end }} 
				   {{ if $readonly}}readonly{{ end }} 
				   {{ if $disabled}}disabled{{ end }}   
				   >
            <span class="ml-2 text-base text-gray-800">{{ . }}</span>
        </label>
        {{ end }}
    </div>
</div>
{{ end }}


{{- block "button" . }}
{{- $disabled := IsTrue .disabled }}
{{ $type := .type }}
{{ if not $type }}
    {{ $type = "submit" }}
{{ end }}

{{- $variant := .variant }}
{{- $class := "secondary" }}

{{- if eq $variant "primary" }}
    {{ $class = "whitespace-nowrap inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500" }}
{{- else if eq $variant "secondary" }}
    {{ $class = "whitespace-nowrap inline-flex items-center px-4 py-2 border border-gray-300 text-base font-medium rounded-md shadow-sm text-gray-700 bg-white hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-500" }}
{{- else if eq $variant "success" }}
    {{ $class = "whitespace-nowrap inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-green-600 hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500" }}
{{- else if eq $variant "danger" }}
    {{ $class = "whitespace-nowrap inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500" }}
{{- else if eq $variant "warning" }}
    {{ $class = "whitespace-nowrap inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-yellow-600 hover:bg-yellow-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-yellow-500" }}
{{- else if eq $variant "info" }}
    {{ $class = "whitespace-nowrap inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-sky-600 hover:bg-sky-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-sky-500" }}
{{- else }}
    {{ $class = "whitespace-nowrap inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500" }}
{{ end }}

<button type="{{ $type }}" {{ if .id }} id="{{ .id }}"{{ end }}
        class="{{ $class }} {{ if $disabled }}opacity-50 cursor-not-allowed{{ end }}"
        {{ if $disabled }}disabled{{ end }}>
    {{ .text }}
</button>
{{ end }}



`
