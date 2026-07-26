package postgres

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVectorOperator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		metric  string
		want    string
		wantErr error
	}{
		{name: "l2", metric: "l2", want: "<->"},
		{name: "euclidean alias", metric: "euclidean", want: "<->"},
		{name: "cosine", metric: "cosine", want: "<=>"},
		{name: "inner product", metric: "ip", want: "<#>"},
		{name: "l1", metric: "l1", want: "<+>"},
		{name: "case insensitive + spaces", metric: " L2 ", want: "<->"},
		{name: "unknown metric", metric: "hamming", wantErr: ErrInvalidVectorMetric},
		{name: "empty metric", metric: "", wantErr: ErrInvalidVectorMetric},
		{name: "injection attempt", metric: "l2; DROP TABLE users", wantErr: ErrInvalidVectorMetric},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetVectorOperator(tt.metric)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeVectorLiteral(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{name: "simple", in: "[1,2,3]", want: "[1,2,3]"},
		{name: "floats", in: "[0.1,0.2,0.3]", want: "[0.1,0.2,0.3]"},
		{name: "negatives", in: "[-1.5,2,-3]", want: "[-1.5,2,-3]"},
		{name: "whitespace tolerant", in: " [ 1 , 2 , 3 ] ", want: "[1,2,3]"},
		{name: "exponent", in: "[1e2,2]", want: "[100,2]"},
		{name: "not bracketed", in: "1,2,3", wantErr: ErrInvalidVector},
		{name: "empty vector", in: "[]", wantErr: ErrInvalidVector},
		{name: "non numeric", in: "[1,abc,3]", wantErr: ErrInvalidVector},
		{name: "single quote injection", in: "[1,2]'; DROP TABLE t --", wantErr: ErrInvalidVector},
		{name: "quote inside", in: "[1,'2',3]", wantErr: ErrInvalidVector},
		{name: "nan rejected", in: "[NaN,1]", wantErr: ErrInvalidVector},
		{name: "inf rejected", in: "[Inf,1]", wantErr: ErrInvalidVector},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeVectorLiteral(tt.in)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeVectorLiteral_DimensionCap(t *testing.T) {
	t.Parallel()

	// build a vector with more than the pgvector max dimensions
	parts := make([]string, maxVectorDims+1)
	for i := range parts {
		parts[i] = "1"
	}
	oversized := "[" + strings.Join(parts, ",") + "]"

	_, err := normalizeVectorLiteral(oversized)
	require.ErrorIs(t, err, ErrInvalidVector)
}

func TestBuildVectorOrderTerm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    string
		want    string
		wantErr error
	}{
		{
			name: "l2 order",
			spec: "embedding:l2:[0.1,0.2,0.3]",
			want: `"embedding" <-> '[0.1,0.2,0.3]'::vector`,
		},
		{
			name: "cosine order with alias column",
			spec: "docs.embedding:cosine:[1,2]",
			want: `"docs"."embedding" <=> '[1,2]'::vector`,
		},
		{name: "missing parts", spec: "embedding:l2", wantErr: ErrInvalidVectorOrder},
		{name: "invalid column", spec: "0col:l2:[1,2]", wantErr: ErrInvalidIdentifier},
		{name: "invalid metric", spec: "embedding:bogus:[1,2]", wantErr: ErrInvalidVectorMetric},
		{name: "invalid vector", spec: "embedding:l2:[a,b]", wantErr: ErrInvalidVector},
		{
			name:    "column injection blocked",
			spec:    `embedding") ; DROP TABLE t --:l2:[1,2]`,
			wantErr: ErrInvalidIdentifier,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildVectorOrderTerm(tt.spec)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
