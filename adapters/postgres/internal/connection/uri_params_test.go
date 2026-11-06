package connection

import (
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestAppendApplicationName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc string
		uri  string
		name string
		want string
	}{
		{
			desc: "key=value URI built from [pg]",
			uri:  "user=u dbname=d host=h port=5432 sslmode=disable connect_timeout=10",
			name: "prestd",
			want: "user=u dbname=d host=h port=5432 sslmode=disable connect_timeout=10 fallback_application_name=prestd",
		},
		{
			desc: "URL with no query",
			uri:  "postgres://u@h:5432/d",
			name: "prestd",
			want: "postgres://u@h:5432/d?fallback_application_name=prestd",
		},
		{
			desc: "URL with a query",
			uri:  "postgres://u@h:5432/d?sslmode=disable",
			name: "prestd",
			want: "postgres://u@h:5432/d?sslmode=disable&fallback_application_name=prestd",
		},
		{
			desc: "postgresql:// scheme",
			uri:  "postgresql://u@h:5432/d",
			name: "prestd",
			want: "postgresql://u@h:5432/d?fallback_application_name=prestd",
		},
		{
			desc: "URL already sets application_name",
			uri:  "postgres://u@h:5432/d?application_name=theirs",
			name: "prestd",
			want: "postgres://u@h:5432/d?application_name=theirs",
		},
		{
			desc: "URL sets it after another parameter",
			uri:  "postgres://u@h:5432/d?sslmode=disable&application_name=theirs",
			name: "prestd",
			want: "postgres://u@h:5432/d?sslmode=disable&application_name=theirs",
		},
		{
			desc: "key=value URI already sets fallback_application_name",
			uri:  "user=u dbname=d fallback_application_name=theirs",
			name: "prestd",
			want: "user=u dbname=d fallback_application_name=theirs",
		},
		{
			desc: "key=value URI opens with application_name",
			uri:  "application_name=theirs user=u dbname=d",
			name: "prestd",
			want: "application_name=theirs user=u dbname=d",
		},
		{
			desc: "the text inside a key=value password is not a parameter",
			uri:  "user=u dbname=d password=application_name=x",
			name: "prestd",
			want: "user=u dbname=d password=application_name=x fallback_application_name=prestd",
		},
		{
			desc: "the text twice inside a password is still not a parameter",
			uri:  "user=u password=application_name=application_name=x",
			name: "prestd",
			want: "user=u password=application_name=application_name=x fallback_application_name=prestd",
		},
		{
			desc: "the text inside a URL password is not a parameter",
			uri:  "postgres://u:application_name=x@h:5432/d",
			name: "prestd",
			want: "postgres://u:application_name=x@h:5432/d?fallback_application_name=prestd",
		},
		{
			desc: "a name with a space is quoted in a key=value URI",
			uri:  "user=u dbname=d",
			name: "my api",
			want: "user=u dbname=d fallback_application_name='my api'",
		},
		{
			desc: "a quote and a backslash are escaped in a key=value URI",
			uri:  "user=u dbname=d",
			name: `it's\x`,
			want: `user=u dbname=d fallback_application_name='it\'s\\x'`,
		},
		{
			desc: "a name is query-escaped in a URL",
			uri:  "postgres://u@h:5432/d",
			name: "my api&x=y",
			want: "postgres://u@h:5432/d?fallback_application_name=my+api%26x%3Dy",
		},
		{
			desc: "an empty name turns it off",
			uri:  "postgres://u@h:5432/d",
			name: "",
			want: "postgres://u@h:5432/d",
		},
		{
			desc: "an empty URI is left alone",
			uri:  "",
			name: "prestd",
			want: "",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, appendApplicationName(tc.uri, tc.name))
		})
	}
}

// Not parallel: it replaces dbConnect.
func TestAddDatabaseToPool_setsApplicationName(t *testing.T) {
	cfg := config.Prest{
		PGApplicationName: "prestd",
		Databases: []config.DatabaseConf{{
			Alias:    "tenant-a",
			URL:      "postgres://u@tenant-a:5432/tenant_a?sslmode=disable",
			Database: "tenant_a",
		}},
	}

	var uris []string
	restore := SetDBConnectForTest(func(_, uri string) (*sqlx.DB, error) {
		uris = append(uris, uri)
		return nil, errors.New("no connection in this test")
	})
	t.Cleanup(restore)
	m := NewManager(&cfg)

	_, err := m.AddDatabaseToPool("tenant-a")
	require.Error(t, err)
	require.Equal(t, []string{cfg.Databases[0].URL + "&fallback_application_name=prestd"}, uris)

	// The pool key stays the database address alone.
	require.Equal(t, cfg.Databases[0].URL, m.GetURI("tenant-a"))
}
