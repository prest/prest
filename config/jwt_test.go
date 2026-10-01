package config

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// Regression coverage for GHSA-fj7v-859r-2fm4: when default JWT enforcement is
// enabled but no verification material is configured, ensureJWTConfig disables
// the middleware instead of leaving an empty HMAC key active.
func TestEnsureJWTConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		cfg             Prest
		wantAuthEnabled bool
		wantDefaultJWT  bool
		wantJWTKey      string
	}{
		{
			name:           "default JWT on, no material → disabled",
			cfg:            Prest{EnableDefaultJWT: true},
			wantDefaultJWT: false,
		},
		{
			name:           "default JWT on, HMAC key set → unchanged",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "test-jwt-hmac-secret-key-32bytes"},
			wantDefaultJWT: true,
			wantJWTKey:     "test-jwt-hmac-secret-key-32bytes",
		},
		{
			name:           "default JWT on, JWKS set → unchanged",
			cfg:            Prest{EnableDefaultJWT: true, JWTJWKS: `{"keys":[]}`},
			wantDefaultJWT: true,
		},
		{
			name:           "default JWT on, well-known URL set → unchanged",
			cfg:            Prest{EnableDefaultJWT: true, JWTWellKnownURL: "http://example.test/.well-known"},
			wantDefaultJWT: true,
		},
		{
			name:           "default JWT off → unchanged",
			cfg:            Prest{EnableDefaultJWT: false},
			wantDefaultJWT: false,
		},
		{
			name:           "debug bypass mirrors middleware/config.go → unchanged",
			cfg:            Prest{EnableDefaultJWT: true, Debug: true},
			wantDefaultJWT: true,
		},
		{
			name:            "auth enabled, empty key → auth disabled",
			cfg:             Prest{AuthEnabled: true},
			wantAuthEnabled: false,
		},
		{
			name:            "auth enabled, key set → unchanged",
			cfg:             Prest{AuthEnabled: true, JWTKey: "test-jwt-hmac-secret-key-32bytes"},
			wantAuthEnabled: true,
			wantJWTKey:      "test-jwt-hmac-secret-key-32bytes",
		},
		{
			name:           "default JWT on, short HMAC key → cleared and disabled",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "s3cr3t", JWTAlgo: "HS256"},
			wantDefaultJWT: false,
		},
		{
			name:            "auth enabled, short HMAC key → auth disabled",
			cfg:             Prest{AuthEnabled: true, JWTKey: "s3cr3t", JWTAlgo: "HS256"},
			wantAuthEnabled: false,
		},
		{
			name:           "short HMAC key with JWKS → key cleared, JWT stays on via JWKS",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "s3cr3t", JWTAlgo: "HS256", JWTJWKS: `{"keys":[]}`},
			wantDefaultJWT: true,
		},
		{
			name:           "HS384 exact 48-byte key → unchanged",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh", JWTAlgo: "HS384"},
			wantDefaultJWT: true,
			wantJWTKey:     "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh",
		},
		{
			name:           "HS384 47-byte key → cleared and disabled",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh", JWTAlgo: "HS384"},
			wantDefaultJWT: false,
		},
		{
			name:           "HS384 47-byte key with JWKS → cleared, JWT stays on via JWKS",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh", JWTAlgo: "HS384", JWTJWKS: `{"keys":[]}`},
			wantDefaultJWT: true,
		},
		{
			name:           "HS512 exact 64-byte key → unchanged",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh", JWTAlgo: "HS512"},
			wantDefaultJWT: true,
			wantJWTKey:     "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh",
		},
		{
			name:           "HS512 63-byte key → cleared and disabled",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh", JWTAlgo: "HS512"},
			wantDefaultJWT: false,
		},
		{
			name:           "HS512 63-byte key with JWKS → cleared, JWT stays on via JWKS",
			cfg:            Prest{EnableDefaultJWT: true, JWTKey: "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh", JWTAlgo: "HS512", JWTJWKS: `{"keys":[]}`},
			wantDefaultJWT: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			ensureJWTConfig(&cfg)
			require.Equal(t, tc.wantAuthEnabled, cfg.AuthEnabled)
			require.Equal(t, tc.wantDefaultJWT, cfg.EnableDefaultJWT)
			require.Equal(t, tc.wantJWTKey, cfg.JWTKey)
		})
	}
}

func Test_redactURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"strips userinfo", "https://user:secret@idp.example.com/jwks", "https://idp.example.com/jwks"},
		{"strips query", "https://idp.example.com/jwks?token=abc", "https://idp.example.com/jwks"},
		{"strips fragment", "https://idp.example.com/jwks#f", "https://idp.example.com/jwks"},
		{"keeps host and path", "https://idp.example.com:8443/realm/jwks", "https://idp.example.com:8443/realm/jwks"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, redactURL(tc.in))
		})
	}
}

func Test_fetchJWKS(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		runFetchJWKSSuccessTest(t)
	})

	t.Run("skips when JWKS already set", func(t *testing.T) {
		t.Parallel()
		cfg := &Prest{
			JWTWellKnownURL: "http://example.test/.well-known",
			JWTJWKS:         `{"keys":[]}`,
		}
		fetchJWKS(cfg)
		require.Equal(t, `{"keys":[]}`, cfg.JWTJWKS)
	})

	t.Run("HTTP GET failure", func(t *testing.T) {
		t.Parallel()
		cfg := &Prest{JWTWellKnownURL: "http://127.0.0.1:1"}
		fetchJWKS(cfg)
		require.Empty(t, cfg.JWTJWKS)
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not json"))
		}))
		t.Cleanup(srv.Close)

		cfg := &Prest{JWTWellKnownURL: srv.URL}
		fetchJWKS(cfg)
		require.Empty(t, cfg.JWTJWKS)
	})

	t.Run("non-2xx JWKS response is rejected", func(t *testing.T) {
		t.Parallel()
		// JWKS endpoint returns syntactically valid JWKS JSON but an error status;
		// the body must not be trusted.
		jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"keys":[]}`))
		}))
		t.Cleanup(jwksSrv.Close)

		wellKnownSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jwks_uri":"` + jwksSrv.URL + `"}`))
		}))
		t.Cleanup(wellKnownSrv.Close)

		cfg := &Prest{JWTWellKnownURL: wellKnownSrv.URL}
		fetchJWKS(cfg)
		require.Empty(t, cfg.JWTJWKS)
	})

	t.Run("missing jwks_uri", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"issuer":"test"}`))
		}))
		t.Cleanup(srv.Close)

		cfg := &Prest{JWTWellKnownURL: srv.URL}
		fetchJWKS(cfg)
		require.Empty(t, cfg.JWTJWKS)
	})

	t.Run("jwks_uri not a string", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jwks_uri":123}`))
		}))
		t.Cleanup(srv.Close)

		cfg := &Prest{JWTWellKnownURL: srv.URL}
		fetchJWKS(cfg)
		require.Empty(t, cfg.JWTJWKS)
	})

	t.Run("JWKS fetch failure", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jwks_uri":"http://127.0.0.1:1"}`))
		}))
		t.Cleanup(srv.Close)

		cfg := &Prest{JWTWellKnownURL: srv.URL}
		fetchJWKS(cfg)
		require.Empty(t, cfg.JWTJWKS)
	})
}

func runFetchJWKSSuccessTest(t *testing.T) {
	t.Helper()

	serverJWKS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[{"alg":"RS256","e":"AQAB","kid":"lmjNOucrGdRiN7XlpWJbQRIzSeKBS7OD-92xrhch6kw","kty":"RSA","n":"9GPbUNJ_7dgq8k0eTbcCZtFMn-oTVpFHjzIi7nuyMm9TvIZNyu0q0O3buSIVTUWWhlakSgTp7hrRbldvxLmA4RSSs8oUw2Pm64q9oCdr0eXcnhL6mnfHASwpVed-aKMbM1Zlh1buDjPU0Ah_6D8sZaxqfOtMfrhT9LySbi91k2Hu16YJ6QK_RTj5BNjLZZSs2ns8-JdZKA-oL0RQwkEqO_QJrRvTWUhwguzpx4zACWc5zAQSWvDImbynH3N9L-rt2KoK3p2Zd0YZlCnZzK0iyYUHkVtTVixTFkYc-itceyZD64Z49q8vu478gIvu4dI8m3GIYeisZkKWBE5sjczvvw","use":"sig","x5c":["MIICmzCCAYMCBgGOLghSADANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQD0Y9tQ0n/t2CryTR5NtwJm0Uyf6hNWkUePMiLue7Iyb1O8hk3K7SrQ7du5IhVNRZaGVqRKBOnuGtFuV2/EuYDhFJKzyhTDY+brir2gJ2vR5dyeEvqad8cBLClV535ooxszVmWHVu4OM9TQCH/oPyxlrGp860x+uFP0vJJuL3WTYe7XpgnpAr9FOPkE2MtllKzaezz4l1koD6gvRFDCQSo79AmtG9NZSHCC7OnHjMAJZznMBBJa8MiZvKcfc30v6u3YqgrenZl3RhmUKdnMrSLJhQeRW1NWLFMWRhz6K1x7JkPrhnj2ry+7jvyAi+7h0jybcYhh6KxmQpYETmyNzO+/AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAAIDB54QwrWSQPou8UlGkpA8D3/Ws0ZGNiFutyIAQU0bzhzSB99AMsPl/4OJm5CGqpZMVyuLFgQHlMaArzeQJK7/8qN6piDZPP6A2lSRYuMJ/a8ciIVvjnepSUF+xx7PqeAnoarH8lxbdwhloBswnxn4iNcWTTMnxo73Ak9jpabj1m1a4e9+li6S8xCyA1AHxFXbjjAp5GxRvcUV2o3rMsDqdjM0IoU/+NNuCGtKApdTZNpFuk71AoKpM2/oxjuexEpOggyF30Pk5IdAgNtFMfD+pwcqzvSACbtKvk6VnSx4UtsFPWuizhWefWIkuV+7ml60NFMyD3eo28U9BQs2veU="],"x5t":"tUcTw0bM8ciXw9zIMlalEfyxdd8","x5t#S256":"eF-XsrHWa6gw8qC4W8RXJgA49xvac_7V-Tz7fdpS7ZM"},{"alg":"RSA-OAEP","e":"AQAB","kid":"V3rRzf_j1beZjEmQnDeT8r8ZVnXpjW1Gk3635CTCEGk","kty":"RSA","n":"1q1Iz-eyhnCWCBRKgq0xKm6cF2zHAi_a-L99OdwgnUgoGfut5bBTU2hGx9R1IGKn0loDjICtU64DVFpOaT7jY7oIG4BsQN3Et5H6O3XlVim5NQgMYVC6hKAreqnnVylUk-XfVvrQOotVkGfMFdARuBaLx1ubFxIHUONi2Mjgl2nZ8mmKg_GCsd5uKfJJ965zqSQu1CFn26YccTPp2doih4rykTGPVJdL5PVp3z4t9rTlahHbgCvv3E50yVK7LCNgtS9nmcZbD0meLqIZi3MoV0dBB_9C-qrEsevAIlPuXUmwtcbyDXOb1m7Xq_MPV_EASzoPYYjmk3k09zJ_p1EUTQ","use":"enc","x5c":["MIICmzCCAYMCBgGOLghSlzANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWrUjP57KGcJYIFEqCrTEqbpwXbMcCL9r4v3053CCdSCgZ+63lsFNTaEbH1HUgYqfSWgOMgK1TrgNUWk5pPuNjuggbgGxA3cS3kfo7deVWKbk1CAxhULqEoCt6qedXKVST5d9W+tA6i1WQZ8wV0BG4FovHW5sXEgdQ42LYyOCXadnyaYqD8YKx3m4p8kn3rnOpJC7UIWfbphxxM+nZ2iKHivKRMY9Ul0vk9WnfPi32tOVqEduAK+/cTnTJUrssI2C1L2eZxlsPSZ4uohmLcyhXR0EH/0L6qsSx68AiU+5dSbC1xvINc5vWbter8w9X8QBLOg9hiOaTeTT3Mn+nURRNAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAIKBZNe4GmyfqRW6Ee8ai1umbstAmyK3W1kP2i0xxINTlvY2rwblV8UCrdyi3laD7zvZy1midZmpKqtZqWpiNigeZ5aUt76paYvdSl5TAuvZGDGoEAhmmECbnDSQKLp36rCn7NlrgiTDfZZ2PvIKZ3cXClzqXLF/iC6uGiKOgY5yOFOa5QgsfItpJmmxHtTzrRF70RVsbZCexB1Lt4bcId6Y3x2w7JNUjKIhf1RZ3QZx8+3xBM4cJ83h2J4nE0+IlFeAJL3VLGdeOk+z+FGMu2mYkxJwkxd9Wl2ubqrRcNy0t61Bgp3s40BgD10pzvawTXl7lEgabc/jzN2R0lcXmLo="],"x5t":"n5Y_Obidr330txi13j50zHzVbfg","x5t#S256":"f-Hrw_t_qUq86Ux0J2EckWVycuM3L_IjdOK6DW0DFoc"}]}`))
	}))
	t.Cleanup(serverJWKS.Close)

	serverWellKnown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issuer":"http://127.0.0.1:8080/realms/master","authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth","token_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token","introspection_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token/introspect","userinfo_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/userinfo","end_session_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/logout","frontchannel_logout_session_supported":true,"frontchannel_logout_supported":true,"jwks_uri":"` + serverJWKS.URL + `","check_session_iframe":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/login-status-iframe.html","grant_types_supported":["authorization_code","implicit","refresh_token","password","client_credentials","urn:openid:params:grant-type:ciba","urn:ietf:params:oauth:grant-type:device_code"],"acr_values_supported":["0","1"],"response_types_supported":["code","none","id_token","token","id_token token","code id_token","code token","code id_token token"],"subject_types_supported":["public","pairwise"],"id_token_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"id_token_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"id_token_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"userinfo_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512","none"],"userinfo_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"userinfo_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"request_object_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512","none"],"request_object_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"request_object_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"response_modes_supported":["query","fragment","form_post","query.jwt","fragment.jwt","form_post.jwt","jwt"],"registration_endpoint":"http://127.0.0.1:8080/realms/master/clients-registrations/openid-connect","token_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"token_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"introspection_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"introspection_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"authorization_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"authorization_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"authorization_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"claims_supported":["aud","sub","iss","auth_time","name","given_name","family_name","preferred_username","email","acr"],"claim_types_supported":["normal"],"claims_parameter_supported":true,"scopes_supported":["openid","roles","offline_access","email","microprofile-jwt","web-origins","acr","phone","profile","address"],"request_parameter_supported":true,"request_uri_parameter_supported":true,"require_request_uri_registration":true,"code_challenge_methods_supported":["plain","S256"],"tls_client_certificate_bound_access_tokens":true,"revocation_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/revoke","revocation_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"revocation_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"backchannel_logout_supported":true,"backchannel_logout_session_supported":true,"device_authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth/device","backchannel_token_delivery_modes_supported":["poll","ping"],"backchannel_authentication_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/ciba/auth","backchannel_authentication_request_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","ES256","RS256","ES512","PS256","PS512","RS512"],"require_pushed_authorization_requests":false,"pushed_authorization_request_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/par/request","mtls_endpoint_aliases":{"token_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token","revocation_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/revoke","introspection_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token/introspect","device_authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth/device","registration_endpoint":"http://127.0.0.1:8080/realms/master/clients-registrations/openid-connect","userinfo_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/userinfo","pushed_authorization_request_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/par/request","backchannel_authentication_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/ciba/auth"},"authorization_response_iss_parameter_supported":true}`))
	}))
	t.Cleanup(serverWellKnown.Close)

	c := &Prest{JWTWellKnownURL: serverWellKnown.URL}
	fetchJWKS(c)
	require.Equal(t, `{"keys":[{"alg":"RS256","e":"AQAB","kid":"lmjNOucrGdRiN7XlpWJbQRIzSeKBS7OD-92xrhch6kw","kty":"RSA","n":"9GPbUNJ_7dgq8k0eTbcCZtFMn-oTVpFHjzIi7nuyMm9TvIZNyu0q0O3buSIVTUWWhlakSgTp7hrRbldvxLmA4RSSs8oUw2Pm64q9oCdr0eXcnhL6mnfHASwpVed-aKMbM1Zlh1buDjPU0Ah_6D8sZaxqfOtMfrhT9LySbi91k2Hu16YJ6QK_RTj5BNjLZZSs2ns8-JdZKA-oL0RQwkEqO_QJrRvTWUhwguzpx4zACWc5zAQSWvDImbynH3N9L-rt2KoK3p2Zd0YZlCnZzK0iyYUHkVtTVixTFkYc-itceyZD64Z49q8vu478gIvu4dI8m3GIYeisZkKWBE5sjczvvw","use":"sig","x5c":["MIICmzCCAYMCBgGOLghSADANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQD0Y9tQ0n/t2CryTR5NtwJm0Uyf6hNWkUePMiLue7Iyb1O8hk3K7SrQ7du5IhVNRZaGVqRKBOnuGtFuV2/EuYDhFJKzyhTDY+brir2gJ2vR5dyeEvqad8cBLClV535ooxszVmWHVu4OM9TQCH/oPyxlrGp860x+uFP0vJJuL3WTYe7XpgnpAr9FOPkE2MtllKzaezz4l1koD6gvRFDCQSo79AmtG9NZSHCC7OnHjMAJZznMBBJa8MiZvKcfc30v6u3YqgrenZl3RhmUKdnMrSLJhQeRW1NWLFMWRhz6K1x7JkPrhnj2ry+7jvyAi+7h0jybcYhh6KxmQpYETmyNzO+/AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAAIDB54QwrWSQPou8UlGkpA8D3/Ws0ZGNiFutyIAQU0bzhzSB99AMsPl/4OJm5CGqpZMVyuLFgQHlMaArzeQJK7/8qN6piDZPP6A2lSRYuMJ/a8ciIVvjnepSUF+xx7PqeAnoarH8lxbdwhloBswnxn4iNcWTTMnxo73Ak9jpabj1m1a4e9+li6S8xCyA1AHxFXbjjAp5GxRvcUV2o3rMsDqdjM0IoU/+NNuCGtKApdTZNpFuk71AoKpM2/oxjuexEpOggyF30Pk5IdAgNtFMfD+pwcqzvSACbtKvk6VnSx4UtsFPWuizhWefWIkuV+7ml60NFMyD3eo28U9BQs2veU="],"x5t":"tUcTw0bM8ciXw9zIMlalEfyxdd8","x5t#S256":"eF-XsrHWa6gw8qC4W8RXJgA49xvac_7V-Tz7fdpS7ZM"},{"alg":"RSA-OAEP","e":"AQAB","kid":"V3rRzf_j1beZjEmQnDeT8r8ZVnXpjW1Gk3635CTCEGk","kty":"RSA","n":"1q1Iz-eyhnCWCBRKgq0xKm6cF2zHAi_a-L99OdwgnUgoGfut5bBTU2hGx9R1IGKn0loDjICtU64DVFpOaT7jY7oIG4BsQN3Et5H6O3XlVim5NQgMYVC6hKAreqnnVylUk-XfVvrQOotVkGfMFdARuBaLx1ubFxIHUONi2Mjgl2nZ8mmKg_GCsd5uKfJJ965zqSQu1CFn26YccTPp2doih4rykTGPVJdL5PVp3z4t9rTlahHbgCvv3E50yVK7LCNgtS9nmcZbD0meLqIZi3MoV0dBB_9C-qrEsevAIlPuXUmwtcbyDXOb1m7Xq_MPV_EASzoPYYjmk3k09zJ_p1EUTQ","use":"enc","x5c":["MIICmzCCAYMCBgGOLghSlzANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWrUjP57KGcJYIFEqCrTEqbpwXbMcCL9r4v3053CCdSCgZ+63lsFNTaEbH1HUgYqfSWgOMgK1TrgNUWk5pPuNjuggbgGxA3cS3kfo7deVWKbk1CAxhULqEoCt6qedXKVST5d9W+tA6i1WQZ8wV0BG4FovHW5sXEgdQ42LYyOCXadnyaYqD8YKx3m4p8kn3rnOpJC7UIWfbphxxM+nZ2iKHivKRMY9Ul0vk9WnfPi32tOVqEduAK+/cTnTJUrssI2C1L2eZxlsPSZ4uohmLcyhXR0EH/0L6qsSx68AiU+5dSbC1xvINc5vWbter8w9X8QBLOg9hiOaTeTT3Mn+nURRNAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAIKBZNe4GmyfqRW6Ee8ai1umbstAmyK3W1kP2i0xxINTlvY2rwblV8UCrdyi3laD7zvZy1midZmpKqtZqWpiNigeZ5aUt76paYvdSl5TAuvZGDGoEAhmmECbnDSQKLp36rCn7NlrgiTDfZZ2PvIKZ3cXClzqXLF/iC6uGiKOgY5yOFOa5QgsfItpJmmxHtTzrRF70RVsbZCexB1Lt4bcId6Y3x2w7JNUjKIhf1RZ3QZx8+3xBM4cJ83h2J4nE0+IlFeAJL3VLGdeOk+z+FGMu2mYkxJwkxd9Wl2ubqrRcNy0t61Bgp3s40BgD10pzvawTXl7lEgabc/jzN2R0lcXmLo="],"x5t":"n5Y_Obidr330txi13j50zHzVbfg","x5t#S256":"f-Hrw_t_qUq86Ux0J2EckWVycuM3L_IjdOK6DW0DFoc"}]}`, c.JWTJWKS)
}
