package handler

import (
	"fullerite/metric"

	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
)

// SignalFx Handler
type SignalFx struct {
	BaseHandler
	endpoint  string
	authToken string
}

// NewSignalFx returns a new SignalFx handler.
func NewSignalFx() *SignalFx {
	s := new(SignalFx)
	s.name = "SignalFx"
	s.maxBufferSize = DefaultBufferSize
	s.channel = make(chan metric.Metric)
	return s
}

// Configure accepts the different configuration options for the signalfx handler
func (s *SignalFx) Configure(config *map[string]string) {
	asmap := *config
	var exists bool
	s.authToken, exists = asmap["authToken"]
	if !exists {
		log.Error("There was no auth key specified for the SignalFx Handler, there won't be any emissions")
	}
	s.endpoint, exists = asmap["endpoint"]
	if !exists {
		log.Error("There was no endpoint specified for the SignalFx Handler, there won't be any emissions")
	}
}

// Run send metrics in the channel to SignalFx.
func (s *SignalFx) Run() {
	datapoints := make([]*DataPoint, 0, s.maxBufferSize)

	lastEmission := time.Now()
	for incomingMetric := range s.Channel() {
		datapoint := s.convertToProto(&incomingMetric)
		log.Debug("SignalFx datapoint: ", datapoint)
		datapoints = append(datapoints, datapoint)
		if time.Since(lastEmission).Seconds() >= float64(s.interval) || len(datapoints) >= s.maxBufferSize {
			s.emitMetrics(datapoints)
			lastEmission = time.Now()
			datapoints = make([]*DataPoint, 0, s.maxBufferSize)
		}
	}
}

func (s *SignalFx) convertToProto(incomingMetric *metric.Metric) *DataPoint {
	outname := s.Prefix() + (*incomingMetric).Name

	datapoint := new(DataPoint)
	datapoint.Metric = &outname
	datapoint.Value = &Datum{
		DoubleValue: &(*incomingMetric).Value,
	}
	datapoint.Source = new(string)
	*datapoint.Source = "fullerite"

	switch incomingMetric.MetricType {
	case metric.Gauge:
		datapoint.MetricType = MetricType_GAUGE.Enum()
	case metric.Counter:
		datapoint.MetricType = MetricType_COUNTER.Enum()
	case metric.CumulativeCounter:
		datapoint.MetricType = MetricType_CUMULATIVE_COUNTER.Enum()
	}

	dimensions := incomingMetric.GetDimensions(s.DefaultDimensions())
	for key, value := range dimensions {
		dim := Dimension{
			Key:   &key,
			Value: &value,
		}
		datapoint.Dimensions = append(datapoint.Dimensions, &dim)
	}

	return datapoint
}

func (s *SignalFx) emitMetrics(datapoints []*DataPoint) {
	log.Info("Starting to emit ", len(datapoints), " datapoints")

	if len(datapoints) == 0 {
		log.Warn("Skipping send because of an empty payload")
		return
	}

	payload := new(DataPointUploadMessage)
	payload.Datapoints = datapoints

	if s.authToken == "" || s.endpoint == "" {
		log.Warn("Skipping emission because we're missing the auth token ",
			"or the endpoint, payload would have been ", payload)
		return
	}
	serialized, err := proto.Marshal(payload)
	if err != nil {
		log.Error("Failed to serailize payload ", payload)
		return
	}

	req, err := http.NewRequest("POST", s.endpoint, bytes.NewBuffer(serialized))
	if err != nil {
		log.Error("Failed to create a request to endpoint ", s.endpoint)
		return
	}
	req.Header.Set("X-SF-TOKEN", s.authToken)
	req.Header.Set("Content-Type", "application/x-protobuf")

	client := &http.Client{}
	rsp, err := client.Do(req)
	if err != nil {
		log.Error("Failed to complete POST ", err)
		return
	}

	defer rsp.Body.Close()
	if rsp.Status != "200 OK" {
		body, _ := ioutil.ReadAll(rsp.Body)
		log.Error("Failed to post to signalfx @", s.endpoint,
			" status was ", rsp.Status,
			" rsp body was ", string(body),
			" payload was ", payload)
		return
	}

	log.Info("Successfully sent ", len(datapoints), " datapoints to SignalFx")
}
