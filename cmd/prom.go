package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	pconf "github.com/prometheus/common/config"
	pmod "github.com/prometheus/common/model"
)

const (
	ResultTypeStream = "streams"
	ResultTypeScalar = "scalar"
	ResultTypeVector = "vector"
	ResultTypeMatrix = "matrix"
)

type ResultType string

type Query struct {
	Range  v1.Range
	PromQL string
}

type QueryResponse struct {
	Status string            `json:"status"`
	Data   QueryResponseData `json:"data"`
}

type QueryResponseData struct {
	ResultType ResultType  `json:"resultType"`
	Result     ResultValue `json:"result"`
	Stats      interface{} `json:"-"`
}

type ResultValue interface {
	Type() ResultType
}

type Matrix []pmod.SampleStream

func (Matrix) Type() ResultType { return ResultTypeMatrix }

func (Matrix) String() string { return ResultTypeMatrix }

func newTransport(timeout time.Duration, skipTLS bool, capath string, userCertPath string, userKeyPath string) *http.Transport {
	transport := &http.Transport{
		DialContext:     (&net.Dialer{Timeout: timeout}).DialContext,
		IdleConnTimeout: timeout,
	}

	if skipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		log.Debugf("skipping TLS checks. SSL certificate verification is now disabled !")
	} else if capath != "" || userCertPath != "" {
		transport.TLSClientConfig = &tls.Config{}

		if capath != "" {
			caCert, err := os.ReadFile(capath)
			if err != nil {
				log.Errorf("Cannot load ca certificate: %v", err)
			} else {
				pool := x509.NewCertPool()
				pool.AppendCertsFromPEM(caCert)
				transport.TLSClientConfig.RootCAs = pool
			}
		}

		if userCertPath != "" {
			cert, err := tls.LoadX509KeyPair(userCertPath, userKeyPath)
			if err != nil {
				log.Errorf("Cannot load user certificate: %v", err)
			} else {
				transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
			}
		}
	}

	return transport
}

func newClient(timeout time.Duration, skipTLS bool, caPath string, tokenPath string, url string) (api.Client, error) {
	if useMocks {
		return api.NewClient(api.Config{})
	}

	maybeTLS := newTransport(timeout, skipTLS, caPath, "", "")

	var roundTripper http.RoundTripper
	if tokenPath != "" {
		bytes, err := os.ReadFile(tokenPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse authorization path '%s': %w", tokenPath, err)
		}
		roundTripper = pconf.NewAuthorizationCredentialsRoundTripper("Bearer", pconf.NewInlineSecret(string(bytes)), maybeTLS)
	} else {
		roundTripper = maybeTLS
	}

	return api.NewClient(api.Config{
		Address:      url,
		RoundTripper: roundTripper,
	})
}

func executeQueryRange(ctx context.Context, cl api.Client, q *Query) (pmod.Value, error) {
	log.Debugf("executeQueryRange: %v; promQL=%s", q.Range, q.PromQL)
	v1api := v1.NewAPI(cl)
	result, warnings, err := v1api.QueryRange(ctx, q.PromQL, q.Range)
	log.Tracef("Result:\n%v", result)
	if len(warnings) > 0 {
		log.Infof("executeQueryRange warnings: %v", warnings)
	}
	if err != nil {
		log.Tracef("Error:\n%v", err)
		return nil, fmt.Errorf("error from Prometheus query: %w", err)
	}

	return result, nil
}

func queryMatrix(ctx context.Context, cl api.Client, q *Query) (QueryResponse, error) {
	if useMocks {
		return matrixMock(), nil
	}

	resp, err := executeQueryRange(ctx, cl, q)
	if err != nil {
		log.WithError(err).Error("Error in QueryMatrix")
		return QueryResponse{}, err
	}
	// Transform response
	m, ok := resp.(pmod.Matrix)
	if !ok {
		err := fmt.Errorf("QueryMatrix: wrong return type: %T", resp)
		log.Error(err.Error())
		return QueryResponse{}, err
	}
	var convMatrix Matrix
	for i := range m {
		convMatrix = append(convMatrix, *m[i])
	}
	qr := QueryResponse{
		Data: QueryResponseData{
			ResultType: ResultTypeMatrix,
			Result:     convMatrix,
		},
	}
	return qr, nil
}
