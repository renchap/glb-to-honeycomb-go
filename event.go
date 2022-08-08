package main

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	libhoney "github.com/honeycombio/libhoney-go"
	"github.com/sanity-io/litter"
)

type RequestEvent struct {
	HTTPRequest struct {
		Latency       string `json:"latency"`
		Protocol      string `json:"protocol"`
		RemoteIp      string `json:"remoteIp"`
		RequestMethod string `json:"requestMethod"`
		RequestSize   string `json:"requestSize"`
		RequestURL    string `json:"requestUrl"`
		ResponseSize  string `json:"responseSize"`
		ServerIp      string `json:"serverIp"`
		Status        int    `json:"status"`
		UserAgent     string `json:"userAgent"`
	} `json:"httpRequest"`
	InsertID string `json:"insertId"`
	Labels   struct {
		InstanceID string `json:"instanceId"`
	} `json:"labels"`
	LogName          string    `json:"logName"`
	ReceiveTimestamp time.Time `json:"receiveTimestamp"`
	Resource         struct {
		Labels struct {
			ConfigurationName string `json:"configuration_name"`
			Location          string `json:"location"`
			ProjectID         string `json:"project_id"`
			RevisionName      string `json:"revision_name"`
			ServiceName       string `json:"service_name"`
		} `json:"labels"`
		Type string `json:"type"`
	} `json:"resource"`
	Severity     string    `json:"severity"`
	SpanID       string    `json:"spanId"`
	Timestamp    time.Time `json:"timestamp"`
	Trace        string    `json:"trace"`
	TraceSampled bool      `json:"traceSampled"`
}

func (e RequestEvent) SendToHoneycomb() error {
	log.Println("SendToHoneycomb")
	litter.Dump(e)

	event := libhoney.NewEvent()

	parsedUrl, err := url.Parse(e.HTTPRequest.RequestURL)

	if err != nil {
		log.Printf("url.Parse: %v", err)
		return err
	}

	durationMs, err := strconv.ParseFloat(strings.TrimSuffix(e.HTTPRequest.Latency, "s"), 32)
	if err != nil {
		log.Printf("unable to parse duration field. ParseFloat: %v", err)
		return err
	}

	requestEndDate := e.Timestamp
	requestStartDate := requestEndDate.Add(-time.Duration(durationMs) * time.Second)

	event.Timestamp = requestStartDate

	// base fields
	event.AddField("name", fmt.Sprintf("%s %s", e.HTTPRequest.RequestMethod, parsedUrl.Path))
	event.AddField("service.name", e.Resource.Labels.ServiceName)
	event.AddField("duration_ms", durationMs)
	event.AddField("library.name", "glb-to-honeycomb")
	// ev.addField("library.version", packageVersion)

	// mandatory for OTEL
	spanId, err := strconv.ParseUint(e.SpanID, 10, 64)

	if err != nil {
		log.Printf("Cannot process spanId. ParseUint: %v", err)
		return err
	}

	event.AddField("trace.span_id", fmt.Sprintf("%x", spanId))

	traceId := strings.Split(e.Trace, "/")[3]

	if traceId == "" {
		log.Printf("Can not parse traceId")
		return errors.New("can not parse trace field, not in the expected format")
	}
	event.AddField("trace.trace_id", traceId)

	// http
	event.AddField("http.client_ip", e.HTTPRequest.RemoteIp)
	event.AddField("http.path", parsedUrl.Path)
	event.AddField("http.url", e.HTTPRequest.RequestURL)
	event.AddField("http.status_code", e.HTTPRequest.Status)
	event.AddField("http.user_agent", e.HTTPRequest.UserAgent)

	// cloudrun
	event.AddField("cloudrun.service_name", e.Resource.Labels.ServiceName)
	event.AddField("cloudrun.location", e.Resource.Labels.Location)
	event.AddField("cloudrun.project_id", e.Resource.Labels.ProjectID)
	event.AddField("cloudrun.revision_name", e.Resource.Labels.RevisionName)

	event.Send()

	return nil
}
