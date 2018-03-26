package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/satori/go.uuid"

	"github.com/TykTechnologies/tyk/config"
	"github.com/TykTechnologies/tyk/test"
)

func TestProcessRequestLiveQuotaLimit(t *testing.T) {
	// setup global config
	config.Global.EnforceOrgQuotas = true
	config.Global.ExperimentalProcessOrgOffThread = false

	// run test server
	ts := newTykTestServer()
	defer ts.Close()

	// load API
	orgID := "test-org-" + uuid.NewV4().String()
	buildAndLoadAPI(func(spec *APISpec) {
		spec.UseKeylessAccess = true
		spec.OrgID = orgID
		spec.Proxy.ListenPath = "/"
	})

	// create org key with quota
	ts.Run(t, test.TestCase{
		Path:      "/tyk/org/keys/" + orgID + "?reset_quota=1",
		AdminAuth: true,
		Method:    http.MethodPost,
		Code:      http.StatusOK,
		Data: map[string]interface{}{
			"quota_max":          10,
			"quota_remaining":    10,
			"quota_renewal_rate": 3,
			"org_id":             orgID,
		},
	})

	t.Run("Process request live with quota", func(t *testing.T) {
		// 1st ten requests within quota
		for i := 0; i < 10; i++ {
			ts.Run(t, test.TestCase{
				Code: http.StatusOK,
			})
		}

		// next request should fail with 403 as it is out of quota
		ts.Run(t, test.TestCase{
			Code: http.StatusForbidden,
		})

		// wait for renewal
		time.Sleep(4 * time.Second)

		// next one should be OK
		ts.Run(t, test.TestCase{
			Code: http.StatusOK,
		})
	})
}

func TestProcessRequestOffThreadQuotaLimit(t *testing.T) {
	// setup global config
	config.Global.EnforceOrgQuotas = true
	config.Global.ExperimentalProcessOrgOffThread = true

	// run test server
	ts := newTykTestServer()
	defer ts.Close()

	// load API
	orgID := "test-org-" + uuid.NewV4().String()
	buildAndLoadAPI(func(spec *APISpec) {
		spec.UseKeylessAccess = true
		spec.OrgID = orgID
		spec.Proxy.ListenPath = "/"
	})

	// create org key with quota
	ts.Run(t, test.TestCase{
		Path:      "/tyk/org/keys/" + orgID + "?reset_quota=1",
		AdminAuth: true,
		Method:    http.MethodPost,
		Code:      http.StatusOK,
		Data: map[string]interface{}{
			"quota_max":          10,
			"quota_remaining":    10,
			"quota_renewal_rate": 3,
			"org_id":             orgID,
		},
	})

	t.Run("Process request off thread with quota", func(t *testing.T) {
		// at least first 10 requests within quota should be OK
		for i := 0; i < 10; i++ {
			ts.Run(t, test.TestCase{
				Code: http.StatusOK,
			})
		}

		// some of next request should fail with 403 as it is out of quota
		failed := false
		i := 0
		for i = 0; i < 10; i++ {
			res, _ := ts.Run(t, test.TestCase{})
			res.Body.Close()
			if res.StatusCode == http.StatusForbidden {
				failed = true
				break
			}
		}
		if !failed {
			t.Error("Requests don't fail after quota exceeded")
		} else {
			t.Logf("Failed with 403 after %d requests over quota", i)
		}

		// wait for renewal
		time.Sleep(4 * time.Second)

		// next 10 requests should be OK again
		ok := false
		for i = 0; i < 10; i++ {
			res, _ := ts.Run(t, test.TestCase{})
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				ok = true
				break
			}
		}
		if !ok {
			t.Error("Requests still failing after quota renewal")
		} else {
			t.Logf("Started responding with 200 after %d requests after quota renewal", i)
		}
	})
}

func TestProcessRequestLiveRedisRollingLimiter(t *testing.T) {
	// setup global config
	config.Global.EnforceOrgQuotas = true
	config.Global.EnableRedisRollingLimiter = true
	config.Global.ExperimentalProcessOrgOffThread = false

	// run test server
	ts := newTykTestServer()
	defer ts.Close()

	// load API
	orgID := "test-org-" + uuid.NewV4().String()
	buildAndLoadAPI(func(spec *APISpec) {
		spec.UseKeylessAccess = true
		spec.OrgID = orgID
		spec.Proxy.ListenPath = "/"
	})

	// create org key with quota
	ts.Run(t, test.TestCase{
		Path:      "/tyk/org/keys/" + orgID + "?reset_quota=1",
		AdminAuth: true,
		Method:    http.MethodPost,
		Code:      http.StatusOK,
		Data: map[string]interface{}{
			"quota_max": -1,
			"allowance": 10,
			"rate":      10,
			"per":       1,
			"org_id":    orgID,
		},
	})

	t.Run("Process request live with rate limit", func(t *testing.T) {
		// ten requests per sec should be OK
		for i := 0; i < 10; i++ {
			ts.Run(t, test.TestCase{
				Code: http.StatusOK,
			})
		}

		// wait for next time window
		time.Sleep(2 * time.Second)

		// try to run over rate limit
		reqNum := 1
		for {
			resp, _ := ts.Run(t, test.TestCase{})
			resp.Body.Close()
			if resp.StatusCode == http.StatusForbidden {
				break
			}
			reqNum++
		}

		if reqNum < 10 {
			t.Errorf("Started failing too early after %d requests", reqNum)
		} else {
			t.Logf("Started failing after %d requests over limit", reqNum-10)
		}
	})
}

func TestProcessRequestOffThreadRedisRollingLimiter(t *testing.T) {
	// setup global config
	config.Global.EnforceOrgQuotas = true
	config.Global.EnableRedisRollingLimiter = true
	config.Global.ExperimentalProcessOrgOffThread = true

	// run test server
	ts := newTykTestServer()
	defer ts.Close()

	// load API
	orgID := "test-org-" + uuid.NewV4().String()
	buildAndLoadAPI(func(spec *APISpec) {
		spec.UseKeylessAccess = true
		spec.OrgID = orgID
		spec.Proxy.ListenPath = "/"
	})

	// create org key with quota
	ts.Run(t, test.TestCase{
		Path:      "/tyk/org/keys/" + orgID + "?reset_quota=1",
		AdminAuth: true,
		Method:    http.MethodPost,
		Code:      http.StatusOK,
		Data: map[string]interface{}{
			"quota_max": -1,
			"allowance": 10,
			"rate":      10,
			"per":       1,
			"org_id":    orgID,
		},
	})

	t.Run("Process request off thread with rate limit", func(t *testing.T) {
		// ten requests per sec should be OK
		for i := 0; i < 10; i++ {
			ts.Run(t, test.TestCase{
				Code: http.StatusOK,
			})
		}

		// wait for next time window
		time.Sleep(2 * time.Second)

		// try to run over rate limit
		reqNum := 1
		for {
			resp, _ := ts.Run(t, test.TestCase{})
			resp.Body.Close()
			if resp.StatusCode == http.StatusForbidden {
				break
			}
			reqNum++
		}

		if reqNum < 10 {
			t.Errorf("Started failing too early after %d requests", reqNum)
		} else {
			t.Logf("Started failing after %d requests over limit", reqNum-10)
		}
	})
}
