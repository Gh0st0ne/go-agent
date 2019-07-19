// Copyright (c) 2016 - 2019 Sqreen. All Rights Reserved.
// Please refer to our terms for more information:
// https://www.sqreen.io/terms.html

package callback_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/sqreen/go-agent/agent/internal/backend/api"
	"github.com/sqreen/go-agent/agent/internal/rule/callback"
	"github.com/sqreen/go-agent/agent/sqlib/sqhook"
	"github.com/stretchr/testify/require"
)

func TestNewWriteHTTPRedirectionCallbacks(t *testing.T) {
	RunCallbackTest(t, TestConfig{
		CallbacksCtor: callback.NewWriteHTTPRedirectionCallbacks,
		ExpectProlog:  true,
		PrologType:    reflect.TypeOf(callback.WriteHTTPRedirectionPrologCallbackType(nil)),
		EpilogType:    reflect.TypeOf(callback.WriteHTTPRedirectionEpilogCallbackType(nil)),
		InvalidTestCases: []interface{}{
			nil,
			33,
			"yet another wrong type",
			&api.CustomErrorPageRuleDataEntry{},
			&api.RedirectionRuleDataEntry{},
			&api.RedirectionRuleDataEntry{"http//sqreen.com"},
		},
		ValidTestCases: []ValidTestCase{
			{
				Rule: &FakeRule{
					config: &api.RedirectionRuleDataEntry{"http://sqreen.com"},
				},
				TestCallbacks: func(t *testing.T, rule *FakeRule, prolog, epilog sqhook.Callback) {
					// Call it and check the behaviour follows the rule's data
					actualProlog, ok := prolog.(callback.WriteHTTPRedirectionPrologCallbackType)
					require.True(t, ok)
					var (
						statusCode int
						headers    http.Header
					)
					err := actualProlog(nil, nil, nil, &headers, &statusCode, nil)
					// Check it behaves as expected
					require.NoError(t, err)
					require.Equal(t, http.StatusSeeOther, statusCode)
					require.NotNil(t, headers)
					require.Equal(t, "http://sqreen.com", headers.Get("Location"))

					// Test the epilog if any
					if epilog != nil {
						actualEpilog, ok := epilog.(callback.WriteHTTPRedirectionEpilogCallbackType)
						require.True(t, ok)
						actualEpilog(&sqhook.Context{})
					}
				},
			},
		},
	})
}