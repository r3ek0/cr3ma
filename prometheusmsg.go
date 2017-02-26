package main

import (
	"encoding/json"
)

type PrometheusAlertAnotations struct {
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

type PrometheusAlert struct {
	Status       string                    `json:"status"`
	Labels       map[string]interface{}    `json:"labels"`
	Annotations  PrometheusAlertAnotations `json:"annotations"`
	StartsAt     string                    `json:"startsAt"`
	EndsAt       string                    `json:"endsAt"`
	GeneratorURL string                    `json:"generatorURL"`
}

type PrometheusMessage struct {
	Receiver          string                    `json:"receiver"`
	Status            string                    `json:"status"`
	Alerts            []PrometheusAlert         `json:"alerts"`
	GroupLabels       map[string]interface{}    `json:"groupLabels"`
	CommonLabels      map[string]interface{}    `json:"commonLabels"`
	CommonAnnotations PrometheusAlertAnotations `json:"commonAnnotations"`
	ExternalURL       string                    `json:"externalURL"`
	Version           string                    `json:"version"`
	GroupKey          json.Number               `json:"groupKey"`
}
