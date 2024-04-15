package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Setenv("PREST_CONF", "../testdata/prest.toml")
	Load()
	require.Greaterf(t, len(PrestConf.AccessConf.Tables), 2,
		"expected > 2, got: %d", len(PrestConf.AccessConf.Tables))

	for _, ignoretable := range PrestConf.AccessConf.IgnoreTable {
		require.Equal(t, "test_permission_does_not_exist", ignoretable,
			"expected ['test_permission_does_not_exist'], but got another result")
	}
	require.True(t, PrestConf.AccessConf.Restrict, "expected true, but got false")
	require.Equal(t, 60, PrestConf.HTTPTimeout)
}

func TestParse(t *testing.T) {
	t.Run("no envs", func(t *testing.T) {
		t.Setenv("PREST_CONF", "../notfound.toml")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, 3000, cfg.HTTPPort)
		require.Equal(t, "prest-test", cfg.PGDatabase)
		require.Equal(t, "postgres", cfg.PGHost)
		require.Equal(t, "postgres", cfg.PGUser)
		require.Equal(t, "postgres", cfg.PGPass)
		require.Equal(t, true, cfg.PGCache)
		require.Equal(t, true, cfg.SingleDB)
		require.Equal(t, "disable", cfg.SSLMode)
		require.Equal(t, false, cfg.Debug)
		require.Equal(t, 1, cfg.Version)
		require.Equal(t, true, cfg.AccessConf.Restrict)
	})

	t.Run("PREST_CONF", func(t *testing.T) {
		t.Setenv("PREST_CONF", "../testdata/prest.toml")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, 3000, cfg.HTTPPort)
		require.Equal(t, "prest-test", cfg.PGDatabase)
	})

	t.Run("PREST_HTTP_PORT and unset PREST_JWT_DEFAULT", func(t *testing.T) {
		t.Setenv("PREST_HTTP_PORT", "4000")
		os.Unsetenv("PREST_JWT_DEFAULT")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, 4000, cfg.HTTPPort)
		require.True(t, cfg.EnableDefaultJWT)
	})

	t.Run("empty PREST_CONF and falsey PREST_JWT_DEFAULT", func(t *testing.T) {
		t.Setenv("PREST_CONF", "")
		t.Setenv("PREST_JWT_DEFAULT", "false")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, 3000, cfg.HTTPPort)
		require.False(t, cfg.EnableDefaultJWT)
	})

	t.Run("empty PREST_CONF", func(t *testing.T) {
		t.Setenv("PREST_CONF", "")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, 3000, cfg.HTTPPort)
	})

	t.Run("PREST_JWT_KEY", func(t *testing.T) {
		t.Setenv("PREST_JWT_KEY", "s3cr3t")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, "s3cr3t", cfg.JWTKey)
		require.Equal(t, "HS256", cfg.JWTAlgo)
	})

	t.Run("PREST_JWT_ALGO", func(t *testing.T) {
		t.Setenv("PREST_JWT_ALGO", "HS512")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, "HS512", cfg.JWTAlgo)
	})

	t.Run("PREST_JWT_WELLKNOWNURL", func(t *testing.T) {
		serverJWKS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"keys":[{"kid":"lmjNOucrGdRiN7XlpWJbQRIzSeKBS7OD-92xrhch6kw","kty":"RSA","alg":"RS256","use":"sig","n":"9GPbUNJ_7dgq8k0eTbcCZtFMn-oTVpFHjzIi7nuyMm9TvIZNyu0q0O3buSIVTUWWhlakSgTp7hrRbldvxLmA4RSSs8oUw2Pm64q9oCdr0eXcnhL6mnfHASwpVed-aKMbM1Zlh1buDjPU0Ah_6D8sZaxqfOtMfrhT9LySbi91k2Hu16YJ6QK_RTj5BNjLZZSs2ns8-JdZKA-oL0RQwkEqO_QJrRvTWUhwguzpx4zACWc5zAQSWvDImbynH3N9L-rt2KoK3p2Zd0YZlCnZzK0iyYUHkVtTVixTFkYc-itceyZD64Z49q8vu478gIvu4dI8m3GIYeisZkKWBE5sjczvvw","e":"AQAB","x5c":["MIICmzCCAYMCBgGOLghSADANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQD0Y9tQ0n/t2CryTR5NtwJm0Uyf6hNWkUePMiLue7Iyb1O8hk3K7SrQ7du5IhVNRZaGVqRKBOnuGtFuV2/EuYDhFJKzyhTDY+brir2gJ2vR5dyeEvqad8cBLClV535ooxszVmWHVu4OM9TQCH/oPyxlrGp860x+uFP0vJJuL3WTYe7XpgnpAr9FOPkE2MtllKzaezz4l1koD6gvRFDCQSo79AmtG9NZSHCC7OnHjMAJZznMBBJa8MiZvKcfc30v6u3YqgrenZl3RhmUKdnMrSLJhQeRW1NWLFMWRhz6K1x7JkPrhnj2ry+7jvyAi+7h0jybcYhh6KxmQpYETmyNzO+/AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAAIDB54QwrWSQPou8UlGkpA8D3/Ws0ZGNiFutyIAQU0bzhzSB99AMsPl/4OJm5CGqpZMVyuLFgQHlMaArzeQJK7/8qN6piDZPP6A2lSRYuMJ/a8ciIVvjnepSUF+xx7PqeAnoarH8lxbdwhloBswnxn4iNcWTTMnxo73Ak9jpabj1m1a4e9+li6S8xCyA1AHxFXbjjAp5GxRvcUV2o3rMsDqdjM0IoU/+NNuCGtKApdTZNpFuk71AoKpM2/oxjuexEpOggyF30Pk5IdAgNtFMfD+pwcqzvSACbtKvk6VnSx4UtsFPWuizhWefWIkuV+7ml60NFMyD3eo28U9BQs2veU="],"x5t":"tUcTw0bM8ciXw9zIMlalEfyxdd8","x5t#S256":"eF-XsrHWa6gw8qC4W8RXJgA49xvac_7V-Tz7fdpS7ZM"},{"kid":"V3rRzf_j1beZjEmQnDeT8r8ZVnXpjW1Gk3635CTCEGk","kty":"RSA","alg":"RSA-OAEP","use":"enc","n":"1q1Iz-eyhnCWCBRKgq0xKm6cF2zHAi_a-L99OdwgnUgoGfut5bBTU2hGx9R1IGKn0loDjICtU64DVFpOaT7jY7oIG4BsQN3Et5H6O3XlVim5NQgMYVC6hKAreqnnVylUk-XfVvrQOotVkGfMFdARuBaLx1ubFxIHUONi2Mjgl2nZ8mmKg_GCsd5uKfJJ965zqSQu1CFn26YccTPp2doih4rykTGPVJdL5PVp3z4t9rTlahHbgCvv3E50yVK7LCNgtS9nmcZbD0meLqIZi3MoV0dBB_9C-qrEsevAIlPuXUmwtcbyDXOb1m7Xq_MPV_EASzoPYYjmk3k09zJ_p1EUTQ","e":"AQAB","x5c":["MIICmzCCAYMCBgGOLghSlzANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWrUjP57KGcJYIFEqCrTEqbpwXbMcCL9r4v3053CCdSCgZ+63lsFNTaEbH1HUgYqfSWgOMgK1TrgNUWk5pPuNjuggbgGxA3cS3kfo7deVWKbk1CAxhULqEoCt6qedXKVST5d9W+tA6i1WQZ8wV0BG4FovHW5sXEgdQ42LYyOCXadnyaYqD8YKx3m4p8kn3rnOpJC7UIWfbphxxM+nZ2iKHivKRMY9Ul0vk9WnfPi32tOVqEduAK+/cTnTJUrssI2C1L2eZxlsPSZ4uohmLcyhXR0EH/0L6qsSx68AiU+5dSbC1xvINc5vWbter8w9X8QBLOg9hiOaTeTT3Mn+nURRNAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAIKBZNe4GmyfqRW6Ee8ai1umbstAmyK3W1kP2i0xxINTlvY2rwblV8UCrdyi3laD7zvZy1midZmpKqtZqWpiNigeZ5aUt76paYvdSl5TAuvZGDGoEAhmmECbnDSQKLp36rCn7NlrgiTDfZZ2PvIKZ3cXClzqXLF/iC6uGiKOgY5yOFOa5QgsfItpJmmxHtTzrRF70RVsbZCexB1Lt4bcId6Y3x2w7JNUjKIhf1RZ3QZx8+3xBM4cJ83h2J4nE0+IlFeAJL3VLGdeOk+z+FGMu2mYkxJwkxd9Wl2ubqrRcNy0t61Bgp3s40BgD10pzvawTXl7lEgabc/jzN2R0lcXmLo="],"x5t":"n5Y_Obidr330txi13j50zHzVbfg","x5t#S256":"f-Hrw_t_qUq86Ux0J2EckWVycuM3L_IjdOK6DW0DFoc"}]}`))
		}))
		defer serverJWKS.Close()

		serverWellKnown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"issuer":"http://127.0.0.1:8080/realms/master","authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth","token_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token","introspection_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token/introspect","userinfo_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/userinfo","end_session_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/logout","frontchannel_logout_session_supported":true,"frontchannel_logout_supported":true,"jwks_uri":"` + serverJWKS.URL + `","check_session_iframe":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/login-status-iframe.html","grant_types_supported":["authorization_code","implicit","refresh_token","password","client_credentials","urn:openid:params:grant-type:ciba","urn:ietf:params:oauth:grant-type:device_code"],"acr_values_supported":["0","1"],"response_types_supported":["code","none","id_token","token","id_token token","code id_token","code token","code id_token token"],"subject_types_supported":["public","pairwise"],"id_token_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"id_token_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"id_token_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"userinfo_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512","none"],"userinfo_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"userinfo_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"request_object_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512","none"],"request_object_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"request_object_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"response_modes_supported":["query","fragment","form_post","query.jwt","fragment.jwt","form_post.jwt","jwt"],"registration_endpoint":"http://127.0.0.1:8080/realms/master/clients-registrations/openid-connect","token_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"token_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"introspection_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"introspection_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"authorization_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"authorization_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"authorization_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"claims_supported":["aud","sub","iss","auth_time","name","given_name","family_name","preferred_username","email","acr"],"claim_types_supported":["normal"],"claims_parameter_supported":true,"scopes_supported":["openid","roles","offline_access","email","microprofile-jwt","web-origins","acr","phone","profile","address"],"request_parameter_supported":true,"request_uri_parameter_supported":true,"require_request_uri_registration":true,"code_challenge_methods_supported":["plain","S256"],"tls_client_certificate_bound_access_tokens":true,"revocation_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/revoke","revocation_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"revocation_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"backchannel_logout_supported":true,"backchannel_logout_session_supported":true,"device_authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth/device","backchannel_token_delivery_modes_supported":["poll","ping"],"backchannel_authentication_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/ciba/auth","backchannel_authentication_request_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","ES256","RS256","ES512","PS256","PS512","RS512"],"require_pushed_authorization_requests":false,"pushed_authorization_request_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/par/request","mtls_endpoint_aliases":{"token_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token","revocation_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/revoke","introspection_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token/introspect","device_authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth/device","registration_endpoint":"http://127.0.0.1:8080/realms/master/clients-registrations/openid-connect","userinfo_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/userinfo","pushed_authorization_request_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/par/request","backchannel_authentication_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/ciba/auth"},"authorization_response_iss_parameter_supported":true}`))
		}))
		defer serverWellKnown.Close()

		t.Setenv("PREST_JWT_WELLKNOWNURL", serverWellKnown.URL)
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, serverWellKnown.URL, cfg.JWTWellKnownURL)
	})

	t.Run("PREST_JWT_JWKS", func(t *testing.T) {
		t.Setenv("PREST_JWT_JWKS", `{"keys":[{"kid":"lmjNOucrGdRiN7XlpWJbQRIzSeKBS7OD-92xrhch6kw","kty":"RSA","alg":"RS256","use":"sig","n":"9GPbUNJ_7dgq8k0eTbcCZtFMn-oTVpFHjzIi7nuyMm9TvIZNyu0q0O3buSIVTUWWhlakSgTp7hrRbldvxLmA4RSSs8oUw2Pm64q9oCdr0eXcnhL6mnfHASwpVed-aKMbM1Zlh1buDjPU0Ah_6D8sZaxqfOtMfrhT9LySbi91k2Hu16YJ6QK_RTj5BNjLZZSs2ns8-JdZKA-oL0RQwkEqO_QJrRvTWUhwguzpx4zACWc5zAQSWvDImbynH3N9L-rt2KoK3p2Zd0YZlCnZzK0iyYUHkVtTVixTFkYc-itceyZD64Z49q8vu478gIvu4dI8m3GIYeisZkKWBE5sjczvvw","e":"AQAB","x5c":["MIICmzCCAYMCBgGOLghSADANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQD0Y9tQ0n/t2CryTR5NtwJm0Uyf6hNWkUePMiLue7Iyb1O8hk3K7SrQ7du5IhVNRZaGVqRKBOnuGtFuV2/EuYDhFJKzyhTDY+brir2gJ2vR5dyeEvqad8cBLClV535ooxszVmWHVu4OM9TQCH/oPyxlrGp860x+uFP0vJJuL3WTYe7XpgnpAr9FOPkE2MtllKzaezz4l1koD6gvRFDCQSo79AmtG9NZSHCC7OnHjMAJZznMBBJa8MiZvKcfc30v6u3YqgrenZl3RhmUKdnMrSLJhQeRW1NWLFMWRhz6K1x7JkPrhnj2ry+7jvyAi+7h0jybcYhh6KxmQpYETmyNzO+/AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAAIDB54QwrWSQPou8UlGkpA8D3/Ws0ZGNiFutyIAQU0bzhzSB99AMsPl/4OJm5CGqpZMVyuLFgQHlMaArzeQJK7/8qN6piDZPP6A2lSRYuMJ/a8ciIVvjnepSUF+xx7PqeAnoarH8lxbdwhloBswnxn4iNcWTTMnxo73Ak9jpabj1m1a4e9+li6S8xCyA1AHxFXbjjAp5GxRvcUV2o3rMsDqdjM0IoU/+NNuCGtKApdTZNpFuk71AoKpM2/oxjuexEpOggyF30Pk5IdAgNtFMfD+pwcqzvSACbtKvk6VnSx4UtsFPWuizhWefWIkuV+7ml60NFMyD3eo28U9BQs2veU="],"x5t":"tUcTw0bM8ciXw9zIMlalEfyxdd8","x5t#S256":"eF-XsrHWa6gw8qC4W8RXJgA49xvac_7V-Tz7fdpS7ZM"},{"kid":"V3rRzf_j1beZjEmQnDeT8r8ZVnXpjW1Gk3635CTCEGk","kty":"RSA","alg":"RSA-OAEP","use":"enc","n":"1q1Iz-eyhnCWCBRKgq0xKm6cF2zHAi_a-L99OdwgnUgoGfut5bBTU2hGx9R1IGKn0loDjICtU64DVFpOaT7jY7oIG4BsQN3Et5H6O3XlVim5NQgMYVC6hKAreqnnVylUk-XfVvrQOotVkGfMFdARuBaLx1ubFxIHUONi2Mjgl2nZ8mmKg_GCsd5uKfJJ965zqSQu1CFn26YccTPp2doih4rykTGPVJdL5PVp3z4t9rTlahHbgCvv3E50yVK7LCNgtS9nmcZbD0meLqIZi3MoV0dBB_9C-qrEsevAIlPuXUmwtcbyDXOb1m7Xq_MPV_EASzoPYYjmk3k09zJ_p1EUTQ","e":"AQAB","x5c":["MIICmzCCAYMCBgGOLghSlzANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWrUjP57KGcJYIFEqCrTEqbpwXbMcCL9r4v3053CCdSCgZ+63lsFNTaEbH1HUgYqfSWgOMgK1TrgNUWk5pPuNjuggbgGxA3cS3kfo7deVWKbk1CAxhULqEoCt6qedXKVST5d9W+tA6i1WQZ8wV0BG4FovHW5sXEgdQ42LYyOCXadnyaYqD8YKx3m4p8kn3rnOpJC7UIWfbphxxM+nZ2iKHivKRMY9Ul0vk9WnfPi32tOVqEduAK+/cTnTJUrssI2C1L2eZxlsPSZ4uohmLcyhXR0EH/0L6qsSx68AiU+5dSbC1xvINc5vWbter8w9X8QBLOg9hiOaTeTT3Mn+nURRNAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAIKBZNe4GmyfqRW6Ee8ai1umbstAmyK3W1kP2i0xxINTlvY2rwblV8UCrdyi3laD7zvZy1midZmpKqtZqWpiNigeZ5aUt76paYvdSl5TAuvZGDGoEAhmmECbnDSQKLp36rCn7NlrgiTDfZZ2PvIKZ3cXClzqXLF/iC6uGiKOgY5yOFOa5QgsfItpJmmxHtTzrRF70RVsbZCexB1Lt4bcId6Y3x2w7JNUjKIhf1RZ3QZx8+3xBM4cJ83h2J4nE0+IlFeAJL3VLGdeOk+z+FGMu2mYkxJwkxd9Wl2ubqrRcNy0t61Bgp3s40BgD10pzvawTXl7lEgabc/jzN2R0lcXmLo="],"x5t":"n5Y_Obidr330txi13j50zHzVbfg","x5t#S256":"f-Hrw_t_qUq86Ux0J2EckWVycuM3L_IjdOK6DW0DFoc"}]}`)
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, `{"keys":[{"kid":"lmjNOucrGdRiN7XlpWJbQRIzSeKBS7OD-92xrhch6kw","kty":"RSA","alg":"RS256","use":"sig","n":"9GPbUNJ_7dgq8k0eTbcCZtFMn-oTVpFHjzIi7nuyMm9TvIZNyu0q0O3buSIVTUWWhlakSgTp7hrRbldvxLmA4RSSs8oUw2Pm64q9oCdr0eXcnhL6mnfHASwpVed-aKMbM1Zlh1buDjPU0Ah_6D8sZaxqfOtMfrhT9LySbi91k2Hu16YJ6QK_RTj5BNjLZZSs2ns8-JdZKA-oL0RQwkEqO_QJrRvTWUhwguzpx4zACWc5zAQSWvDImbynH3N9L-rt2KoK3p2Zd0YZlCnZzK0iyYUHkVtTVixTFkYc-itceyZD64Z49q8vu478gIvu4dI8m3GIYeisZkKWBE5sjczvvw","e":"AQAB","x5c":["MIICmzCCAYMCBgGOLghSADANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQD0Y9tQ0n/t2CryTR5NtwJm0Uyf6hNWkUePMiLue7Iyb1O8hk3K7SrQ7du5IhVNRZaGVqRKBOnuGtFuV2/EuYDhFJKzyhTDY+brir2gJ2vR5dyeEvqad8cBLClV535ooxszVmWHVu4OM9TQCH/oPyxlrGp860x+uFP0vJJuL3WTYe7XpgnpAr9FOPkE2MtllKzaezz4l1koD6gvRFDCQSo79AmtG9NZSHCC7OnHjMAJZznMBBJa8MiZvKcfc30v6u3YqgrenZl3RhmUKdnMrSLJhQeRW1NWLFMWRhz6K1x7JkPrhnj2ry+7jvyAi+7h0jybcYhh6KxmQpYETmyNzO+/AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAAIDB54QwrWSQPou8UlGkpA8D3/Ws0ZGNiFutyIAQU0bzhzSB99AMsPl/4OJm5CGqpZMVyuLFgQHlMaArzeQJK7/8qN6piDZPP6A2lSRYuMJ/a8ciIVvjnepSUF+xx7PqeAnoarH8lxbdwhloBswnxn4iNcWTTMnxo73Ak9jpabj1m1a4e9+li6S8xCyA1AHxFXbjjAp5GxRvcUV2o3rMsDqdjM0IoU/+NNuCGtKApdTZNpFuk71AoKpM2/oxjuexEpOggyF30Pk5IdAgNtFMfD+pwcqzvSACbtKvk6VnSx4UtsFPWuizhWefWIkuV+7ml60NFMyD3eo28U9BQs2veU="],"x5t":"tUcTw0bM8ciXw9zIMlalEfyxdd8","x5t#S256":"eF-XsrHWa6gw8qC4W8RXJgA49xvac_7V-Tz7fdpS7ZM"},{"kid":"V3rRzf_j1beZjEmQnDeT8r8ZVnXpjW1Gk3635CTCEGk","kty":"RSA","alg":"RSA-OAEP","use":"enc","n":"1q1Iz-eyhnCWCBRKgq0xKm6cF2zHAi_a-L99OdwgnUgoGfut5bBTU2hGx9R1IGKn0loDjICtU64DVFpOaT7jY7oIG4BsQN3Et5H6O3XlVim5NQgMYVC6hKAreqnnVylUk-XfVvrQOotVkGfMFdARuBaLx1ubFxIHUONi2Mjgl2nZ8mmKg_GCsd5uKfJJ965zqSQu1CFn26YccTPp2doih4rykTGPVJdL5PVp3z4t9rTlahHbgCvv3E50yVK7LCNgtS9nmcZbD0meLqIZi3MoV0dBB_9C-qrEsevAIlPuXUmwtcbyDXOb1m7Xq_MPV_EASzoPYYjmk3k09zJ_p1EUTQ","e":"AQAB","x5c":["MIICmzCCAYMCBgGOLghSlzANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWrUjP57KGcJYIFEqCrTEqbpwXbMcCL9r4v3053CCdSCgZ+63lsFNTaEbH1HUgYqfSWgOMgK1TrgNUWk5pPuNjuggbgGxA3cS3kfo7deVWKbk1CAxhULqEoCt6qedXKVST5d9W+tA6i1WQZ8wV0BG4FovHW5sXEgdQ42LYyOCXadnyaYqD8YKx3m4p8kn3rnOpJC7UIWfbphxxM+nZ2iKHivKRMY9Ul0vk9WnfPi32tOVqEduAK+/cTnTJUrssI2C1L2eZxlsPSZ4uohmLcyhXR0EH/0L6qsSx68AiU+5dSbC1xvINc5vWbter8w9X8QBLOg9hiOaTeTT3Mn+nURRNAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAIKBZNe4GmyfqRW6Ee8ai1umbstAmyK3W1kP2i0xxINTlvY2rwblV8UCrdyi3laD7zvZy1midZmpKqtZqWpiNigeZ5aUt76paYvdSl5TAuvZGDGoEAhmmECbnDSQKLp36rCn7NlrgiTDfZZ2PvIKZ3cXClzqXLF/iC6uGiKOgY5yOFOa5QgsfItpJmmxHtTzrRF70RVsbZCexB1Lt4bcId6Y3x2w7JNUjKIhf1RZ3QZx8+3xBM4cJ83h2J4nE0+IlFeAJL3VLGdeOk+z+FGMu2mYkxJwkxd9Wl2ubqrRcNy0t61Bgp3s40BgD10pzvawTXl7lEgabc/jzN2R0lcXmLo="],"x5t":"n5Y_Obidr330txi13j50zHzVbfg","x5t#S256":"f-Hrw_t_qUq86Ux0J2EckWVycuM3L_IjdOK6DW0DFoc"}]}`, cfg.JWTJWKS)
	})

	t.Run("PREST_JSON_AGG_TYPE", func(t *testing.T) {
		t.Setenv("PREST_JSON_AGG_TYPE", "invalid")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, jsonAggDefault, cfg.JSONAggType)
	})

	t.Run("PREST_JSON_AGG_TYPE backwards compatible", func(t *testing.T) {
		t.Setenv("PREST_JSON_AGG_TYPE", jsonAgg)
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, jsonAgg, cfg.JSONAggType)
	})

	t.Run("PREST_JSON_AGG_TYPE default works", func(t *testing.T) {
		t.Setenv("PREST_JSON_AGG_TYPE", jsonAggDefault)
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, jsonAggDefault, cfg.JSONAggType)
	})
}

func Test_getPrestConfFile(t *testing.T) {
	testCases := []struct {
		name      string
		prestConf string
		expected  string
	}{
		{"custom config", "../prest.toml", "../prest.toml"},
		{"default config", "", "./prest.toml"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := getPrestConfFile(tc.prestConf)
			require.Equal(t, tc.expected, cfg)
		})
	}
}

func TestDatabaseURL(t *testing.T) {
	viperCfg()

	t.Run("PREST_PG_URL", func(t *testing.T) {
		t.Setenv("PREST_PG_URL", "postgresql://user:pass@localhost:1234/mydatabase/?sslmode=disable")
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, "mydatabase", cfg.PGDatabase)
		require.Equal(t, "localhost", cfg.PGHost)
		require.Equal(t, 1234, cfg.PGPort)
		require.Equal(t, "user", cfg.PGUser)
		require.Equal(t, "pass", cfg.PGPass)
		require.Equal(t, "disable", cfg.SSLMode)
	})

	t.Run("DATABASE_URL", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgresql://cloud:cloudPass@localhost:5432/CloudDatabase/?sslmode=disable")
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, "CloudDatabase", cfg.PGDatabase)
		require.Equal(t, 5432, cfg.PGPort)
		require.Equal(t, "cloud", cfg.PGUser)
		require.Equal(t, "cloudPass", cfg.PGPass)
		require.Equal(t, "disable", cfg.SSLMode)
	})
}

func TestHTTPPort(t *testing.T) {
	viperCfg()

	t.Run("set PORT", func(t *testing.T) {
		t.Setenv("PORT", "8080")
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, 8080, cfg.HTTPPort)
	})

	t.Run("set PREST_HTTP_PORT", func(t *testing.T) {
		t.Setenv("PREST_HTTP_PORT", "3030")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, 3030, cfg.HTTPPort)
	})

	t.Run("set PORT and PREST_HTTP_PORT", func(t *testing.T) {
		t.Setenv("PORT", "8080")
		t.Setenv("PREST_HTTP_PORT", "3000")
		viperCfg()
		cfg := &Prest{}
		Parse(cfg)
		require.Equal(t, 8080, cfg.HTTPPort)
	})
}

func Test_parseDatabaseURL(t *testing.T) {
	c := &Prest{PGURL: "postgresql://user:pass@localhost:5432/mydatabase/?sslmode=require"}
	parseDatabaseURL(c)
	require.Equal(t, "mydatabase", c.PGDatabase)
	require.Equal(t, 5432, c.PGPort)
	require.Equal(t, "user", c.PGUser)
	require.Equal(t, "pass", c.PGPass)
	require.Equal(t, "require", c.SSLMode)

	// errors
	// todo: make this default on any problem
	c = &Prest{PGURL: "postgresql://user:pass@localhost:port/mydatabase/?sslmode=require"}
	parseDatabaseURL(c)
	require.Equal(t, "", c.PGDatabase)

	c = &Prest{PGURL: `invalid%+o`}
	parseDatabaseURL(c)
	require.Equal(t, "", c.PGDatabase)
	require.Equal(t, "", c.PGUser)
}

func Test_fetchJWKS(t *testing.T) {
	serverJWKS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[{"alg":"RS256","e":"AQAB","kid":"lmjNOucrGdRiN7XlpWJbQRIzSeKBS7OD-92xrhch6kw","kty":"RSA","n":"9GPbUNJ_7dgq8k0eTbcCZtFMn-oTVpFHjzIi7nuyMm9TvIZNyu0q0O3buSIVTUWWhlakSgTp7hrRbldvxLmA4RSSs8oUw2Pm64q9oCdr0eXcnhL6mnfHASwpVed-aKMbM1Zlh1buDjPU0Ah_6D8sZaxqfOtMfrhT9LySbi91k2Hu16YJ6QK_RTj5BNjLZZSs2ns8-JdZKA-oL0RQwkEqO_QJrRvTWUhwguzpx4zACWc5zAQSWvDImbynH3N9L-rt2KoK3p2Zd0YZlCnZzK0iyYUHkVtTVixTFkYc-itceyZD64Z49q8vu478gIvu4dI8m3GIYeisZkKWBE5sjczvvw","use":"sig","x5c":["MIICmzCCAYMCBgGOLghSADANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQD0Y9tQ0n/t2CryTR5NtwJm0Uyf6hNWkUePMiLue7Iyb1O8hk3K7SrQ7du5IhVNRZaGVqRKBOnuGtFuV2/EuYDhFJKzyhTDY+brir2gJ2vR5dyeEvqad8cBLClV535ooxszVmWHVu4OM9TQCH/oPyxlrGp860x+uFP0vJJuL3WTYe7XpgnpAr9FOPkE2MtllKzaezz4l1koD6gvRFDCQSo79AmtG9NZSHCC7OnHjMAJZznMBBJa8MiZvKcfc30v6u3YqgrenZl3RhmUKdnMrSLJhQeRW1NWLFMWRhz6K1x7JkPrhnj2ry+7jvyAi+7h0jybcYhh6KxmQpYETmyNzO+/AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAAIDB54QwrWSQPou8UlGkpA8D3/Ws0ZGNiFutyIAQU0bzhzSB99AMsPl/4OJm5CGqpZMVyuLFgQHlMaArzeQJK7/8qN6piDZPP6A2lSRYuMJ/a8ciIVvjnepSUF+xx7PqeAnoarH8lxbdwhloBswnxn4iNcWTTMnxo73Ak9jpabj1m1a4e9+li6S8xCyA1AHxFXbjjAp5GxRvcUV2o3rMsDqdjM0IoU/+NNuCGtKApdTZNpFuk71AoKpM2/oxjuexEpOggyF30Pk5IdAgNtFMfD+pwcqzvSACbtKvk6VnSx4UtsFPWuizhWefWIkuV+7ml60NFMyD3eo28U9BQs2veU="],"x5t":"tUcTw0bM8ciXw9zIMlalEfyxdd8","x5t#S256":"eF-XsrHWa6gw8qC4W8RXJgA49xvac_7V-Tz7fdpS7ZM"},{"alg":"RSA-OAEP","e":"AQAB","kid":"V3rRzf_j1beZjEmQnDeT8r8ZVnXpjW1Gk3635CTCEGk","kty":"RSA","n":"1q1Iz-eyhnCWCBRKgq0xKm6cF2zHAi_a-L99OdwgnUgoGfut5bBTU2hGx9R1IGKn0loDjICtU64DVFpOaT7jY7oIG4BsQN3Et5H6O3XlVim5NQgMYVC6hKAreqnnVylUk-XfVvrQOotVkGfMFdARuBaLx1ubFxIHUONi2Mjgl2nZ8mmKg_GCsd5uKfJJ965zqSQu1CFn26YccTPp2doih4rykTGPVJdL5PVp3z4t9rTlahHbgCvv3E50yVK7LCNgtS9nmcZbD0meLqIZi3MoV0dBB_9C-qrEsevAIlPuXUmwtcbyDXOb1m7Xq_MPV_EASzoPYYjmk3k09zJ_p1EUTQ","use":"enc","x5c":["MIICmzCCAYMCBgGOLghSlzANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWrUjP57KGcJYIFEqCrTEqbpwXbMcCL9r4v3053CCdSCgZ+63lsFNTaEbH1HUgYqfSWgOMgK1TrgNUWk5pPuNjuggbgGxA3cS3kfo7deVWKbk1CAxhULqEoCt6qedXKVST5d9W+tA6i1WQZ8wV0BG4FovHW5sXEgdQ42LYyOCXadnyaYqD8YKx3m4p8kn3rnOpJC7UIWfbphxxM+nZ2iKHivKRMY9Ul0vk9WnfPi32tOVqEduAK+/cTnTJUrssI2C1L2eZxlsPSZ4uohmLcyhXR0EH/0L6qsSx68AiU+5dSbC1xvINc5vWbter8w9X8QBLOg9hiOaTeTT3Mn+nURRNAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAIKBZNe4GmyfqRW6Ee8ai1umbstAmyK3W1kP2i0xxINTlvY2rwblV8UCrdyi3laD7zvZy1midZmpKqtZqWpiNigeZ5aUt76paYvdSl5TAuvZGDGoEAhmmECbnDSQKLp36rCn7NlrgiTDfZZ2PvIKZ3cXClzqXLF/iC6uGiKOgY5yOFOa5QgsfItpJmmxHtTzrRF70RVsbZCexB1Lt4bcId6Y3x2w7JNUjKIhf1RZ3QZx8+3xBM4cJ83h2J4nE0+IlFeAJL3VLGdeOk+z+FGMu2mYkxJwkxd9Wl2ubqrRcNy0t61Bgp3s40BgD10pzvawTXl7lEgabc/jzN2R0lcXmLo="],"x5t":"n5Y_Obidr330txi13j50zHzVbfg","x5t#S256":"f-Hrw_t_qUq86Ux0J2EckWVycuM3L_IjdOK6DW0DFoc"}]}`))
	}))
	defer serverJWKS.Close()

	serverWellKnown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issuer":"http://127.0.0.1:8080/realms/master","authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth","token_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token","introspection_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token/introspect","userinfo_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/userinfo","end_session_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/logout","frontchannel_logout_session_supported":true,"frontchannel_logout_supported":true,"jwks_uri":"` + serverJWKS.URL + `","check_session_iframe":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/login-status-iframe.html","grant_types_supported":["authorization_code","implicit","refresh_token","password","client_credentials","urn:openid:params:grant-type:ciba","urn:ietf:params:oauth:grant-type:device_code"],"acr_values_supported":["0","1"],"response_types_supported":["code","none","id_token","token","id_token token","code id_token","code token","code id_token token"],"subject_types_supported":["public","pairwise"],"id_token_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"id_token_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"id_token_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"userinfo_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512","none"],"userinfo_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"userinfo_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"request_object_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512","none"],"request_object_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"request_object_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"response_modes_supported":["query","fragment","form_post","query.jwt","fragment.jwt","form_post.jwt","jwt"],"registration_endpoint":"http://127.0.0.1:8080/realms/master/clients-registrations/openid-connect","token_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"token_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"introspection_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"introspection_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"authorization_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"authorization_encryption_alg_values_supported":["RSA-OAEP","RSA-OAEP-256","RSA1_5"],"authorization_encryption_enc_values_supported":["A256GCM","A192GCM","A128GCM","A128CBC-HS256","A192CBC-HS384","A256CBC-HS512"],"claims_supported":["aud","sub","iss","auth_time","name","given_name","family_name","preferred_username","email","acr"],"claim_types_supported":["normal"],"claims_parameter_supported":true,"scopes_supported":["openid","roles","offline_access","email","microprofile-jwt","web-origins","acr","phone","profile","address"],"request_parameter_supported":true,"request_uri_parameter_supported":true,"require_request_uri_registration":true,"code_challenge_methods_supported":["plain","S256"],"tls_client_certificate_bound_access_tokens":true,"revocation_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/revoke","revocation_endpoint_auth_methods_supported":["private_key_jwt","client_secret_basic","client_secret_post","tls_client_auth","client_secret_jwt"],"revocation_endpoint_auth_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","HS256","HS512","ES256","RS256","HS384","ES512","PS256","PS512","RS512"],"backchannel_logout_supported":true,"backchannel_logout_session_supported":true,"device_authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth/device","backchannel_token_delivery_modes_supported":["poll","ping"],"backchannel_authentication_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/ciba/auth","backchannel_authentication_request_signing_alg_values_supported":["PS384","RS384","EdDSA","ES384","ES256","RS256","ES512","PS256","PS512","RS512"],"require_pushed_authorization_requests":false,"pushed_authorization_request_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/par/request","mtls_endpoint_aliases":{"token_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token","revocation_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/revoke","introspection_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/token/introspect","device_authorization_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/auth/device","registration_endpoint":"http://127.0.0.1:8080/realms/master/clients-registrations/openid-connect","userinfo_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/userinfo","pushed_authorization_request_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/par/request","backchannel_authentication_endpoint":"http://127.0.0.1:8080/realms/master/protocol/openid-connect/ext/ciba/auth"},"authorization_response_iss_parameter_supported":true}`))
	}))
	defer serverWellKnown.Close()

	c := &Prest{JWTWellKnownURL: serverWellKnown.URL}
	fetchJWKS(c)
	require.Equal(t, `{"keys":[{"alg":"RS256","e":"AQAB","kid":"lmjNOucrGdRiN7XlpWJbQRIzSeKBS7OD-92xrhch6kw","kty":"RSA","n":"9GPbUNJ_7dgq8k0eTbcCZtFMn-oTVpFHjzIi7nuyMm9TvIZNyu0q0O3buSIVTUWWhlakSgTp7hrRbldvxLmA4RSSs8oUw2Pm64q9oCdr0eXcnhL6mnfHASwpVed-aKMbM1Zlh1buDjPU0Ah_6D8sZaxqfOtMfrhT9LySbi91k2Hu16YJ6QK_RTj5BNjLZZSs2ns8-JdZKA-oL0RQwkEqO_QJrRvTWUhwguzpx4zACWc5zAQSWvDImbynH3N9L-rt2KoK3p2Zd0YZlCnZzK0iyYUHkVtTVixTFkYc-itceyZD64Z49q8vu478gIvu4dI8m3GIYeisZkKWBE5sjczvvw","use":"sig","x5c":["MIICmzCCAYMCBgGOLghSADANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQD0Y9tQ0n/t2CryTR5NtwJm0Uyf6hNWkUePMiLue7Iyb1O8hk3K7SrQ7du5IhVNRZaGVqRKBOnuGtFuV2/EuYDhFJKzyhTDY+brir2gJ2vR5dyeEvqad8cBLClV535ooxszVmWHVu4OM9TQCH/oPyxlrGp860x+uFP0vJJuL3WTYe7XpgnpAr9FOPkE2MtllKzaezz4l1koD6gvRFDCQSo79AmtG9NZSHCC7OnHjMAJZznMBBJa8MiZvKcfc30v6u3YqgrenZl3RhmUKdnMrSLJhQeRW1NWLFMWRhz6K1x7JkPrhnj2ry+7jvyAi+7h0jybcYhh6KxmQpYETmyNzO+/AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAAIDB54QwrWSQPou8UlGkpA8D3/Ws0ZGNiFutyIAQU0bzhzSB99AMsPl/4OJm5CGqpZMVyuLFgQHlMaArzeQJK7/8qN6piDZPP6A2lSRYuMJ/a8ciIVvjnepSUF+xx7PqeAnoarH8lxbdwhloBswnxn4iNcWTTMnxo73Ak9jpabj1m1a4e9+li6S8xCyA1AHxFXbjjAp5GxRvcUV2o3rMsDqdjM0IoU/+NNuCGtKApdTZNpFuk71AoKpM2/oxjuexEpOggyF30Pk5IdAgNtFMfD+pwcqzvSACbtKvk6VnSx4UtsFPWuizhWefWIkuV+7ml60NFMyD3eo28U9BQs2veU="],"x5t":"tUcTw0bM8ciXw9zIMlalEfyxdd8","x5t#S256":"eF-XsrHWa6gw8qC4W8RXJgA49xvac_7V-Tz7fdpS7ZM"},{"alg":"RSA-OAEP","e":"AQAB","kid":"V3rRzf_j1beZjEmQnDeT8r8ZVnXpjW1Gk3635CTCEGk","kty":"RSA","n":"1q1Iz-eyhnCWCBRKgq0xKm6cF2zHAi_a-L99OdwgnUgoGfut5bBTU2hGx9R1IGKn0loDjICtU64DVFpOaT7jY7oIG4BsQN3Et5H6O3XlVim5NQgMYVC6hKAreqnnVylUk-XfVvrQOotVkGfMFdARuBaLx1ubFxIHUONi2Mjgl2nZ8mmKg_GCsd5uKfJJ965zqSQu1CFn26YccTPp2doih4rykTGPVJdL5PVp3z4t9rTlahHbgCvv3E50yVK7LCNgtS9nmcZbD0meLqIZi3MoV0dBB_9C-qrEsevAIlPuXUmwtcbyDXOb1m7Xq_MPV_EASzoPYYjmk3k09zJ_p1EUTQ","use":"enc","x5c":["MIICmzCCAYMCBgGOLghSlzANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDDAZtYXN0ZXIwHhcNMjQwMzExMTQ1OTQxWhcNMzQwMzExMTUwMTIxWjARMQ8wDQYDVQQDDAZtYXN0ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWrUjP57KGcJYIFEqCrTEqbpwXbMcCL9r4v3053CCdSCgZ+63lsFNTaEbH1HUgYqfSWgOMgK1TrgNUWk5pPuNjuggbgGxA3cS3kfo7deVWKbk1CAxhULqEoCt6qedXKVST5d9W+tA6i1WQZ8wV0BG4FovHW5sXEgdQ42LYyOCXadnyaYqD8YKx3m4p8kn3rnOpJC7UIWfbphxxM+nZ2iKHivKRMY9Ul0vk9WnfPi32tOVqEduAK+/cTnTJUrssI2C1L2eZxlsPSZ4uohmLcyhXR0EH/0L6qsSx68AiU+5dSbC1xvINc5vWbter8w9X8QBLOg9hiOaTeTT3Mn+nURRNAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAIKBZNe4GmyfqRW6Ee8ai1umbstAmyK3W1kP2i0xxINTlvY2rwblV8UCrdyi3laD7zvZy1midZmpKqtZqWpiNigeZ5aUt76paYvdSl5TAuvZGDGoEAhmmECbnDSQKLp36rCn7NlrgiTDfZZ2PvIKZ3cXClzqXLF/iC6uGiKOgY5yOFOa5QgsfItpJmmxHtTzrRF70RVsbZCexB1Lt4bcId6Y3x2w7JNUjKIhf1RZ3QZx8+3xBM4cJ83h2J4nE0+IlFeAJL3VLGdeOk+z+FGMu2mYkxJwkxd9Wl2ubqrRcNy0t61Bgp3s40BgD10pzvawTXl7lEgabc/jzN2R0lcXmLo="],"x5t":"n5Y_Obidr330txi13j50zHzVbfg","x5t#S256":"f-Hrw_t_qUq86Ux0J2EckWVycuM3L_IjdOK6DW0DFoc"}]}`, c.JWTJWKS)
}

func Test_portFromEnv_Error(t *testing.T) {
	c := &Prest{}

	t.Setenv("PORT", "PORT")

	portFromEnv(c)
	// this should be zero as this only modifies c.HTTPPort when the "PORT" env is set
	require.Equal(t, 0, c.HTTPPort)
}

func Test_portFromEnv_OK(t *testing.T) {
	c := &Prest{}

	t.Setenv("PORT", "1234")
	portFromEnv(c)
	require.Equal(t, 1234, c.HTTPPort)
}

func Test_Auth(t *testing.T) {
	t.Setenv("PREST_CONF", "../testdata/prest.toml")

	viperCfg()
	cfg := &Prest{}
	Parse(cfg)
	require.Equal(t, false, cfg.AuthEnabled)
	require.Equal(t, "public", cfg.AuthSchema)
	require.Equal(t, "prest_users", cfg.AuthTable)
	require.Equal(t, "username", cfg.AuthUsername)
	require.Equal(t, "password", cfg.AuthPassword)
	require.Equal(t, "MD5", cfg.AuthEncrypt)

	metadata := []string{"first_name", "last_name", "last_login"}
	require.Equal(t, len(metadata), len(cfg.AuthMetadata))

	for i, v := range cfg.AuthMetadata {
		require.Equal(t, metadata[i], v)
	}
}

func Test_ExposeDataConfig(t *testing.T) {
	t.Setenv("PREST_CONF", "../testdata/prest_expose.toml")

	viperCfg()
	cfg := &Prest{}
	Parse(cfg)
	require.Equal(t, true, cfg.ExposeConf.Enabled)
	require.Equal(t, false, cfg.ExposeConf.DatabaseListing)
	require.Equal(t, false, cfg.ExposeConf.SchemaListing)
	require.Equal(t, false, cfg.ExposeConf.TableListing)

	metadata := []string{"first_name", "last_name", "last_login"}
	require.Equal(t, len(metadata), len(cfg.AuthMetadata))

	for i, v := range cfg.AuthMetadata {
		require.Equal(t, metadata[i], v)
	}
}
